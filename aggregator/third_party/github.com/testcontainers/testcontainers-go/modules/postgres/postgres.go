package postgres

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/testcontainers/testcontainers-go"
)

type Option func(*config)

type config struct {
	database string
	username string
	password string
}

func WithDatabase(name string) testcontainers.CustomizeRequestOption {
	return func(r *testcontainers.Request) {
		if r.Env == nil {
			r.Env = make(map[string]string)
		}
		r.Env["POSTGRES_DB"] = name
	}
}

func WithUsername(username string) testcontainers.CustomizeRequestOption {
	return func(r *testcontainers.Request) {
		if r.Env == nil {
			r.Env = make(map[string]string)
		}
		r.Env["POSTGRES_USER"] = username
	}
}

func WithPassword(password string) testcontainers.CustomizeRequestOption {
	return func(r *testcontainers.Request) {
		if r.Env == nil {
			r.Env = make(map[string]string)
		}
		r.Env["POSTGRES_PASSWORD"] = password
	}
}

type PostgresContainer struct {
	dsn string
}

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

func (c *PostgresContainer) ConnectionString(ctx context.Context, params ...string) (string, error) {
	_ = ctx
	if len(params) == 0 {
		return c.dsn, nil
	}
	return c.dsn + "?" + strings.Join(params, "&"), nil
}

func (c *PostgresContainer) Terminate(ctx context.Context) error {
	_ = ctx
	return nil
}

func sanitize(name string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_")
	return replacer.Replace(name)
}
