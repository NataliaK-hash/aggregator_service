package core

import (
	"context"
	"math/rand"
	"time"

	"aggregator-service/app/src/domain"
	"aggregator-service/app/src/infra"
	"aggregator-service/app/src/shared/constants"
)

// GeneratorConfig describes the runtime characteristics of the generator.
type GeneratorConfig struct {
	Interval   time.Duration
	PacketSize int
	RandSource rand.Source
}

// Generator produces measurement packets at a fixed interval.
type Generator struct {
	cfg    GeneratorConfig
	logger Logger
	rnd    *rand.Rand
}

// New creates a configured generator instance.
func NewGenerator(cfg GeneratorConfig, logger Logger) *Generator {
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
			g.log(ctx, "generator: context cancelled: %v", ctx.Err())
			return
		case <-ticker.C:
		}

		packetID := constants.GenerateUUID()
		now := time.Now().UTC()
		measurements := make([]domain.Measurement, g.cfg.PacketSize)
		for i := range measurements {
			value := float64(g.rnd.Int63n(1000))
			measurements[i] = domain.Measurement{
				PacketID:  packetID,
				SourceID:  constants.GenerateUUID(),
				Value:     value,
				Timestamp: now,
			}
		}

		g.log(ctx, "generator: produced packet id=%s", packetID)
		infra.IncGeneratorPackets()

		packet := domain.DataPacket{ID: packetID, Measurements: measurements}

		select {
		case <-ctx.Done():
			g.log(ctx, "generator: stopping before delivering packet: %v", ctx.Err())
			return
		case out <- packet:
		}
	}
}

func (g *Generator) log(ctx context.Context, format string, v ...any) {
	if g.logger == nil {
		return
	}
	g.logger.Printf(ctx, format, v...)
}

var _ domain.PacketGenerator = (*Generator)(nil)
