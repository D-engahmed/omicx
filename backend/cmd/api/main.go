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
	"github.com/D-engahmed/omicx/internal/platform/database"
	"github.com/D-engahmed/omicx/internal/platform/httpserver"
	"github.com/D-engahmed/omicx/internal/platform/logger"
)

func main() {
	os.Exit(run())
}

func run() int {
	// Load and validate application configuration.
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

	// Database configuration is required to start the API.
	if cfg.DatabaseURL == "" {
		appLogger.Error(
			"database_configuration_missing",
			"setting", "DATABASE_URL",
		)
		return 1
	}

	// Initialize and verify the PostgreSQL connection pool.
	pool, err := database.NewPool(
		context.Background(),
		database.PoolConfig{
			URL:            cfg.DatabaseURL,
			MaxConns:       cfg.DatabaseMaxConns,
			MinConns:       cfg.DatabaseMinConns,
			ConnectTimeout: cfg.DatabaseConnectTimeout,
		},
	)
	if err != nil {
		appLogger.Error(
			"database_connection_failed",
			"error", err,
		)
		return 1
	}

	// Keep the pool alive until the API has completely stopped.
	defer pool.Close()

	appLogger.Info("database_connected")

	// Create request logging middleware.
	requestLogging, err := httpserver.NewRequestLoggingMiddleware(appLogger)
	if err != nil {
		appLogger.Error(
			"failed to initialize request middleware",
			"error", err,
		)
		return 1
	}

	// Build HTTP handler with database readiness support.
	handler := requestLogging(
		httpserver.NewHandler(pool),
	)

	// Configure the HTTP server.
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
	}

	// Register operating system termination signals.
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

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

	// Run the HTTP server and manage graceful shutdown.
	if err := serveHTTP(
		ctx,
		server,
		listener,
		cfg.HTTPShutdownTimeout,
		appLogger,
	); err != nil {
		appLogger.Error(
			"http_server_failed",
			"error", err,
		)
		return 1
	}

	appLogger.Info("http_server_stopped")

	return 0
}
