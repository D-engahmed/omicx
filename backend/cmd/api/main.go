package main

import (
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/D-engahmed/omicx/internal/platform/config"
	"github.com/D-engahmed/omicx/internal/platform/httpserver"
	"github.com/D-engahmed/omicx/internal/platform/logger"
)

func main() {
	os.Exit(run())
}

func run() int {
	// Load configuration.
	cfg, err := config.Load()
	if err != nil {
		log.Printf("failed to load configuration: %v", err)
		return 1
	}

	// Initialize structured logger.
	appLogger, err := logger.New(logger.Options{
		Environment: cfg.AppEnv,
		Level:       cfg.LogLevel,
		Output:      os.Stdout,
	})
	if err != nil {
		log.Printf("failed to initialize logger: %v", err)
		return 1
	}

	// Create request logging middleware.
	requestLogging, err := httpserver.NewRequestLoggingMiddleware(appLogger)
	if err != nil {
		appLogger.Error("failed to initialize request middleware", "error", err)
		return 1
	}

	// Build HTTP handler with middleware.
	handler := requestLogging(httpserver.NewHandler())

	// Configure HTTP server.
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
	}

	appLogger.Info(
		"http_server_starting",
		"environment", cfg.AppEnv,
		"address", cfg.HTTPAddr,
	)

	// Start server.
	if err := server.ListenAndServe(); err != nil &&
		!errors.Is(err, http.ErrServerClosed) {
		appLogger.Error(
			"http_server_failed",
			"error", err,
		)
		return 1
	}

	appLogger.Info("http_server_stopped")

	return 0
}
