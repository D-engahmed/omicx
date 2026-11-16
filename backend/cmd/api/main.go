package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/D-engahmed/omicx/internal/platform/config"
	"github.com/D-engahmed/omicx/internal/platform/httpserver"
	"github.com/D-engahmed/omicx/internal/platform/logger"
)

func main() {
	os.Exit(run())
}

func run() int {
	// Load and validate configuration.
	cfg, err := config.Load()
	if err != nil {
		log.Printf("failed to load configuration: %v", err)
		return 1
	}

	// Initialize structured logging.
	appLogger, err := logger.New(logger.Options{
		Environment: cfg.AppEnv,
		Level:       cfg.LogLevel,
		Output:      os.Stdout,
	})
	if err != nil {
		log.Printf("failed to initialize logger: %v", err)
		return 1
	}

	// Build request logging middleware.
	requestLogging, err := httpserver.NewRequestLoggingMiddleware(appLogger)
	if err != nil {
		appLogger.Error(
			"failed to initialize request middleware",
			"error", err,
		)
		return 1
	}

	handler := requestLogging(httpserver.NewHandler())

	// Configure the HTTP server.
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
	}

	// Bind the network listener.
	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		appLogger.Error(
			"http_server_listen_failed",
			"address", cfg.HTTPAddr,
			"error", err,
		)
		return 1
	}

	appLogger.Info(
		"http_server_starting",
		"environment", cfg.AppEnv,
		"address", listener.Addr().String(),
	)

	// Convert termination signals into context cancellation.
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	// Run the server until it stops or receives a termination signal.
	if err := serveHTTP(
		ctx,
		server,
		listener,
		cfg.HTTPShutdownTimeout,
		appLogger,
	); err != nil {
		appLogger.Error("http_server_failed", "error", err)
		return 1
	}

	appLogger.Info("http_server_stopped")

	return 0
}
