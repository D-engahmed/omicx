package httpserver

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newRequestTestLogger() (*slog.Logger, *bytes.Buffer) {
	var output bytes.Buffer

	logger := slog.New(
		slog.NewJSONHandler(&output, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}),
	)

	return logger, &output
}

func readLogEvent(t *testing.T, output *bytes.Buffer) map[string]any {
	t.Helper()

	var event map[string]any

	if err := json.Unmarshal(output.Bytes(), &event); err != nil {
		t.Fatalf("failed to decode log event: %v", err)
	}

	return event
}

func TestRequestLogging(t *testing.T) {
	appLogger, output := newRequestTestLogger()

	middleware, err := NewRequestLoggingMiddleware(appLogger)
	if err != nil {
		t.Fatalf("unexpected middleware error: %v", err)
	}

	var contextRequestID string

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contextRequestID = RequestIDFromContext(r.Context())

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	})

	handler := middleware(next)

	req := httptest.NewRequest(
		http.MethodPost,
		"/conversations?token=do-not-log",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	// Verify response.
	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", recorder.Code)
	}

	// Verify request ID.
	requestID := recorder.Header().Get(RequestIDHeader)

	if len(requestID) != 32 {
		t.Fatalf("expected 32-character request ID, got %q", requestID)
	}

	if contextRequestID != requestID {
		t.Errorf("request ID in context does not match response header")
	}

	// Verify structured log.
	event := readLogEvent(t, output)

	if event["msg"] != "http_request_completed" {
		t.Errorf("unexpected message: %v", event["msg"])
	}

	if event["method"] != "POST" {
		t.Errorf("expected POST, got %v", event["method"])
	}

	if event["path"] != "/conversations" {
		t.Errorf("unexpected path: %v", event["path"])
	}

	if event["status"] != float64(http.StatusCreated) {
		t.Errorf("unexpected status: %v", event["status"])
	}

	if event["request_id"] != requestID {
		t.Error("request ID missing or incorrect in log")
	}

	if event["response_bytes"] != float64(7) {
		t.Errorf("expected 7 response bytes, got %v", event["response_bytes"])
	}

	if _, exists := event["duration_ms"]; !exists {
		t.Error("duration_ms field missing")
	}

	if strings.Contains(output.String(), "do-not-log") {
		t.Error("query parameters must not be logged")
	}
}

func TestRequestLoggingMapsStatusToSeverity(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		expectedLevel string
	}{
		{
			name:          "successful request",
			status:        http.StatusOK,
			expectedLevel: "INFO",
		},
		{
			name:          "client error",
			status:        http.StatusNotFound,
			expectedLevel: "WARN",
		},
		{
			name:          "server error",
			status:        http.StatusInternalServerError,
			expectedLevel: "ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appLogger, output := newRequestTestLogger()

			middleware, err := NewRequestLoggingMiddleware(appLogger)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			})

			handler := middleware(next)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, req)

			event := readLogEvent(t, output)

			if event["level"] != tt.expectedLevel {
				t.Errorf(
					"expected %s, got %v",
					tt.expectedLevel,
					event["level"],
				)
			}
		})
	}
}

func TestRequestLoggingGeneratesServerOwnedID(t *testing.T) {
	appLogger, _ := newRequestTestLogger()

	middleware, err := NewRequestLoggingMiddleware(appLogger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler := middleware(http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set(RequestIDHeader, "client-supplied-id")

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	requestID := recorder.Header().Get(RequestIDHeader)

	if requestID == "client-supplied-id" {
		t.Fatal("server must not trust an arbitrary client request ID")
	}

	if len(requestID) != 32 {
		t.Fatalf("invalid generated request ID: %q", requestID)
	}
}

func TestRequestLoggingRejectsNilLogger(t *testing.T) {
	_, err := NewRequestLoggingMiddleware(nil)

	if err == nil {
		t.Fatal("expected error for nil logger")
	}
}

func TestRequestLoggingTracksImplicitOK(t *testing.T) {
	appLogger, output := newRequestTestLogger()

	middleware, err := NewRequestLoggingMiddleware(appLogger)
	if err != nil {
		t.Fatal(err)
	}

	handler := middleware(http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	event := readLogEvent(t, output)

	if event["status"] != float64(http.StatusOK) {
		t.Errorf("expected implicit 200, got %v", event["status"])
	}
}

func TestRequestLoggingRecordsPanic(t *testing.T) {
	appLogger, output := newRequestTestLogger()

	middleware, err := NewRequestLoggingMiddleware(appLogger)
	if err != nil {
		t.Fatal(err)
	}

	handler := middleware(http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		panic("test panic")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	recorder := httptest.NewRecorder()

	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("expected handler panic to propagate")
			}
		}()

		handler.ServeHTTP(recorder, req)
	}()

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", recorder.Code)
	}

	event := readLogEvent(t, output)

	if event["level"] != "ERROR" {
		t.Errorf("expected ERROR, got %v", event["level"])
	}

	if event["panicked"] != true {
		t.Error("expected panicked=true")
	}

	if strings.Contains(output.String(), "test panic") {
		t.Error("panic payload must not be exposed in logs")
	}
}
