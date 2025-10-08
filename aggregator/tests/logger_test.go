package tests

import (
	"aggregator/internal/logging"
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

type logEntry struct {
	Level   string `json:"level"`
	Message string `json:"msg"`
	TraceID string `json:"traceId"`
}

// TestLoggerHonorsLogLevel проверяет соблюдение уровней логирования.
func TestLoggerHonorsLogLevel(t *testing.T) {
	buf := &bytes.Buffer{}
	logger, err := logging.New("info", logging.WithWriter(buf))
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	logger.Debug("debug message")
	if buf.Len() != 0 {
		t.Fatalf("expected no output for debug message at info level")
	}

	logger.Info("info message")
	if buf.Len() == 0 {
		t.Fatalf("expected output for info message")
	}
}

// TestLoggerTraceID удостоверяется, что идентификатор трассы попадает в вывод.
func TestLoggerTraceID(t *testing.T) {
	buf := &bytes.Buffer{}
	logger, err := logging.New("debug", logging.WithWriter(buf))
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	traceLogger := logger.WithTraceID("trace-123")
	traceLogger.Info("with trace")

	entries := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(entries) == 0 {
		t.Fatalf("expected at least one log entry")
	}

	var entry logEntry
	if err := json.Unmarshal([]byte(entries[len(entries)-1]), &entry); err != nil {
		t.Fatalf("failed to unmarshal log entry: %v", err)
	}

	if entry.TraceID != "trace-123" {
		t.Fatalf("expected trace id trace-123, got %s", entry.TraceID)
	}
}
