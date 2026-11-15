package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := load(func(string) (string, bool) {
		return "", false
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.AppEnv != "development" {
		t.Errorf("expected development, got %s", cfg.AppEnv)
	}

	if cfg.HTTPAddr != ":8080" {
		t.Errorf("expected :8080, got %s", cfg.HTTPAddr)
	}

	if cfg.HTTPReadHeaderTimeout != 5*time.Second {
		t.Errorf("unexpected timeout: %v", cfg.HTTPReadHeaderTimeout)
	}
}

func TestLoadCustomConfiguration(t *testing.T) {
	values := map[string]string{
		"APP_ENV":                  "production",
		"HTTP_ADDR":                "127.0.0.1:9090",
		"HTTP_READ_HEADER_TIMEOUT": "10s",
	}

	cfg, err := load(func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.AppEnv != "production" {
		t.Errorf("expected production, got %s", cfg.AppEnv)
	}

	if cfg.HTTPAddr != "127.0.0.1:9090" {
		t.Errorf("unexpected address: %s", cfg.HTTPAddr)
	}

	if cfg.HTTPReadHeaderTimeout != 10*time.Second {
		t.Errorf("unexpected timeout: %v", cfg.HTTPReadHeaderTimeout)
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
	}{
		{
			name: "invalid environment",
			values: map[string]string{
				"APP_ENV": "unknown",
			},
		},
		{
			name: "invalid HTTP address",
			values: map[string]string{
				"HTTP_ADDR": "invalid-address",
			},
		},
		{
			name: "invalid HTTP port",
			values: map[string]string{
				"HTTP_ADDR": ":70000",
			},
		},
		{
			name: "invalid timeout",
			values: map[string]string{
				"HTTP_READ_HEADER_TIMEOUT": "invalid",
			},
		},
		{
			name: "zero timeout",
			values: map[string]string{
				"HTTP_READ_HEADER_TIMEOUT": "0s",
			},
		},
		{
			name: "empty HTTP address",
			values: map[string]string{
				"HTTP_ADDR": "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := load(func(key string) (string, bool) {
				value, ok := tt.values[key]
				return value, ok
			})

			if err == nil {
				t.Fatal("expected configuration error, got nil")
			}
		})
	}
}
