package database

import (
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

type SQLRunner struct {
	mu  sync.Mutex
	dbs map[string]*sql.DB
}

func newSQLRunner() CommandRunner {
	return &SQLRunner{dbs: make(map[string]*sql.DB)}
}

// NewSQLRunner returns a CommandRunner implementation backed by database/sql.
func NewSQLRunner() CommandRunner {
	return newSQLRunner()
}

func (r *SQLRunner) Exec(ctx context.Context, dsn, _ string, statement string, args ...any) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	trimmed := strings.TrimSpace(statement)
	if trimmed == "" {
		return "", nil
	}

	db, err := r.dbFor(ctx, dsn)
	if err != nil {
		return "", err
	}

	if isSelectStatement(trimmed) {
		return queryToCSV(ctx, db, trimmed, args...)
	}

	result, err := db.ExecContext(ctx, trimmed, args...)
	if err != nil {
		return "", err
	}

	return commandTag(trimmed, result)
}

func (r *SQLRunner) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for dsn, db := range r.dbs {
		db.Close()
		delete(r.dbs, dsn)
	}

	return nil
}

func (r *SQLRunner) dbFor(ctx context.Context, dsn string) (*sql.DB, error) {
	r.mu.Lock()
	if db, ok := r.dbs[dsn]; ok {
		r.mu.Unlock()
		return db, nil
	}
	r.mu.Unlock()

	createCtx, cancel := contextForPool(ctx)
	if cancel != nil {
		defer cancel()
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("sql runner: open: %w", err)
	}

	db.SetMaxOpenConns(15)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.PingContext(createCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("sql runner: ping: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.dbs[dsn]; ok {
		db.Close()
		return existing, nil
	}
	r.dbs[dsn] = db
	return db, nil
}

func contextForPool(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, nil
	}
	return context.WithTimeout(context.Background(), 5*time.Second)
}

func isSelectStatement(statement string) bool {
	upper := strings.ToUpper(statement)
	return strings.HasPrefix(upper, "SELECT") || strings.HasPrefix(upper, "WITH")
}

func queryToCSV(ctx context.Context, db *sql.DB, statement string, args ...any) (string, error) {
	rows, err := db.QueryContext(ctx, statement, args...)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	writer := csv.NewWriter(&builder)
	wrote := false

	for rows.Next() {
		values := make([]any, len(columns))
		scanTargets := make([]any, len(columns))
		for i := range values {
			scanTargets[i] = &values[i]
		}

		if err := rows.Scan(scanTargets...); err != nil {
			return "", err
		}

		record := make([]string, len(values))
		for i, value := range values {
			record[i] = formatValue(value)
		}

		if err := writer.Write(record); err != nil {
			return "", err
		}
		wrote = true
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}

	if err := rows.Err(); err != nil {
		return "", err
	}

	if !wrote {
		return "", nil
	}

	return builder.String(), nil
}

func formatValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case []byte:
		return string(v)
	case time.Time:
		return v.UTC().Format(time.RFC3339Nano)
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

func commandTag(statement string, result sql.Result) (string, error) {
	affected, err := result.RowsAffected()
	if err != nil {
		return "", err
	}

	verb := ""
	fields := strings.Fields(strings.ToUpper(statement))
	if len(fields) > 0 {
		verb = fields[0]
	}

	switch verb {
	case "INSERT":
		return fmt.Sprintf("INSERT 0 %d", affected), nil
	case "UPDATE", "DELETE":
		return fmt.Sprintf("%s %d", verb, affected), nil
	default:
		if verb == "" {
			verb = "EXEC"
		}
		return fmt.Sprintf("%s %d", verb, affected), nil
	}
}

var _ CommandRunner = (*SQLRunner)(nil)
