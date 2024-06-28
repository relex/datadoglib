package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/goccy/go-json"
	"github.com/klauspost/compress/gzip"
	"github.com/relex/gotils/logger"
)

const (
	serverTimeout = time.Minute
)

var config struct {
	serverPort         string
	failAuthChance     float64
	slowReceiveChance  float64
	badResponseChance  float64
	randomNetworkLag   int
	disableJsonParsing bool
	showTimestamp      bool
}

func main() {
	parseConfig()

	logger.Info("starting the server at port " + config.serverPort)
	srv := http.Server{
		Addr:         ":" + config.serverPort,
		Handler:      getRouter(),
		ReadTimeout:  serverTimeout,
		WriteTimeout: serverTimeout,
	}

	err := srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Fatal("error while serving http: ", err)
	}

	logger.Info("exiting the application normally")
	logger.Exit(0)
}

func parseConfig() {
	flag.StringVar(&config.serverPort, "port", "8083", "local port to launch the server at")
	flag.Float64Var(&config.failAuthChance, "random_no_auth", 0, "chance to fail authentication, from 0.0 to 1.0")
	flag.Float64Var(&config.slowReceiveChance, "random_slow_receive", 0, "chance to receive data slowly, from 0.0 to 1.0")
	flag.Float64Var(&config.badResponseChance, "random_bad_response", 0, "chance to return a non-200 response status code, from 0.0 to 1.0")
	flag.IntVar(&config.randomNetworkLag, "random_network_lag", 0, "maximum random network lag on receiving request and responding, msec")
	flag.BoolVar(&config.disableJsonParsing, "disable_json_parsing", false, "disable payload unmarshalling to increase processing speed, boolean")
	flag.BoolVar(&config.showTimestamp, "show_timestamp", false, "show timestamp in output before each request or log, boolean")
	flag.Parse()
	logger.Debug("Config: ", config)
}

func getRouter() chi.Router {
	mux := chi.NewMux()

	mux.Use(middleware.StripSlashes)
	mux.Use(randomFailMiddleware)

	mux.Route("/api/v2", func(r chi.Router) {
		r.Post("/logs", handleLogs)
	})

	return mux
}

var numOngoingRequests int64 = 0

func randomFailMiddleware(next http.Handler) http.Handler {
	rand.Seed(time.Now().UnixNano())

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		numReqs := atomic.AddInt64(&numOngoingRequests, 1)
		defer atomic.AddInt64(&numOngoingRequests, -1)

		reqLog := logger.WithField("remote", r.RemoteAddr)
		reqLog.Infof("incoming request(%d): addr=%s, protocol=%s, headers=%v", numReqs, r.RemoteAddr, r.Proto, r.Header)

		if config.failAuthChance > rand.Float64() {
			reqLog.Infof("triggered a fail auth chance, returning status 403")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if config.slowReceiveChance > rand.Float64() {
			reqLog.Infof("triggered a slow receive chance, sleeping for 30 seconds")
			time.Sleep(time.Second * 30)
			return
		}
		if config.badResponseChance > rand.Float64() {
			reqLog.Infof("triggered a bad response chance, returning status 500")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if config.randomNetworkLag > 0 {
			networkLag := time.Duration(rand.Intn(config.randomNetworkLag)) * time.Millisecond
			defer time.Sleep(networkLag)
		}

		next.ServeHTTP(w, r)
	})
}

const maxPayloadSize = 5 * 1024 * 1024

func handleLogs(w http.ResponseWriter, r *http.Request) {
	reqLog := logger.WithField("remote", r.RemoteAddr)
	reader, err := gzip.NewReader(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		reqLog.Warn("failed to create gzip reader: ", err)
		return
	}

	buf := bytes.NewBuffer(make([]byte, 0, maxPayloadSize))
	if config.disableJsonParsing {
		if config.showTimestamp {
			buf.WriteString(time.Now().Format(time.RFC3339Nano) + ": ")
		}

		written, err := io.Copy(buf, reader)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			reqLog.Warn("failed to read gzip request: ", err)
			return
		}
		if written > maxPayloadSize {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			reqLog.Errorf("payload size %d exceeds the maximum allowed size of %d", written, maxPayloadSize)
			return
		}

		_ = buf.WriteByte('\n')
		_, _ = buf.WriteTo(os.Stdout)

		w.WriteHeader(http.StatusAccepted)
		return
	}

	dataBytes, err := io.ReadAll(reader)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		reqLog.Warn("failed to read gzip request: ", err)
		return
	}

	if len(dataBytes) > maxPayloadSize {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		reqLog.Errorf("payload size %d exceeds the maximum allowed size of %d", len(dataBytes), maxPayloadSize)
		return
	}

	prefix := ""
	if config.showTimestamp {
		prefix = time.Now().Format(time.RFC3339Nano) + ": "
	}

	var requestData []map[string]any
	err = json.Unmarshal(dataBytes, &requestData)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		reqLog.Errorf("failed to decode JSON request body: %s. Body: %s", err, string(dataBytes))
		return
	}

	for _, log := range requestData {
		_, _ = buf.WriteString(prefix)
		_, _ = fmt.Fprintln(buf, log)
		_, _ = buf.WriteTo(os.Stdout)
	}

	w.WriteHeader(http.StatusAccepted)
}
