package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

type logEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Service   string `json:"service,omitempty"`
	TraceID   string `json:"trace_id,omitempty"`
}

func TestLoggerPrintfIncludesCorrelationID(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, "test-service")

	ctx := WithCorrelationID(context.Background(), "trace-123")
	logger.Printf(ctx, "hello %s", "world")

	var entry logEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to unmarshal log entry: %v", err)
	}

	if entry.Level != "info" {
		t.Fatalf("expected level info, got %s", entry.Level)
	}
	if entry.Message != "hello world" {
		t.Fatalf("unexpected message: %s", entry.Message)
	}
	if entry.Service != "test-service" {
		t.Fatalf("unexpected service: %s", entry.Service)
	}
	if entry.TraceID != "trace-123" {
		t.Fatalf("expected trace id trace-123, got %s", entry.TraceID)
	}
	if strings.TrimSpace(entry.Timestamp) == "" {
		t.Fatalf("expected timestamp to be populated")
	}
}

func TestLoggerPrintlnOmitsEmptyTraceID(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, "")

	logger.Println(context.Background(), "message")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to decode log entry: %v", err)
	}

	if _, exists := entry["trace_id"]; exists {
		t.Fatalf("expected trace_id to be omitted")
	}
	if entry["message"] != "message" {
		t.Fatalf("unexpected message: %v", entry["message"])
	}
}

func TestNewLoggerDefaults(t *testing.T) {
	logger := NewLogger(nil, " ")
	if logger == nil {
		t.Fatal("expected logger to be created")
	}
	logger.Printf(context.Background(), "hello")
}

func TestWithCorrelationIDHandlesNilContext(t *testing.T) {
	ctx := WithCorrelationID(nil, " id ")
	if got := CorrelationIDFromContext(ctx); got != "id" {
		t.Fatalf("expected id, got %s", got)
	}
}

func TestCorrelationIDFromContextMissing(t *testing.T) {
	if CorrelationIDFromContext(nil) != "" {
		t.Fatalf("expected empty id")
	}
}

func TestLoggerFatalfExits(t *testing.T) {
	if os.Getenv("LOGGER_FATALF_SUBPROCESS") == "1" {
		logger := NewLogger(os.Stdout, "test")
		logger.Fatalf(context.Background(), "fatal")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestLoggerFatalfExits")
	cmd.Env = append(os.Environ(), "LOGGER_FATALF_SUBPROCESS=1")

	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected process to exit with error")
	}
}
