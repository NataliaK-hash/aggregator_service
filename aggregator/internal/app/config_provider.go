package app

import (
	"aggregator/internal/config"
	"aggregator/internal/generator"
)

func provideGeneratorConfig(cfg *config.Config) generator.Config {
	return generator.Config{
		PayloadLen: cfg.Generator.PayloadLen,
		Interval:   cfg.Generator.Interval,
		BufferSize: 1024,
	}
}
