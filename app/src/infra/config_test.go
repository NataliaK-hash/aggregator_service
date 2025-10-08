package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfigDefaults(t *testing.T) {
	t.Log("Шаг 1: очищаем переменные окружения и загружаем конфиг")
	t.Setenv("HTTP_PORT", "")
	t.Setenv("DB_BATCH_SIZE", "")

	cfg := LoadConfig()

	assert.Equal(t, "8080", cfg.HTTPPort)
	assert.Equal(t, 32, cfg.DatabaseBatchSize)
	assert.Equal(t, 250, cfg.DatabaseBatchTimeoutMS)
}

func TestLoadConfigReadsEnvironment(t *testing.T) {
	t.Log("Шаг 1: устанавливаем переменные окружения для конфигурации")
	t.Setenv("HTTP_PORT", "9000")
	t.Setenv("DB_DSN", "dsn")
	t.Setenv("DB_BATCH_SIZE", "64")
	t.Setenv("M", "8")

	cfg := LoadConfig()

	assert.Equal(t, "9000", cfg.HTTPPort)
	assert.Equal(t, "dsn", cfg.DatabaseDSN)
	assert.Equal(t, 64, cfg.DatabaseBatchSize)
	assert.Equal(t, 8, cfg.WorkerCount)
}

func TestLogConfigProducesEntries(t *testing.T) {
	t.Log("Шаг 1: логируем конфигурацию и проверяем записи")
	var buf bytes.Buffer
	logger := NewLogger(&buf, "test")
	cfg := Config{}

	LogConfig(context.Background(), logger, cfg)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.NotEmpty(t, lines)

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var payload map[string]any
		assert.NoError(t, json.Unmarshal([]byte(line), &payload))
		assert.Equal(t, "info", payload["level"])
	}
}

func TestGetEnv(t *testing.T) {
	t.Log("получаем переменную окружения с запасным значением")
	t.Setenv("FOO", "bar")
	assert.Equal(t, "bar", getEnv("FOO", "baz"))
	t.Setenv("FOO", "")
	assert.Equal(t, "baz", getEnv("FOO", "baz"))
}

func TestGetEnvInt(t *testing.T) {
	t.Log("читаем целочисленную переменную окружения")
	t.Setenv("NUM", "42")
	assert.Equal(t, 42, getEnvInt("NUM", 1))
	t.Log("проверяем поведение при некорректном значении")
	t.Setenv("NUM", "invalid")
	assert.Equal(t, 1, getEnvInt("NUM", 1))
}
