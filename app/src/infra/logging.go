package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

type contextKey string

const correlationIDKey contextKey = "correlation_id"

// Logger provides JSON structured logging with correlation identifiers sourced from context.
type Logger struct {
	mu      sync.Mutex
	out     io.Writer
	service string
}

// NewLogger constructs a structured logger writing JSON entries to the provided writer.
func NewLogger(out io.Writer, service string) *Logger {
	if out == nil {
		out = io.Discard
	}
	return &Logger{out: out, service: strings.TrimSpace(service)}
}

// WithCorrelationID stores the provided correlation identifier within the context.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, correlationIDKey, strings.TrimSpace(id))
}

// CorrelationIDFromContext extracts the correlation identifier from context when present.
func CorrelationIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if value, ok := ctx.Value(correlationIDKey).(string); ok {
		return value
	}
	return ""
}

// Printf writes a structured log entry with fmt.Sprintf semantics.
func (l *Logger) Printf(ctx context.Context, format string, v ...any) {
	if l == nil {
		return
	}
	msg := fmt.Sprintf(format, v...)
	l.log(ctx, "info", msg)
}

// Println writes a structured log entry combining the arguments with fmt.Sprintln semantics.
func (l *Logger) Println(ctx context.Context, v ...any) {
	if l == nil {
		return
	}
	msg := strings.TrimSpace(fmt.Sprintln(v...))
	l.log(ctx, "info", msg)
}

// Fatalf logs the message at fatal level and terminates the process.
func (l *Logger) Fatalf(ctx context.Context, format string, v ...any) {
	if l == nil {
		os.Exit(1)
	}
	msg := fmt.Sprintf(format, v...)
	l.log(ctx, "fatal", msg)
	os.Exit(1)
}

type entry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Service   string `json:"service,omitempty"`
	TraceID   string `json:"trace_id,omitempty"`
}

func (l *Logger) log(ctx context.Context, level, msg string) {
	traceID := CorrelationIDFromContext(ctx)
	record := entry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     level,
		Message:   msg,
		Service:   l.service,
		TraceID:   traceID,
	}

	payload, err := json.Marshal(record)
	if err != nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = l.out.Write(append(payload, '\n'))
}
