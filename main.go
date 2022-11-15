package main

import (
	"flag"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/goccy/go-json"
	"github.com/klauspost/compress/gzip"
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
}

func main() {
	parseConfig()

	log.Println("starting the server at port " + config.serverPort)
	srv := http.Server{
		Addr:         ":" + config.serverPort,
		Handler:      getRouter(),
		ReadTimeout:  serverTimeout,
		WriteTimeout: serverTimeout,
	}

	panic(srv.ListenAndServe())
}

func parseConfig() {
	flag.StringVar(&config.serverPort, "port", "8083", "local port to launch the server at")
	flag.Float64Var(&config.failAuthChance, "random_no_auth", 0, "chance to fail authentication, from 0.0 to 1.0")
	flag.Float64Var(&config.slowReceiveChance, "random_slow_receive", 0, "chance to receive data slowly, from 0.0 to 1.0")
	flag.Float64Var(&config.badResponseChance, "random_bad_response", 0, "chance to return a non-200 response status code, from 0.0 to 1.0")
	flag.IntVar(&config.randomNetworkLag, "random_network_lag", 0, "maximum random network lag on receiving request and responding, msec")
	flag.BoolVar(&config.disableJsonParsing, "disable_json_parsing", false, "disable payload unmarshalling to increase processing speed, boolean")
	flag.Parse()
}

func getRouter() *chi.Mux {
	mux := chi.NewMux()

	mux.Use(middleware.StripSlashes)
	mux.Use(randomFailMiddleware)

	mux.Post("/", handleLogs)

	return mux
}

func randomFailMiddleware(next http.Handler) http.Handler {
	rand.Seed(time.Now().UnixNano())

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if config.failAuthChance > rand.Float64() {
			log.Println("triggered a fail auth chance, returning status 403")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if config.slowReceiveChance > rand.Float64() {
			log.Println("triggered a slow receive chance, sleeping for 30 seconds")
			time.Sleep(time.Second * 30)
			return
		}
		if config.badResponseChance > rand.Float64() {
			log.Println("triggered a bad response chance, returning status 500")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if config.randomNetworkLag > 0 {
			networkLag := time.Duration(rand.Intn(config.randomNetworkLag)) * time.Millisecond
			defer time.Sleep(networkLag)
			time.Sleep(networkLag)
		}

		next.ServeHTTP(w, r)
	})
}

func handleLogs(w http.ResponseWriter, r *http.Request) {
	reader, err := gzip.NewReader(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println(err)
		return
	}

	if config.disableJsonParsing {
		_, err = io.Copy(os.Stdout, reader)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Println(err)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	dataBytes, err := io.ReadAll(reader)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println(err)
		return
	}

	var requestData []map[string]string
	err = json.Unmarshal(dataBytes, &requestData)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println(err, string(dataBytes))
		return
	}

	log.Println(requestData)

	w.WriteHeader(http.StatusOK)
}
