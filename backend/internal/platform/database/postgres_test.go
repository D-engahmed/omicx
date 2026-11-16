package database

import (
	"testing"
	"time"
)

func validPoolConfig() PoolConfig {
	return PoolConfig{
		URL:            "postgres://omicx:local_dev_only_change_me@127.0.0.1:5433/omicx?sslmode=disable",
		MaxConns:       10,
		MinConns:       0,
		ConnectTimeout: 5 * time.Second,
	}
}

func TestParsePoolConfig(t *testing.T) {
	cfg := validPoolConfig()

	poolConfig, err := parsePoolConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if poolConfig.MaxConns != 10 {
		t.Errorf("expected 10 max connections, got %d", poolConfig.MaxConns)
	}

	if poolConfig.MinConns != 0 {
		t.Errorf("expected 0 minimum connections, got %d", poolConfig.MinConns)
	}

	if poolConfig.ConnConfig.ConnectTimeout != 5*time.Second {
		t.Errorf("unexpected connection timeout: %v", poolConfig.ConnConfig.ConnectTimeout)
	}

	if poolConfig.PingTimeout != 5*time.Second {
		t.Errorf("unexpected ping timeout: %v", poolConfig.PingTimeout)
	}

	if poolConfig.MaxConnLifetime != time.Hour {
		t.Errorf("unexpected connection lifetime: %v", poolConfig.MaxConnLifetime)
	}
}

func TestParsePoolConfigRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*PoolConfig)
	}{
		{
			name: "empty URL",
			mutate: func(cfg *PoolConfig) {
				cfg.URL = ""
			},
		},
		{
			name: "invalid URL",
			mutate: func(cfg *PoolConfig) {
				cfg.URL = "not-a-postgres-url"
			},
		},
		{
			name: "zero maximum connections",
			mutate: func(cfg *PoolConfig) {
				cfg.MaxConns = 0
			},
		},
		{
			name: "negative minimum connections",
			mutate: func(cfg *PoolConfig) {
				cfg.MinConns = -1
			},
		},
		{
			name: "minimum exceeds maximum",
			mutate: func(cfg *PoolConfig) {
				cfg.MinConns = 11
			},
		},
		{
			name: "zero connection timeout",
			mutate: func(cfg *PoolConfig) {
				cfg.ConnectTimeout = 0
			},
		},
		{
			name: "negative connection timeout",
			mutate: func(cfg *PoolConfig) {
				cfg.ConnectTimeout = -time.Second
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validPoolConfig()
			tt.mutate(&cfg)

			_, err := parsePoolConfig(cfg)
			if err == nil {
				t.Fatal("expected configuration error")
			}
		})
	}
}
