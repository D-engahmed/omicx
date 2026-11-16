package logger

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// Options defines the configuration required to construct a logger.
type Options struct {
	Environment string
	Level       string
	Output      io.Writer
}

// New creates a structured logger using the application environment
// and configured severity level.
func New(opts Options) (*slog.Logger, error) {
	if opts.Output == nil {
		return nil, errors.New("logger output must not be nil")
	}

	environment := strings.ToLower(strings.TrimSpace(opts.Environment))

	level, err := parseLevel(opts.Level)
	if err != nil {
		return nil, err
	}

	handlerOptions := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler

	switch environment {
	case "development":
		handlerOptions.AddSource = true
		handler = slog.NewTextHandler(opts.Output, handlerOptions)

	case "test", "staging", "production":
		handler = slog.NewJSONHandler(opts.Output, handlerOptions)

	default:
		return nil, fmt.Errorf(
			"unsupported logger environment: %q",
			environment,
		)
	}

	return slog.New(handler), nil
}

// parseLevel converts the configured string into a slog severity level.
func parseLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug, nil

	case "info":
		return slog.LevelInfo, nil

	case "warn":
		return slog.LevelWarn, nil

	case "error":
		return slog.LevelError, nil

	default:
		return 0, fmt.Errorf(
			"invalid log level %q: expected debug, info, warn, or error",
			value,
		)
	}
}
