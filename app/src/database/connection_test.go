package database

import (
	"context"
	"net"
	"net/url"
	"testing"
	"time"

	"aggregator-service/app/src/infra"

	"github.com/stretchr/testify/assert"
)

// вспомогательные функции

func newCtxWithTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

func mustParseDSN(t *testing.T, dsn string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("failed to parse DSN: %v", err)
	}
	return parsed
}

// Connect

func TestConnectValidatesConfig(t *testing.T) {
	t.Log("Проверяем ошибку при отсутствии конфига")
	_, err := Connect(nil)
	assert.Error(t, err)

	t.Log("Проверяем ошибку при пустом конфиге")
	_, err = Connect(&Config{})
	assert.Error(t, err)
}

func TestConnectPingFailure(t *testing.T) {
	t.Log("Создаём конфиг с недоступной базой")
	cfg := &Config{DSN: "postgres://invalid:invalid@127.0.0.1:1/db?sslmode=disable"}

	t.Log("Ожидаем ошибку подключения")
	_, err := Connect(cfg)
	assert.Error(t, err)
}

// ShouldCheckDatabase

func TestShouldCheckDatabase(t *testing.T) {
	cfg := infra.Config{}
	assert.False(t, ShouldCheckDatabase(cfg))

	cfg.DatabaseDSN = "dsn"
	assert.True(t, ShouldCheckDatabase(cfg))

	cfg.DatabaseDSN = ""
	cfg.DatabaseHost = "host"
	assert.True(t, ShouldCheckDatabase(cfg))
}

// WaitForDatabase

func TestWaitForDatabaseSkipsWhenNoHost(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := WaitForDatabase(ctx, infra.Config{}, nil)
	assert.NoError(t, err)
}

func TestWaitForDatabaseHonoursContext(t *testing.T) {
	cfg := infra.Config{DatabaseHost: "127.0.0.1", DatabasePort: "1"}
	ctx, cancel := newCtxWithTimeout(time.Millisecond)
	defer cancel()

	err := WaitForDatabase(ctx, cfg, nil)
	assert.Error(t, err)
}

func TestWaitForDatabaseParsesDSN(t *testing.T) {
	cfg := infra.Config{DatabaseDSN: "postgres://user:pass@localhost:1234/db"}
	ctx, cancel := newCtxWithTimeout(10 * time.Millisecond)
	defer cancel()

	err := WaitForDatabase(ctx, cfg, nil)
	assert.Error(t, err)
}

// BuildDatabaseDSN

func TestBuildDatabaseDSN(t *testing.T) {
	cfg := infra.Config{DatabaseDSN: "dsn"}
	dsn, err := BuildDatabaseDSN(cfg)
	assert.NoError(t, err)
	assert.Equal(t, "dsn", dsn)

	cfg = infra.Config{DatabaseHost: "host", DatabaseUser: "user", DatabaseName: "db"}
	dsn, err = BuildDatabaseDSN(cfg)
	assert.NoError(t, err)
	assert.Contains(t, dsn, "host")
	assert.Contains(t, dsn, "sslmode=disable")

	_, err = BuildDatabaseDSN(infra.Config{})
	assert.Error(t, err)
}

func TestBuildDatabaseDSNDefaultsPort(t *testing.T) {
	cfg := infra.Config{DatabaseHost: "host", DatabaseUser: "user", DatabaseName: "db"}
	dsn, err := BuildDatabaseDSN(cfg)
	assert.NoError(t, err)

	_, port, _ := net.SplitHostPort(mustParseDSN(t, dsn).Host)
	assert.Equal(t, "5432", port)
}

// SetupRepository

func TestSetupRepositoryPropagatesErrors(t *testing.T) {
	_, _, err := SetupRepository(context.Background(), infra.Config{}, nil)
	assert.Error(t, err)
}
