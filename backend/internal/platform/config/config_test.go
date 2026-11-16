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
		t.Errorf(
			"expected timeout 5s, got %v",
			cfg.HTTPReadHeaderTimeout,
		)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("expected debug, got %s", cfg.LogLevel)
	}

	if cfg.HTTPShutdownTimeout != 10*time.Second {
		t.Errorf(
			"expected shutdown timeout 10s, got %v",
			cfg.HTTPShutdownTimeout,
		)
	}
}

func TestLoadCustomConfiguration(t *testing.T) {
	values := map[string]string{
		"APP_ENV":                  "production",
		"HTTP_ADDR":                "127.0.0.1:9090",
		"HTTP_READ_HEADER_TIMEOUT": "10s",
		"LOG_LEVEL":                "warn",
		"HTTP_SHUTDOWN_TIMEOUT":    "20s",
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
		t.Errorf(
			"expected timeout 10s, got %v",
			cfg.HTTPReadHeaderTimeout,
		)
	}

	if cfg.LogLevel != "warn" {
		t.Errorf("expected warn, got %s", cfg.LogLevel)
	}
	if cfg.HTTPShutdownTimeout != 20*time.Second {
		t.Errorf(
			"expected shutdown timeout 20s, got %v",
			cfg.HTTPShutdownTimeout,
		)
	}
}

func TestDefaultLogLevelByEnvironment(t *testing.T) {
	tests := []struct {
		environment   string
		expectedLevel string
	}{
		{
			environment:   "development",
			expectedLevel: "debug",
		},
		{
			environment:   "test",
			expectedLevel: "info",
		},
		{
			environment:   "staging",
			expectedLevel: "info",
		},
		{
			environment:   "production",
			expectedLevel: "info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.environment, func(t *testing.T) {
			values := map[string]string{
				"APP_ENV": tt.environment,
			}

			cfg, err := load(func(key string) (string, bool) {
				value, ok := values[key]
				return value, ok
			})

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if cfg.LogLevel != tt.expectedLevel {
				t.Errorf(
					"expected %s, got %s",
					tt.expectedLevel,
					cfg.LogLevel,
				)
			}
		})
	}
}

func TestLogLevelNormalization(t *testing.T) {
	values := map[string]string{
		"LOG_LEVEL": "  DEBUG  ",
	}

	cfg, err := load(func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("expected normalized debug, got %s", cfg.LogLevel)
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
			name: "zero HTTP port",
			values: map[string]string{
				"HTTP_ADDR": ":0",
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
			name: "negative timeout",
			values: map[string]string{
				"HTTP_READ_HEADER_TIMEOUT": "-5s",
			},
		},
		{
			name: "empty HTTP address",
			values: map[string]string{
				"HTTP_ADDR": "",
			},
		},
		{
			name: "invalid log level",
			values: map[string]string{
				"LOG_LEVEL": "verbose",
			},
		},
		{
			name: "empty log level",
			values: map[string]string{
				"LOG_LEVEL": "",
			},
		},

		{
			name: "invalid shutdown timeout",
			values: map[string]string{
				"HTTP_SHUTDOWN_TIMEOUT": "invalid",
			},
		},
		{
			name: "zero shutdown timeout",
			values: map[string]string{
				"HTTP_SHUTDOWN_TIMEOUT": "0s",
			},
		},
		{
			name: "negative shutdown timeout",
			values: map[string]string{
				"HTTP_SHUTDOWN_TIMEOUT": "-5s",
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
