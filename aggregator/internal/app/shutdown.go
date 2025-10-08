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

type ShutdownManager struct {
	timeout time.Duration
	logger  *logging.Logger
	signals <-chan os.Signal
	once    sync.Once
	cleanup func()
}

type ShutdownOption func(*ShutdownManager)

func WithSignalChannel(ch <-chan os.Signal) ShutdownOption {
	return func(sm *ShutdownManager) {
		sm.signals = ch
	}
}

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

func (sm *ShutdownManager) CleanupContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), sm.timeout)
}

func (sm *ShutdownManager) Close() {
	sm.once.Do(func() {
		if sm.cleanup != nil {
			sm.cleanup()
		}
	})
}

func (sm *ShutdownManager) WaitFor(ctx context.Context, done <-chan struct{}) error {
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
