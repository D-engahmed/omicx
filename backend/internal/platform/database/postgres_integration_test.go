package database

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestNewPoolConnectsToPostgres(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")

	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	pool, err := NewPool(ctx, PoolConfig{
		URL:            databaseURL,
		MaxConns:       3,
		MinConns:       0,
		ConnectTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create PostgreSQL pool: %v", err)
	}
	defer pool.Close()

	var databaseName string

	err = pool.QueryRow(
		ctx,
		"SELECT current_database()",
	).Scan(&databaseName)

	if err != nil {
		t.Fatalf("failed to query PostgreSQL: %v", err)
	}

	if databaseName != "omicx" {
		t.Fatalf(
			"expected database omicx, got %s",
			databaseName,
		)
	}
}
