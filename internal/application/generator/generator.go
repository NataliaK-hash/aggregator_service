package generator

import (
	"context"
	"math/rand"
	"time"

	"aggregator-service-project/internal/domain"
	"aggregator-service-project/internal/pkg/uuid"
)

// Logger defines the logging behaviour required by the generator.
type Logger interface {
	Printf(format string, v ...any)
}

// Config describes the runtime characteristics of the generator.
type Config struct {
	Interval   time.Duration
	PacketSize int
	RandSource rand.Source
}

// Generator produces measurement packets at a fixed interval.
type Generator struct {
	cfg    Config
	logger Logger
	rnd    *rand.Rand
}

// New creates a configured generator instance.
func New(cfg Config, logger Logger) *Generator {
	if cfg.Interval <= 0 {
		cfg.Interval = time.Second
	}
	if cfg.PacketSize <= 0 {
		cfg.PacketSize = 1
	}

	source := cfg.RandSource
	if source == nil {
		source = rand.NewSource(time.Now().UnixNano())
	}

	return &Generator{
		cfg:    cfg,
		logger: logger,
		rnd:    rand.New(source),
	}
}

// Run starts generating packets until the provided context is cancelled. The
// output channel is closed once generation stops.
func (g *Generator) Run(ctx context.Context, out chan<- domain.DataPacket) {
	defer close(out)

	ticker := time.NewTicker(g.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			g.log("generator: context cancelled: %v", ctx.Err())
			return
		case <-ticker.C:
		}

		packetID := uuid.NewString()
		now := time.Now().UTC()
		measurements := make([]domain.Measurement, g.cfg.PacketSize)
		for i := range measurements {
			measurements[i] = domain.Measurement{
				PacketID:  packetID,
				SourceID:  uuid.NewString(),
				Value:     g.rnd.Float64() * 100,
				Timestamp: now,
			}
		}

		g.log("generator: produced packet id=%s", packetID)

		packet := domain.DataPacket{ID: packetID, Measurements: measurements}

		select {
		case <-ctx.Done():
			g.log("generator: stopping before delivering packet: %v", ctx.Err())
			return
		case out <- packet:
		}
	}
}

func (g *Generator) log(format string, v ...any) {
	if g.logger == nil {
		return
	}
	g.logger.Printf(format, v...)
}

var _ domain.PacketGenerator = (*Generator)(nil)
