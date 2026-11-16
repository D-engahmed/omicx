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

// RequestIDFromContext retrieves the server-generated request ID.
func RequestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return requestID
}

type Middleware func(http.Handler) http.Handler

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

			// Add the request ID to the response.
			w.Header().Set(RequestIDHeader, requestID)

			// Add the request ID to the request context.
			ctx := context.WithValue(
				r.Context(),
				requestIDContextKey{},
				requestID,
			)

			r = r.WithContext(ctx)

			recorder := &responseRecorder{
				ResponseWriter: w,
			}

			// Log the request even if the handler panics.
			defer func() {
				recovered := recover()
				panicked := recovered != nil

				if panicked && recorder.statusCode == 0 {
					recorder.WriteHeader(
						http.StatusInternalServerError,
					)
				}

				logRequest(
					appLogger,
					r,
					recorder,
					requestID,
					startedAt,
					panicked,
				)

				// Preserve net/http's normal panic recovery behavior.
				if panicked {
					panic(recovered)
				}
			}()

			next.ServeHTTP(recorder, r)
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
		slog.Int64(
			"duration_ms",
			time.Since(startedAt).Milliseconds(),
		),
		slog.Int("response_bytes", recorder.bytesWritten),
		slog.Bool("panicked", panicked),
	)
}

// responseRecorder tracks the HTTP response status and body size.
type responseRecorder struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (recorder *responseRecorder) WriteHeader(statusCode int) {
	if recorder.statusCode != 0 {
		return
	}

	recorder.statusCode = statusCode
	recorder.ResponseWriter.WriteHeader(statusCode)
}

func (recorder *responseRecorder) Write(body []byte) (int, error) {
	if recorder.statusCode == 0 {
		recorder.WriteHeader(http.StatusOK)
	}

	n, err := recorder.ResponseWriter.Write(body)
	recorder.bytesWritten += n

	return n, err
}

func (recorder *responseRecorder) status() int {
	if recorder.statusCode == 0 {
		return http.StatusOK
	}

	return recorder.statusCode
}

// Unwrap allows http.ResponseController to reach the original writer.
func (recorder *responseRecorder) Unwrap() http.ResponseWriter {
	return recorder.ResponseWriter
}
