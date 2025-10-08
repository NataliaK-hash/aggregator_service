package postgres

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/testcontainers/testcontainers-go"
)

// Option представляет опцию настройки контейнера PostgreSQL.
type Option func(*config)

type config struct {
	database string
	username string
	password string
}

// WithDatabase задаёт имя базы данных для заглушки.
func WithDatabase(name string) testcontainers.CustomizeRequestOption {
	return func(r *testcontainers.Request) {
		if r.Env == nil {
			r.Env = make(map[string]string)
		}
		r.Env["POSTGRES_DB"] = name
	}
}

// WithUsername задаёт имя пользователя.
func WithUsername(username string) testcontainers.CustomizeRequestOption {
	return func(r *testcontainers.Request) {
		if r.Env == nil {
			r.Env = make(map[string]string)
		}
		r.Env["POSTGRES_USER"] = username
	}
}

// WithPassword задаёт пароль пользователя.
func WithPassword(password string) testcontainers.CustomizeRequestOption {
	return func(r *testcontainers.Request) {
		if r.Env == nil {
			r.Env = make(map[string]string)
		}
		r.Env["POSTGRES_PASSWORD"] = password
	}
}

// PostgresContainer представляет запущенный заглушечный контейнер.
type PostgresContainer struct {
	dsn string
}

// RunContainer создаёт заглушечный контейнер PostgreSQL.
func RunContainer(ctx context.Context, opts ...testcontainers.CustomizeRequestOption) (*PostgresContainer, error) {
	_ = ctx
	req := &testcontainers.Request{Env: make(map[string]string)}
	for _, opt := range opts {
		opt(req)
	}

	name := req.Env["POSTGRES_DB"]
	if name == "" {
		name = fmt.Sprintf("stubdb_%d", rand.New(rand.NewSource(time.Now().UnixNano())).Intn(1_000_000))
	}

	dsn := fmt.Sprintf("stub://%s", sanitize(name))
	return &PostgresContainer{dsn: dsn}, nil
}

// ConnectionString возвращает DSN для подключения.
func (c *PostgresContainer) ConnectionString(ctx context.Context, params ...string) (string, error) {
	_ = ctx
	if len(params) == 0 {
		return c.dsn, nil
	}
	return c.dsn + "?" + strings.Join(params, "&"), nil
}

// Terminate завершает работу контейнера.
func (c *PostgresContainer) Terminate(ctx context.Context) error {
	_ = ctx
	return nil
}

func sanitize(name string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_")
	return replacer.Replace(name)
}
