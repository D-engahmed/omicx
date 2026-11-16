package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNewDevelopmentUsesTextOutput(t *testing.T) {
	var output bytes.Buffer

	log, err := New(Options{
		Environment: "development",
		Level:       "debug",
		Output:      &output,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	log.Info("server started", "port", 8080)

	result := output.String()

	if !strings.Contains(result, "level=INFO") {
		t.Errorf("expected INFO level in output, got: %s", result)
	}

	if !strings.Contains(result, "msg=") {
		t.Errorf("expected message field in output, got: %s", result)
	}

	if !strings.Contains(result, "port=8080") {
		t.Errorf("expected port field in output, got: %s", result)
	}

	if json.Valid([]byte(strings.TrimSpace(result))) {
		t.Error("expected development output to be text, not JSON")
	}
}

func TestNewProductionUsesJSONOutput(t *testing.T) {
	var output bytes.Buffer

	log, err := New(Options{
		Environment: "production",
		Level:       "info",
		Output:      &output,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	log.Info("server started", "port", 8080)

	var result map[string]any

	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON output: %v", err)
	}

	if result["level"] != "INFO" {
		t.Errorf("expected INFO level, got %v", result["level"])
	}

	if result["msg"] != "server started" {
		t.Errorf("unexpected message: %v", result["msg"])
	}

	if result["port"] != float64(8080) {
		t.Errorf("unexpected port: %v", result["port"])
	}
}

func TestLogLevelFiltering(t *testing.T) {
	var output bytes.Buffer

	log, err := New(Options{
		Environment: "development",
		Level:       "warn",
		Output:      &output,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	log.Debug("debug event")
	log.Info("info event")
	log.Warn("warning event")
	log.Error("error event")

	result := output.String()

	if strings.Contains(result, "debug event") {
		t.Error("DEBUG event should have been filtered")
	}

	if strings.Contains(result, "info event") {
		t.Error("INFO event should have been filtered")
	}

	if !strings.Contains(result, "warning event") {
		t.Error("WARN event should have been logged")
	}

	if !strings.Contains(result, "error event") {
		t.Error("ERROR event should have been logged")
	}
}

func TestNewRejectsInvalidLogLevel(t *testing.T) {
	var output bytes.Buffer

	_, err := New(Options{
		Environment: "production",
		Level:       "verbose",
		Output:      &output,
	})

	if err == nil {
		t.Fatal("expected error for invalid log level")
	}
}

func TestNewRejectsInvalidEnvironment(t *testing.T) {
	var output bytes.Buffer

	_, err := New(Options{
		Environment: "unknown",
		Level:       "info",
		Output:      &output,
	})

	if err == nil {
		t.Fatal("expected error for unsupported environment")
	}
}

func TestNewRejectsNilOutput(t *testing.T) {
	_, err := New(Options{
		Environment: "development",
		Level:       "debug",
		Output:      nil,
	})

	if err == nil {
		t.Fatal("expected error for nil output")
	}
}

func TestNewNormalizesOptions(t *testing.T) {
	var output bytes.Buffer

	log, err := New(Options{
		Environment: " DEVELOPMENT ",
		Level:       " DEBUG ",
		Output:      &output,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	log.Debug("normalized configuration")

	if !strings.Contains(output.String(), "normalized configuration") {
		t.Error("expected normalized configuration to be accepted")
	}
}
