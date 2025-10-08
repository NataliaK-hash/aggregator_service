package core

import (
	"context"
	"math/rand"
	"time"

	"aggregator-service/app/src/domain"
	"aggregator-service/app/src/infra"
	"aggregator-service/app/src/shared/constants"
)

type GeneratorConfig struct {
	Interval   time.Duration
	PacketSize int
	RandSource rand.Source
}

type Generator struct {
	cfg    GeneratorConfig
	logger Logger
	rnd    *rand.Rand
}

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
	cfg.RandSource = source

	return &Generator{
		cfg:    cfg,
		logger: logger,
		rnd:    rand.New(source),
	}
}

func (g *Generator) Run(ctx context.Context, out chan<- domain.DataPacket) {
	defer close(out)

	ticker := time.NewTicker(g.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			g.log(ctx, "generator: остановлен (context cancelled): %v", ctx.Err())
			return
		case <-ticker.C:
		}

		packet := g.generatePacket()
		g.log(ctx, "generator: создан пакет id=%s", packet.ID)
		infra.IncGeneratorPackets()

		if !g.sendPacket(ctx, out, packet) {
			return
		}
	}
}

func (g *Generator) generatePacket() domain.DataPacket {
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

	return domain.DataPacket{ID: packetID, Measurements: measurements}
}

func (g *Generator) sendPacket(ctx context.Context, out chan<- domain.DataPacket, packet domain.DataPacket) bool {
	select {
	case <-ctx.Done():
		g.log(ctx, "generator: остановка перед отправкой пакета: %v", ctx.Err())
		return false
	case out <- packet:
		return true
	}
}

func (g *Generator) log(ctx context.Context, format string, v ...any) {
	if g.logger != nil {
		g.logger.Printf(ctx, format, v...)
	}
}

var _ domain.PacketGenerator = (*Generator)(nil)
