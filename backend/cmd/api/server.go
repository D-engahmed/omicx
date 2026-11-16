package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// serveHTTP starts the HTTP server and manages its lifecycle.
//
// It returns when the server stops unexpectedly or when
// graceful shutdown has completed.
func serveHTTP(
	ctx context.Context,
	server *http.Server,
	listener net.Listener,
	shutdownTimeout time.Duration,
	appLogger *slog.Logger,
) error {
	if ctx == nil {
		return errors.New("server context must not be nil")
	}

	if server == nil {
		return errors.New("HTTP server must not be nil")
	}

	if listener == nil {
		return errors.New("HTTP listener must not be nil")
	}

	if appLogger == nil {
		return errors.New("server logger must not be nil")
	}

	if shutdownTimeout <= 0 {
		return errors.New("shutdown timeout must be greater than zero")
	}

	// A buffered channel ensures the Serve goroutine can
	// report its result even if this function returns early.
	serverErrors := make(chan error, 1)

	go func() {
		serverErrors <- server.Serve(listener)
	}()

	// Wait for either a server failure or a termination signal.
	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP server stopped unexpectedly: %w", err)
		}

		return nil

	case <-ctx.Done():
		appLogger.Info(
			"http_server_shutdown_started",
			"timeout", shutdownTimeout.String(),
		)
	}

	// IMPORTANT:
	// Do not derive this context from the canceled signal context.
	// Doing so would make Shutdown time out immediately.
	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		shutdownTimeout,
	)
	defer cancel()

	// Stop accepting new connections and wait for active
	// requests to finish.
	if err := server.Shutdown(shutdownCtx); err != nil {
		appLogger.Error(
			"http_server_shutdown_failed",
			"error", err,
		)

		// Graceful shutdown exceeded its deadline.
		// Force-close remaining connections.
		appLogger.Warn("http_server_forced_shutdown")

		if closeErr := server.Close(); closeErr != nil {
			return errors.Join(
				fmt.Errorf("graceful shutdown failed: %w", err),
				fmt.Errorf("forced shutdown failed: %w", closeErr),
			)
		}

		return fmt.Errorf("graceful shutdown failed: %w", err)
	}

	// Wait for Serve to return after Shutdown.
	if err := <-serverErrors; err != nil &&
		!errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("HTTP server stopped unexpectedly: %w", err)
	}

	return nil
}
