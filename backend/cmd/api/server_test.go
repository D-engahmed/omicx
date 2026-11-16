package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"
)

func newTestServerLogger() *slog.Logger {
	return slog.New(
		slog.NewTextHandler(io.Discard, nil),
	)
}

func newTestListener(t *testing.T) net.Listener {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create test listener: %v", err)
	}

	t.Cleanup(func() {
		_ = listener.Close()
	})

	return listener
}

func waitForSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()

	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for signal")
	}
}

func TestServeHTTPGracefulShutdown(t *testing.T) {
	requestStarted := make(chan struct{})
	finishRequest := make(chan struct{})

	releaseRequest := func() {
		select {
		case <-finishRequest:
		default:
			close(finishRequest)
		}
	}
	t.Cleanup(releaseRequest)

	server := &http.Server{
		Handler: http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			close(requestStarted)

			<-finishRequest

			_, _ = io.WriteString(w, "request completed")
		}),
	}

	listener := newTestListener(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverDone := make(chan error, 1)

	go func() {
		serverDone <- serveHTTP(
			ctx,
			server,
			listener,
			2*time.Second,
			newTestServerLogger(),
		)
	}()

	clientDone := make(chan error, 1)

	go func() {
		response, err := http.Get(
			"http://" + listener.Addr().String(),
		)
		if err != nil {
			clientDone <- err
			return
		}
		defer response.Body.Close()

		body, err := io.ReadAll(response.Body)
		if err != nil {
			clientDone <- err
			return
		}

		if response.StatusCode != http.StatusOK {
			clientDone <- errors.New("unexpected HTTP response status")
			return
		}

		if string(body) != "request completed" {
			clientDone <- errors.New("unexpected response body")
			return
		}

		clientDone <- nil
	}()

	waitForSignal(t, requestStarted)

	// Request a graceful shutdown while the request is active.
	cancel()

	// The server must not stop before the active request finishes.
	select {
	case err := <-serverDone:
		t.Fatalf("server stopped before request completed: %v", err)

	case <-time.After(100 * time.Millisecond):
		// Expected: Shutdown is waiting for the active request.
	}

	releaseRequest()

	select {
	case err := <-clientDone:
		if err != nil {
			t.Fatalf("client request failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("client request did not finish")
	}

	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatalf("graceful shutdown failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not shut down")
	}
}

func TestServeHTTPForcedShutdownAfterTimeout(t *testing.T) {
	requestStarted := make(chan struct{})

	server := &http.Server{
		Handler: http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			close(requestStarted)

			// Wait until the server forcibly closes the connection.
			<-r.Context().Done()
		}),
	}

	listener := newTestListener(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverDone := make(chan error, 1)

	go func() {
		serverDone <- serveHTTP(
			ctx,
			server,
			listener,
			100*time.Millisecond,
			newTestServerLogger(),
		)
	}()

	clientDone := make(chan struct{})

	go func() {
		defer close(clientDone)

		response, err := http.Get(
			"http://" + listener.Addr().String(),
		)
		if err == nil {
			_ = response.Body.Close()
		}
	}()

	waitForSignal(t, requestStarted)

	cancel()

	select {
	case err := <-serverDone:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf(
				"expected shutdown deadline error, got: %v",
				err,
			)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("forced shutdown did not complete")
	}

	select {
	case <-clientDone:
	case <-time.After(2 * time.Second):
		t.Fatal("client connection did not close")
	}
}

func TestServeHTTPReturnsUnexpectedServerError(t *testing.T) {
	listener := newTestListener(t)

	if err := listener.Close(); err != nil {
		t.Fatalf("failed to close test listener: %v", err)
	}

	server := &http.Server{}

	err := serveHTTP(
		context.Background(),
		server,
		listener,
		time.Second,
		newTestServerLogger(),
	)

	if err == nil {
		t.Fatal("expected server error, got nil")
	}
}
