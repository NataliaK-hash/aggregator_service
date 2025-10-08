package app

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"aggregator/internal/logging"
)

// ShutdownManager координирует логику корректного завершения работы приложения.
type ShutdownManager struct {
	timeout time.Duration
	logger  *logging.Logger
	signals <-chan os.Signal
	once    sync.Once
	cleanup func()
}

// ShutdownOption настраивает ShutdownManager.
type ShutdownOption func(*ShutdownManager)

// WithSignalChannel позволяет передать пользовательский канал для получения сигналов завершения.
func WithSignalChannel(ch <-chan os.Signal) ShutdownOption {
	return func(sm *ShutdownManager) {
		sm.signals = ch
	}
}

// NewShutdownManager создаёт ShutdownManager с заданным таймаутом.
func NewShutdownManager(timeout time.Duration, logger *logging.Logger, opts ...ShutdownOption) *ShutdownManager {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	sm := &ShutdownManager{
		timeout: timeout,
		logger:  logger,
	}

	for _, opt := range opts {
		opt(sm)
	}

	if sm.signals == nil {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sm.signals = sigCh
		sm.cleanup = func() { signal.Stop(sigCh) }
	}

	return sm
}

// WithContext возвращает контекст, который отменяется при завершении родительского контекста или получении сигнала остановки.
func (sm *ShutdownManager) WithContext(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)

	go func() {
		select {
		case <-parent.Done():
			cancel()
		case sig := <-sm.signals:
			if sm.logger != nil {
				sm.logger.Info("shutdown signal received", "signal", sig.String())
			}
			cancel()
		}
	}()

	return ctx, cancel
}

// CleanupContext предоставляет контекст с настроенным таймаутом для завершения процедур остановки.
func (sm *ShutdownManager) CleanupContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), sm.timeout)
}

// Close освобождает ресурсы, связанные с ShutdownManager.
func (sm *ShutdownManager) Close() {
	sm.once.Do(func() {
		if sm.cleanup != nil {
			sm.cleanup()
		}
	})
}

// WaitFor ожидает завершения переданного канала или возвращает ошибку отмены контекста.
func (sm *ShutdownManager) WaitFor(ctx context.Context, done <-chan struct{}) error {
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
