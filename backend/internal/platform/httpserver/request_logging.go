package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

const RequestIDHeader = "X-Request-ID"

type requestIDContextKey struct{}

type Middleware func(http.Handler) http.Handler

// RequestIDFromContext retrieves the server-generated request ID.
func RequestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return requestID
}

// NewRequestLoggingMiddleware creates HTTP request logging middleware.
func NewRequestLoggingMiddleware(
	appLogger *slog.Logger,
) (Middleware, error) {
	if appLogger == nil {
		return nil, errors.New("request logging requires a logger")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()

			requestID, err := generateRequestID()
			if err != nil {
				appLogger.Error(
					"request_id_generation_failed",
					"error", err,
				)

				http.Error(
					w,
					"Internal server error",
					http.StatusInternalServerError,
				)
				return
			}

			// Generate the ID on the server; never trust a client-supplied ID.
			w.Header().Set(RequestIDHeader, requestID)

			ctx := context.WithValue(
				r.Context(),
				requestIDContextKey{},
				requestID,
			)
			r = r.WithContext(ctx)

			// The returned writer exposes only optional interfaces supported
			// by the original ResponseWriter.
			recorder, wrappedWriter := newResponseRecorder(w)

			defer func() {
				recovered := recover()
				panicked := recovered != nil

				// Do not write an HTTP status after the connection has been
				// successfully hijacked.
				if panicked && recorder.statusCode == 0 && !recorder.hijacked {
					recorder.WriteHeader(http.StatusInternalServerError)
				}

				logRequest(
					appLogger,
					r,
					recorder,
					requestID,
					startedAt,
					panicked,
				)

				// Leave panic recovery to net/http or an outer recovery
				// middleware, after recording this request.
				if panicked {
					panic(recovered)
				}
			}()

			next.ServeHTTP(wrappedWriter, r)
		})
	}, nil
}

// generateRequestID creates a cryptographically random request identifier.
func generateRequestID() (string, error) {
	var id [16]byte

	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}

	return hex.EncodeToString(id[:]), nil
}

// logRequest writes the completed request as a structured event.
func logRequest(
	appLogger *slog.Logger,
	r *http.Request,
	recorder *responseRecorder,
	requestID string,
	startedAt time.Time,
	panicked bool,
) {
	status := recorder.status()

	level := slog.LevelInfo

	switch {
	case panicked || status >= http.StatusInternalServerError:
		level = slog.LevelError
	case status >= http.StatusBadRequest:
		level = slog.LevelWarn
	}

	appLogger.LogAttrs(
		r.Context(),
		level,
		"http_request_completed",
		slog.String("request_id", requestID),
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.Int("status", status),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		slog.Int("response_bytes", recorder.bytesWritten),
		slog.Bool("panicked", panicked),
	)
}
