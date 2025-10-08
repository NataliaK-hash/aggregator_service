package stdlib

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"
)

func init() {
	sql.Register("pgx", &drv{})
}

type drv struct{}

func (d *drv) Open(name string) (driver.Conn, error) {
	db := obtainDatabase(name)
	return &conn{db: db}, nil
}

type conn struct {
	db *database
}

var (
	_ driver.ExecerContext     = (*conn)(nil)
	_ driver.QueryerContext    = (*conn)(nil)
	_ driver.NamedValueChecker = (*conn)(nil)
	_ driver.Pinger            = (*conn)(nil)
)

func (c *conn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("pgx stub does not support prepared statements")
}

func (c *conn) Close() error {
	c.db.release()
	return nil
}

func (c *conn) Begin() (driver.Tx, error) {
	return nil, errors.New("pgx stub does not support transactions")
}

func (c *conn) CheckNamedValue(*driver.NamedValue) error { return nil }

func (c *conn) Ping(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (c *conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	upper := strings.ToUpper(strings.TrimSpace(query))
	if strings.HasPrefix(upper, "CREATE TABLE") || strings.HasPrefix(upper, "CREATE INDEX") || (strings.Contains(upper, "CREATE TABLE") && strings.Contains(upper, "CREATE INDEX")) {
		return driver.RowsAffected(0), nil
	}
	if strings.HasPrefix(upper, "INSERT INTO PACKET_MAX") {
		values := namedValues(args)
		if len(values)%3 != 0 {
			return nil, fmt.Errorf("invalid argument count: %d", len(values))
		}

		c.db.mu.Lock()
		defer c.db.mu.Unlock()

		affected := 0
		for i := 0; i < len(values); i += 3 {
			id, ok := values[i].(string)
			if !ok {
				return nil, fmt.Errorf("expected string id, got %T", values[i])
			}
			ts, ok := values[i+1].(time.Time)
			if !ok {
				return nil, fmt.Errorf("expected time.Time timestamp, got %T", values[i+1])
			}
			max, err := toInt(values[i+2])
			if err != nil {
				return nil, err
			}
			c.db.records[id] = record{ID: id, Timestamp: ts, MaxValue: max}
			affected++
		}

		return driver.RowsAffected(affected), nil
	}

	return nil, fmt.Errorf("unsupported exec query: %s", query)
}

func (c *conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	upper := strings.ToUpper(strings.TrimSpace(query))

	switch {
	case strings.Contains(upper, "WHERE ID = $1"):
		if len(args) != 1 {
			return nil, fmt.Errorf("expected 1 arg, got %d", len(args))
		}
		id, ok := args[0].Value.(string)
		if !ok {
			return nil, fmt.Errorf("expected string id, got %T", args[0].Value)
		}

		c.db.mu.RLock()
		defer c.db.mu.RUnlock()

		rec, ok := c.db.records[id]
		if !ok {
			return emptyRows(), nil
		}

		return &rows{
			columns: []string{"id", "timestamp", "max_value"},
			data:    [][]driver.Value{{rec.ID, rec.Timestamp, rec.MaxValue}},
		}, nil
	case strings.Contains(upper, "TIMESTAMP >= $1") && strings.Contains(upper, "TIMESTAMP < $2"):
		if len(args) != 2 {
			return nil, fmt.Errorf("expected 2 args, got %d", len(args))
		}
		from, ok := args[0].Value.(time.Time)
		if !ok {
			return nil, fmt.Errorf("expected time.Time for from, got %T", args[0].Value)
		}
		to, ok := args[1].Value.(time.Time)
		if !ok {
			return nil, fmt.Errorf("expected time.Time for to, got %T", args[1].Value)
		}

		c.db.mu.RLock()
		defer c.db.mu.RUnlock()

		data := make([]record, 0)
		for _, rec := range c.db.records {
			if !rec.Timestamp.Before(from) && rec.Timestamp.Before(to) {
				data = append(data, rec)
			}
		}

		sort.Slice(data, func(i, j int) bool {
			return data[i].Timestamp.Before(data[j].Timestamp)
		})

		values := make([][]driver.Value, len(data))
		for i, rec := range data {
			values[i] = []driver.Value{rec.ID, rec.Timestamp, rec.MaxValue}
		}

		return &rows{
			columns: []string{"id", "timestamp", "max_value"},
			data:    values,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported query: %s", query)
	}
}

func namedValues(values []driver.NamedValue) []any {
	out := make([]any, len(values))
	for i := range values {
		out[i] = values[i].Value
	}
	return out
}

func toInt(value any) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case uint:
		return int(v), nil
	case uint32:
		return int(v), nil
	case uint64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("unsupported numeric type %T", value)
	}
}

type rows struct {
	columns []string
	data    [][]driver.Value
	idx     int
}

var _ driver.Rows = (*rows)(nil)

func (r *rows) Columns() []string { return append([]string(nil), r.columns...) }

func (r *rows) Close() error { r.data = nil; return nil }

func (r *rows) Next(dest []driver.Value) error {
	if r.idx >= len(r.data) {
		return io.EOF
	}
	copy(dest, r.data[r.idx])
	r.idx++
	return nil
}

type record struct {
	ID        string
	Timestamp time.Time
	MaxValue  int
}

type database struct {
	mu      sync.RWMutex
	records map[string]record
	refCnt  int
}

func (db *database) release() {
	databasesMu.Lock()
	defer databasesMu.Unlock()
	db.refCnt--
	if db.refCnt <= 0 {
		for key, candidate := range databases {
			if candidate == db {
				delete(databases, key)
				break
			}
		}
	}
}

var (
	databases   = map[string]*database{}
	databasesMu sync.Mutex
)

func obtainDatabase(name string) *database {
	databasesMu.Lock()
	defer databasesMu.Unlock()

	db, ok := databases[name]
	if !ok {
		db = &database{records: make(map[string]record)}
		databases[name] = db
	}
	db.refCnt++
	return db
}

func emptyRows() driver.Rows {
	return &rows{columns: []string{"id", "timestamp", "max_value"}, data: nil}
}
