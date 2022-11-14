package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/klauspost/compress/gzip"
)

const (
	serverTimeout = time.Minute
)

var config struct {
	serverPort        string
	failAuthChance    float64
	noReceiveChance   float64
	badResponseChance float64
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
	flag.Float64Var(&config.noReceiveChance, "random_no_receive", 0, "chance to stop receiving data, from 0.0 to 1.0")
	flag.Float64Var(&config.badResponseChance, "random_bad_response", 0, "chance to return a non-200 response status code, from 0.0 to 1.0")
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
		if config.noReceiveChance > rand.Float64() {
			log.Println("triggered a stop receiving chance, sleeping for 30 seconds")
			time.Sleep(time.Second * 30)
			return
		}
		if config.badResponseChance > rand.Float64() {
			log.Println("triggered a bad response chance, returning status 500")
			w.WriteHeader(http.StatusInternalServerError)
			return
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