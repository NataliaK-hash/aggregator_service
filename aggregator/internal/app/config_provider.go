package app

import (
	"aggregator/internal/config"
	"aggregator/internal/generator"
)

// provideGeneratorConfig формирует конфигурацию генератора для системы внедрения зависимостей.
func provideGeneratorConfig(cfg *config.Config) generator.Config {
	return generator.Config{
		PayloadLen: cfg.Generator.PayloadLen,
		Interval:   cfg.Generator.Interval,
		BufferSize: 1024,
	}
}
