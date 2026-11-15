package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	handler := NewHandler()

	tests := []struct {
		name           string
		method         string
		expectedStatus int
	}{
		{
			name:           "GET health",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST health",
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/health", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Fatalf(
					"expected status %d, got %d",
					tt.expectedStatus,
					rec.Code,
				)
			}

			if tt.method == http.MethodGet {
				var response healthResponse

				if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				if response.Status != "ok" {
					t.Errorf("expected status ok, got %s", response.Status)
				}

				if response.Service != "omicx-api" {
					t.Errorf("unexpected service: %s", response.Service)
				}
			}
		})
	}
}
