package logging

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
)

type Logger struct {
	logger *slog.Logger
}

type Option func(*options)

type options struct {
	writer io.Writer
}

func WithWriter(w io.Writer) Option {
	return func(o *options) {
		o.writer = w
	}
}

func New(level string, opts ...Option) (*Logger, error) {
	cfg := options{}
	for _, opt := range opts {
		opt(&cfg)
	}

	handlerOptions := &slog.HandlerOptions{Level: parseLevel(level)}
	writer := cfg.writer
	if writer == nil {
		writer = os.Stdout
	}

	handler := slog.NewJSONHandler(writer, handlerOptions)

	return &Logger{logger: slog.New(handler)}, nil
}

func parseLevel(level string) slog.Leveler {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func (l *Logger) WithTraceID(traceID string) *Logger {
	if l == nil {
		return nil
	}
	return &Logger{logger: l.logger.With("traceId", traceID)}
}

func (l *Logger) WithContext(ctx context.Context) context.Context {
	if l == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxLoggerKey{}, l)
}

func FromContext(ctx context.Context) (*Logger, bool) {
	if ctx == nil {
		return nil, false
	}
	logger, ok := ctx.Value(ctxLoggerKey{}).(*Logger)
	return logger, ok
}

type ctxLoggerKey struct{}

func (l *Logger) Debug(msg string, args ...any) {
	l.log(slog.LevelDebug, msg, args...)
}

func (l *Logger) Info(msg string, args ...any) {
	l.log(slog.LevelInfo, msg, args...)
}

func (l *Logger) Warn(msg string, args ...any) {
	l.log(slog.LevelWarn, msg, args...)
}

func (l *Logger) Error(msg string, args ...any) {
	l.log(slog.LevelError, msg, args...)
}

func (l *Logger) log(level slog.Level, msg string, args ...any) {
	if l == nil || l.logger == nil {
		return
	}
	l.logger.Log(context.Background(), level, msg, args...)
}

func (l *Logger) ErrorWithTrace(traceID string, err error) {
	if l == nil {
		return
	}
	attrs := []any{"traceId", traceID}
	if err != nil {
		attrs = append(attrs, "error", err.Error())
	}
	l.Error("operation failed", attrs...)
}

func (l *Logger) SetDefault() {
	if l == nil || l.logger == nil {
		return
	}
	slog.SetDefault(l.logger)
}

func MustNew(level string, opts ...Option) *Logger {
	logger, err := New(level, opts...)
	if err != nil {
		panic(err)
	}
	return logger
}

func AttachError(err error, args ...any) []any {
	if err == nil {
		return args
	}
	return append(args, "error", err.Error())
}

func (l *Logger) Validate() error {
	if l == nil || l.logger == nil {
		return errors.New("logger is not initialized")
	}
	return nil
}
