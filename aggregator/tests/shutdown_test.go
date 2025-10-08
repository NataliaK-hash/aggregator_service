package tests

import (
	"aggregator/internal/app"
	"context"
	"io"
	"os"
	"syscall"
	"testing"
	"time"

	"aggregator/internal/logging"
)

// TestShutdownContextCancelledOnSignal проверяет отмену контекста при получении сигнала.
func TestShutdownContextCancelledOnSignal(t *testing.T) {
	signalCh := make(chan os.Signal, 1)
	logger, _ := logging.New("debug", logging.WithWriter(io.Discard))
	manager := app.NewShutdownManager(50*time.Millisecond, logger, app.WithSignalChannel(signalCh))

	ctx, cancel := manager.WithContext(context.Background())
	defer cancel()

	signalCh <- syscall.SIGTERM

	select {
	case <-ctx.Done():
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("context was not cancelled after signal")
	}
}

// TestCleanupContextTimeout убеждается, что контекст очистки использует заданный таймаут.
func TestCleanupContextTimeout(t *testing.T) {
	logger, _ := logging.New("debug", logging.WithWriter(io.Discard))
	manager := app.NewShutdownManager(25*time.Millisecond, logger, app.WithSignalChannel(make(chan os.Signal)))

	cleanupCtx, cancel := manager.CleanupContext()
	defer cancel()

	deadline, ok := cleanupCtx.Deadline()
	if !ok {
		t.Fatalf("expected cleanup context to have a deadline")
	}

	remaining := time.Until(deadline)
	if remaining > 30*time.Millisecond || remaining <= 0 {
		t.Fatalf("unexpected cleanup timeout: %s", remaining)
	}
}
