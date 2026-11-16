package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultMaxConnLifetime   = time.Hour
	defaultMaxConnIdleTime   = 30 * time.Minute
	defaultHealthCheckPeriod = time.Minute
)

type PoolConfig struct {
	URL            string
	MaxConns       int32
	MinConns       int32
	ConnectTimeout time.Duration
}

// NewPool creates a PostgreSQL connection pool and verifies
// that the database is reachable before returning it.
func NewPool(
	ctx context.Context,
	cfg PoolConfig,
) (*pgxpool.Pool, error) {
	if ctx == nil {
		return nil, errors.New("database context must not be nil")
	}

	poolConfig, err := parsePoolConfig(cfg)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL pool: %w", err)
	}

	// Creating a pool does not guarantee that a connection
	// to PostgreSQL has actually been established.
	pingCtx, cancel := context.WithTimeout(
		ctx,
		cfg.ConnectTimeout,
	)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()

		return nil, fmt.Errorf(
			"verify PostgreSQL connectivity: %w",
			err,
		)
	}

	return pool, nil
}

// parsePoolConfig validates application configuration and
// converts it into a pgxpool configuration.
func parsePoolConfig(
	cfg PoolConfig,
) (*pgxpool.Config, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, errors.New("DATABASE_URL must not be empty")
	}

	if cfg.MaxConns < 1 {
		return nil, errors.New(
			"DATABASE_MAX_CONNS must be greater than zero",
		)
	}

	if cfg.MinConns < 0 {
		return nil, errors.New(
			"DATABASE_MIN_CONNS must not be negative",
		)
	}

	if cfg.MinConns > cfg.MaxConns {
		return nil, errors.New(
			"DATABASE_MIN_CONNS cannot exceed DATABASE_MAX_CONNS",
		)
	}

	if cfg.ConnectTimeout <= 0 {
		return nil, errors.New(
			"DATABASE_CONNECT_TIMEOUT must be greater than zero",
		)
	}

	poolConfig, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		// Avoid including the raw connection string in our own
		// error message because it can contain credentials.
		return nil, fmt.Errorf(
			"invalid PostgreSQL connection configuration: %w",
			err,
		)
	}

	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns

	poolConfig.MaxConnLifetime = defaultMaxConnLifetime
	poolConfig.MaxConnIdleTime = defaultMaxConnIdleTime
	poolConfig.HealthCheckPeriod = defaultHealthCheckPeriod

	poolConfig.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	poolConfig.PingTimeout = cfg.ConnectTimeout

	return poolConfig, nil
}
