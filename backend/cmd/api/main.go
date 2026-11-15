package main

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/D-engahmed/omicx/internal/platform/httpserver"
)

func main() {
	handler := httpserver.NewHandler()

	server := &http.Server{
		Addr:              ":8080",
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Println("omicx API starting on :8080")

	if err := server.ListenAndServe(); err != nil &&
		!errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("Server failed: %v", err)
	}
}
