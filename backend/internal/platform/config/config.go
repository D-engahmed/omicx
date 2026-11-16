package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAppEnv                 = "development"
	defaultHTTPAddr               = ":8080"
	defaultHTTPReadHeaderTimeout  = "5s"
	defaultHTTPShutdownTimeout    = "10s"
	defaultDatabaseMaxConns       = "10"
	defaultDatabaseMinConns       = "0"
	defaultDatabaseConnectTimeout = "5s"
)

type Config struct {
	AppEnv                 string
	HTTPAddr               string
	HTTPReadHeaderTimeout  time.Duration
	HTTPShutdownTimeout    time.Duration
	LogLevel               string
	DatabaseURL            string
	DatabaseMaxConns       int32
	DatabaseMinConns       int32
	DatabaseConnectTimeout time.Duration
}

// Load reads and validates configuration from environment variables.
func Load() (Config, error) {
	return load(os.LookupEnv)
}

// load accepts an environment lookup function to make configuration
// independently testable without modifying process environment variables.
func load(lookup func(string) (string, bool)) (Config, error) {
	// Application environment
	appEnv, err := readValue(lookup, "APP_ENV", defaultAppEnv)
	if err != nil {
		return Config{}, err
	}

	switch appEnv {
	case "development", "test", "staging", "production":
		// Supported environments.
	default:
		return Config{}, fmt.Errorf(
			"APP_ENV must be development, test, staging, or production",
		)
	}

	// Logging configuration
	logLevel, err := readValue(
		lookup,
		"LOG_LEVEL",
		defaultLogLevel(appEnv),
	)
	if err != nil {
		return Config{}, err
	}

	logLevel = strings.ToLower(logLevel)

	switch logLevel {
	case "debug", "info", "warn", "error":
		// Supported logging levels.
	default:
		return Config{}, fmt.Errorf(
			"LOG_LEVEL must be debug, info, warn, or error",
		)
	}

	// HTTP server address
	httpAddr, err := readValue(
		lookup,
		"HTTP_ADDR",
		defaultHTTPAddr,
	)
	if err != nil {
		return Config{}, err
	}

	if err := validateHTTPAddr(httpAddr); err != nil {
		return Config{}, err
	}

	// HTTP read header timeout
	timeoutValue, err := readValue(
		lookup,
		"HTTP_READ_HEADER_TIMEOUT",
		defaultHTTPReadHeaderTimeout,
	)
	if err != nil {
		return Config{}, err
	}

	readHeaderTimeout, err := time.ParseDuration(timeoutValue)
	if err != nil {
		return Config{}, fmt.Errorf(
			"HTTP_READ_HEADER_TIMEOUT must be a valid duration: %w",
			err,
		)
	}

	if readHeaderTimeout <= 0 {
		return Config{}, fmt.Errorf(
			"HTTP_READ_HEADER_TIMEOUT must be greater than zero",
		)
	}

	// HTTP shutdown timeout
	shutdownTimeoutValue, err := readValue(
		lookup,
		"HTTP_SHUTDOWN_TIMEOUT",
		defaultHTTPShutdownTimeout,
	)
	if err != nil {
		return Config{}, err
	}

	shutdownTimeout, err := time.ParseDuration(shutdownTimeoutValue)
	if err != nil {
		return Config{}, fmt.Errorf(
			"HTTP_SHUTDOWN_TIMEOUT must be a valid duration: %w",
			err,
		)
	}

	if shutdownTimeout <= 0 {
		return Config{}, fmt.Errorf(
			"HTTP_SHUTDOWN_TIMEOUT must be greater than zero",
		)
	}
	// Database connection URL.
	// Optional at the config parsing stage; required by the API startup.
	databaseURL, err := readValue(
		lookup,
		"DATABASE_URL",
		"",
	)
	if err != nil {
		return Config{}, err
	}

	// Maximum database connections.
	maxConnsValue, err := readValue(
		lookup,
		"DATABASE_MAX_CONNS",
		defaultDatabaseMaxConns,
	)
	if err != nil {
		return Config{}, err
	}

	maxConns, err := strconv.ParseInt(maxConnsValue, 10, 32)
	if err != nil || maxConns < 1 {
		return Config{}, fmt.Errorf(
			"DATABASE_MAX_CONNS must be a positive integer",
		)
	}

	// Minimum database connections.
	minConnsValue, err := readValue(
		lookup,
		"DATABASE_MIN_CONNS",
		defaultDatabaseMinConns,
	)
	if err != nil {
		return Config{}, err
	}

	minConns, err := strconv.ParseInt(minConnsValue, 10, 32)
	if err != nil || minConns < 0 {
		return Config{}, fmt.Errorf(
			"DATABASE_MIN_CONNS must be a non-negative integer",
		)
	}

	if minConns > maxConns {
		return Config{}, fmt.Errorf(
			"DATABASE_MIN_CONNS cannot exceed DATABASE_MAX_CONNS",
		)
	}

	// Database connection timeout.
	connectTimeoutValue, err := readValue(
		lookup,
		"DATABASE_CONNECT_TIMEOUT",
		defaultDatabaseConnectTimeout,
	)
	if err != nil {
		return Config{}, err
	}

	connectTimeout, err := time.ParseDuration(connectTimeoutValue)
	if err != nil || connectTimeout <= 0 {
		return Config{}, fmt.Errorf(
			"DATABASE_CONNECT_TIMEOUT must be a valid positive duration",
		)
	}
	return Config{
		AppEnv:                 appEnv,
		HTTPAddr:               httpAddr,
		HTTPReadHeaderTimeout:  readHeaderTimeout,
		HTTPShutdownTimeout:    shutdownTimeout,
		LogLevel:               logLevel,
		DatabaseURL:            databaseURL,
		DatabaseMaxConns:       int32(maxConns),
		DatabaseMinConns:       int32(minConns),
		DatabaseConnectTimeout: connectTimeout,
	}, nil
}

// defaultLogLevel determines the default logging level
// based on the application environment.
func defaultLogLevel(appEnv string) string {
	if appEnv == "development" {
		return "debug"
	}

	return "info"
}

// readValue retrieves an environment variable or uses its default value.
func readValue(
	lookup func(string) (string, bool),
	key string,
	defaultValue string,
) (string, error) {
	value, exists := lookup(key)

	if !exists {
		return defaultValue, nil
	}

	value = strings.TrimSpace(value)

	if value == "" {
		return "", fmt.Errorf("%s must not be empty", key)
	}

	return value, nil
}

// validateHTTPAddr checks the host:port format and validates the port.
func validateHTTPAddr(addr string) error {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf(
			"HTTP_ADDR must use host:port format: %w",
			err,
		)
	}

	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return fmt.Errorf(
			"HTTP_ADDR port must be between 1 and 65535",
		)
	}

	return nil
}
