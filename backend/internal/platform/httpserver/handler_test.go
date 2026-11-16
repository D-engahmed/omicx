package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type testReadinessChecker struct {
	err   error
	calls int
}

func (checker *testReadinessChecker) Ping(
	ctx context.Context,
) error {
	checker.calls++
	return checker.err
}

func TestHealthEndpoint(t *testing.T) {
	checker := &testReadinessChecker{}
	handler := NewHandler(checker)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	if checker.calls != 0 {
		t.Fatal("liveness endpoint must not query the database")
	}
}

func TestReadinessEndpoint(t *testing.T) {
	tests := []struct {
		name           string
		checker        *testReadinessChecker
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "database available",
			checker:        &testReadinessChecker{},
			expectedStatus: http.StatusOK,
			expectedBody:   "ready",
		},
		{
			name: "database unavailable",
			checker: &testReadinessChecker{
				err: errors.New("database unavailable"),
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedBody:   "not_ready",
		},
		{
			name:           "no database configured",
			checker:        nil,
			expectedStatus: http.StatusServiceUnavailable,
			expectedBody:   "not_ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var readiness ReadinessChecker

			if tt.checker != nil {
				readiness = tt.checker
			}

			handler := NewHandler(readiness)

			req := httptest.NewRequest(
				http.MethodGet,
				"/ready",
				nil,
			)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Fatalf(
					"expected status %d, got %d",
					tt.expectedStatus,
					rec.Code,
				)
			}

			if tt.checker != nil && tt.checker.calls != 1 {
				t.Errorf(
					"expected one database ping, got %d",
					tt.checker.calls,
				)
			}

			var response healthResponse

			if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if response.Status != tt.expectedBody {
				t.Errorf(
					"expected %s, got %s",
					tt.expectedBody,
					response.Status,
				)
			}
		})
	}
}

func TestHealthEndpointsRejectUnsupportedMethods(t *testing.T) {
	handler := NewHandler(&testReadinessChecker{})

	paths := []string{"/health", "/ready"}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf(
					"expected 405, got %d",
					rec.Code,
				)
			}
		})
	}
}
