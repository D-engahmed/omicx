package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

const readinessTimeout = 2 * time.Second

type ReadinessChecker interface {
	Ping(context.Context) error
}

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

func NewHandler(readiness ReadinessChecker) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		writeHealthResponse(w, http.StatusOK, "ok")
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if readiness == nil {
			writeHealthResponse(w, http.StatusServiceUnavailable, "not_ready")
			return
		}

		ctx, cancel := context.WithTimeout(
			r.Context(),
			readinessTimeout,
		)
		defer cancel()

		if err := readiness.Ping(ctx); err != nil {
			writeHealthResponse(w, http.StatusServiceUnavailable, "not_ready")
			return
		}

		writeHealthResponse(w, http.StatusOK, "ready")
	})

	return mux
}

func writeHealthResponse(
	w http.ResponseWriter,
	statusCode int,
	status string,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	_ = json.NewEncoder(w).Encode(healthResponse{
		Status:  status,
		Service: "omicx-api",
	})
}
