package main

import (
	"log"
	"net/http"

	"dante-simulator-api/internal/config"
	"dante-simulator-api/internal/profiles"
	"dante-simulator-api/internal/proxy"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	cfg := config.Load()

	httpClient := &http.Client{
		Timeout: cfg.HTTPClientTimeout,
	}

	proxyHandler := proxy.NewHandler(cfg, httpClient)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Get("/", handleRoot)
	r.Get("/ready", handleReady)
	r.Get("/profiles", profiles.HandleProfiles)
	r.HandleFunc("/proxy/*", proxyHandler.ServeHTTP)

	log.Printf("simulator api listening on %s", cfg.AppAddr)
	log.Printf("dante target: %s", cfg.DanteBaseURL)
	log.Printf("legacy target: %s", cfg.LegacyBaseURL)

	if err := http.ListenAndServe(cfg.AppAddr, r); err != nil {
		log.Fatal(err)
	}
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_, _ = w.Write([]byte(`{"service":"dante-simulator-api","status":"ok"}`))
}

func handleReady(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_, _ = w.Write([]byte(`{"status":"ok","service":"dante-simulator-api"}`))
}