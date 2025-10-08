package infra

import (
	"bytes"
	"context"
	"encoding/json"
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
