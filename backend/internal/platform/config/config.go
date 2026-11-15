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
	defaultAppEnv                = "development"
	defaultHTTPAddr              = ":8080"
	defaultHTTPReadHeaderTimeout = "5s"
)

type Config struct {
	AppEnv                string
	HTTPAddr              string
	HTTPReadHeaderTimeout time.Duration
}

// Load reads and validates configuration from environment variables.
func Load() (Config, error) {
	return load(os.LookupEnv)
}

// load accepts an environment lookup function to make configuration
// independently testable without modifying process environment variables.
func load(lookup func(string) (string, bool)) (Config, error) {
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

	httpAddr, err := readValue(lookup, "HTTP_ADDR", defaultHTTPAddr)
	if err != nil {
		return Config{}, err
	}

	if err := validateHTTPAddr(httpAddr); err != nil {
		return Config{}, err
	}

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

	return Config{
		AppEnv:                appEnv,
		HTTPAddr:              httpAddr,
		HTTPReadHeaderTimeout: readHeaderTimeout,
	}, nil
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
