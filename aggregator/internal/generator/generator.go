package generator

import (
	"context"
	"math/rand"
	"time"

	"github.com/google/uuid"

	"aggregator/internal/domain"
)

type Source interface {
	Start(ctx context.Context) <-chan domain.DataPacket
}

type Config struct {
	PayloadLen int
	Interval   time.Duration
	BufferSize int
}

type RandomSource struct {
	cfg Config
	rnd *rand.Rand
}

func NewRandomSource(cfg Config) Source {
	normalized := normalizeConfig(cfg)

	return &RandomSource{
		cfg: normalized,
		rnd: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *RandomSource) Start(ctx context.Context) <-chan domain.DataPacket {
	out := make(chan domain.DataPacket, s.cfg.BufferSize)

	go func() {
		ticker := time.NewTicker(s.cfg.Interval)
		defer func() {
			ticker.Stop()
			close(out)
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				packet := domain.DataPacket{
					ID:        uuid.New(),
					Timestamp: time.Now().UTC(),
					Payload:   s.generatePayload(),
				}

				select {
				case out <- packet:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return out
}

func (s *RandomSource) generatePayload() []int64 {
	payload := make([]int64, s.cfg.PayloadLen)
	for i := range payload {
		payload[i] = s.rnd.Int63()
	}
	return payload
}

func normalizeConfig(cfg Config) Config {
	if cfg.PayloadLen <= 0 {
		cfg.PayloadLen = 1
	}
	if cfg.Interval <= 0 {
		cfg.Interval = time.Millisecond
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 1024
	}
	return cfg
}
