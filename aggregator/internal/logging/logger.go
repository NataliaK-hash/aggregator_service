package logging

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Logger представляет обёртку над slog.Logger с удобными помощниками для структурированного логирования и трассировки.
type Logger struct {
	logger *slog.Logger
}

// Option описывает параметр конфигурации логгера.
type Option func(*options)

type options struct {
	writer io.Writer
}

// WithWriter позволяет задать пользовательский io.Writer для вывода логов.
func WithWriter(w io.Writer) Option {
	return func(o *options) {
		o.writer = w
	}
}

// New создаёт новый структурированный JSON-логгер с указанным уровнем логирования.
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

// parseLevel преобразует строковое представление уровня логирования в slog.Level.
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

// WithTraceID возвращает логгер с добавленным указанным идентификатором трассы.
func (l *Logger) WithTraceID(traceID string) *Logger {
	if l == nil {
		return nil
	}
	return &Logger{logger: l.logger.With("traceId", traceID)}
}

// WithContext прикрепляет логгер к переданному контексту.
func (l *Logger) WithContext(ctx context.Context) context.Context {
	if l == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxLoggerKey{}, l)
}

// FromContext извлекает логгер из контекста, если он туда был помещён.
func FromContext(ctx context.Context) (*Logger, bool) {
	if ctx == nil {
		return nil, false
	}
	logger, ok := ctx.Value(ctxLoggerKey{}).(*Logger)
	return logger, ok
}

type ctxLoggerKey struct{}

// Debug записывает сообщение уровня debug.
func (l *Logger) Debug(msg string, args ...any) {
	l.log(slog.LevelDebug, msg, args...)
}

// Info записывает сообщение уровня info.
func (l *Logger) Info(msg string, args ...any) {
	l.log(slog.LevelInfo, msg, args...)
}

// Warn записывает сообщение уровня warn.
func (l *Logger) Warn(msg string, args ...any) {
	l.log(slog.LevelWarn, msg, args...)
}

// Error записывает сообщение уровня error.
func (l *Logger) Error(msg string, args ...any) {
	l.log(slog.LevelError, msg, args...)
}

// log выполняет фактическую отправку сообщения в базовый slog.Logger.
func (l *Logger) log(level slog.Level, msg string, args ...any) {
	if l == nil || l.logger == nil {
		return
	}
	l.logger.Log(context.Background(), level, msg, args...)
}

// ErrorWithTrace записывает сообщение об ошибке с передачей идентификатора трассы и деталей ошибки.
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

// SetDefault заменяет пакетный логгер по умолчанию в slog на текущий экземпляр.
func (l *Logger) SetDefault() {
	if l == nil || l.logger == nil {
		return
	}
	slog.SetDefault(l.logger)
}

// MustNew создаёт новый логгер либо паникует при ошибке конфигурации.
func MustNew(level string, opts ...Option) *Logger {
	logger, err := New(level, opts...)
	if err != nil {
		panic(err)
	}
	return logger
}

// AttachError дополняет список аргументов данными об ошибке.
func AttachError(err error, args ...any) []any {
	if err == nil {
		return args
	}
	return append(args, "error", err.Error())
}

// Validate проверяет корректность настроек логгера.
func (l *Logger) Validate() error {
	if l == nil || l.logger == nil {
		return errors.New("logger is not initialized")
	}
	return nil
}
