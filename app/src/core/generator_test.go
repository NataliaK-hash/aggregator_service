package core

import (
	"aggregator-service/app/src/domain"
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type stubLogger struct {
	mu      sync.Mutex
	entries []string
}

func (l *stubLogger) Printf(_ context.Context, format string, v ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, fmt.Sprintf(format, v...))
}

func (l *stubLogger) Println(_ context.Context, v ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, fmt.Sprintln(v...))
}

func (l *stubLogger) messages() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.entries...)
}

func newTestGenerator(cfg GeneratorConfig) *Generator {
	return NewGenerator(cfg, &stubLogger{})
}

// Тесты
func TestNewGeneratorAppliesDefaults(t *testing.T) {
	logger := &stubLogger{}
	gen := NewGenerator(GeneratorConfig{}, logger)

	assert.Equal(t, time.Second, gen.cfg.Interval)
	assert.Equal(t, 1, gen.cfg.PacketSize)
	assert.NotNil(t, gen.cfg.RandSource)
	assert.NotNil(t, gen.rnd)
	assert.Equal(t, logger, gen.logger)
}

func TestNewGeneratorUsesProvidedConfig(t *testing.T) {
	source := rand.NewSource(1)
	cfg := GeneratorConfig{
		Interval:   5 * time.Millisecond,
		PacketSize: 3,
		RandSource: source,
	}
	logger := &stubLogger{}

	gen := NewGenerator(cfg, logger)

	assert.Equal(t, cfg.Interval, gen.cfg.Interval)
	assert.Equal(t, cfg.PacketSize, gen.cfg.PacketSize)
	assert.Equal(t, source, gen.cfg.RandSource)
	assert.Equal(t, logger, gen.logger)
}

func TestGeneratorRunProducesPackets(t *testing.T) {
	source := rand.NewSource(123)
	cfg := GeneratorConfig{Interval: time.Millisecond, PacketSize: 2, RandSource: source}
	gen := newTestGenerator(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := make(chan domain.DataPacket, 1)
	done := make(chan struct{})

	go func() {
		gen.Run(ctx, out)
		close(done)
	}()

	var packet domain.DataPacket
	select {
	case packet = <-out:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("не получили пакет за отведённое время")
	}

	cancel()
	<-done

	assert.NotEmpty(t, packet.ID)
	assert.Len(t, packet.Measurements, 2)
	for _, m := range packet.Measurements {
		assert.Equal(t, packet.ID, m.PacketID)
		assert.NotEmpty(t, m.SourceID)
		assert.False(t, m.Timestamp.IsZero())
	}
}

func TestGeneratorLog(t *testing.T) {
	logger := &stubLogger{}
	gen := NewGenerator(GeneratorConfig{}, logger)

	gen.log(context.Background(), "hello %s", "world")

	assert.Len(t, logger.messages(), 1)
	assert.Contains(t, logger.messages()[0], "hello world")
}

func TestGeneratorLogWithNilLogger(t *testing.T) {
	gen := &Generator{}
	assert.NotPanics(t, func() {
		gen.log(context.Background(), "ignored")
	})
}
