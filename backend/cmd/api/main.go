package main

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/D-engahmed/omicx/internal/platform/config"
	"github.com/D-engahmed/omicx/internal/platform/httpserver"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	handler := httpserver.NewHandler()

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
	}

	log.Printf(
		"omicx API starting: environment=%s address=%s",
		cfg.AppEnv,
		cfg.HTTPAddr,
	)

	if err := server.ListenAndServe(); err != nil &&
		!errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("Server failed: %v", err)
	}
}
