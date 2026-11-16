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
	// 1. Load and validate configuration.
	cfg, err := config.Load()
	if err != nil {
		log.Printf("failed to load configuration: %v", err)
		return 1
	}

	// 2. Initialize structured logger.
	appLogger, err := logger.New(logger.Options{
		Environment: cfg.AppEnv,
		Level:       cfg.LogLevel,
		Output:      os.Stdout,
	})
	if err != nil {
		log.Printf("failed to initialize logger: %v", err)
		return 1
	}

	// 3. Initialize HTTP handler.
	handler := httpserver.NewHandler()

	// 4. Configure HTTP server.
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
	}

	// 5. Log server startup.
	appLogger.Info(
		"http_server_starting",
		"environment", cfg.AppEnv,
		"address", cfg.HTTPAddr,
	)

	// 6. Start HTTP server.
	if err := server.ListenAndServe(); err != nil &&
		!errors.Is(err, http.ErrServerClosed) {

		appLogger.Error(
			"http_server_failed",
			"error", err,
		)

		return 1
	}

	// 7. Log normal shutdown.
	appLogger.Info("http_server_stopped")

	return 0
}
