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

func NewSQLRunner() CommandRunner {
	return newSQLRunner()
}

func (r *SQLRunner) Exec(ctx context.Context, dsn, _ string, statement string, args ...any) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	query := strings.TrimSpace(statement)
	if query == "" {
		return "", nil
	}

	db, err := r.dbFor(ctx, dsn)
	if err != nil {
		return "", err
	}

	if isSelectStatement(query) {
		return runSelectAsCSV(ctx, db, query, args...)
	}
	return runCommand(ctx, db, query, args...)
}

func (r *SQLRunner) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for dsn, db := range r.dbs {
		_ = db.Close()
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

	db, err := openDB(ctx, dsn)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.dbs[dsn]; ok {
		_ = db.Close()
		return existing, nil
	}

	r.dbs[dsn] = db
	return db, nil
}

func openDB(ctx context.Context, dsn string) (*sql.DB, error) {
	poolCtx, cancel := contextForPool(ctx)
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

	if err := db.PingContext(poolCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("sql runner: ping: %w", err)
	}

	return db, nil
}

func contextForPool(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, nil
	}
	return context.WithTimeout(context.Background(), 5*time.Second)
}

func isSelectStatement(statement string) bool {
	s := strings.ToUpper(statement)
	return strings.HasPrefix(s, "SELECT") || strings.HasPrefix(s, "WITH")
}

func runSelectAsCSV(ctx context.Context, db *sql.DB, query string, args ...any) (string, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	return writeCSV(rows)
}

func runCommand(ctx context.Context, db *sql.DB, query string, args ...any) (string, error) {
	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return "", err
	}
	return commandTag(query, result)
}

func writeCSV(rows *sql.Rows) (string, error) {
	columns, err := rows.Columns()
	if err != nil {
		return "", err
	}

	var b strings.Builder
	writer := csv.NewWriter(&b)
	wrote := false

	for rows.Next() {
		record, err := scanRow(rows, len(columns))
		if err != nil {
			return "", err
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

	return b.String(), nil
}

func scanRow(rows *sql.Rows, n int) ([]string, error) {
	values := make([]any, n)
	ptrs := make([]any, n)
	for i := range values {
		ptrs[i] = &values[i]
	}

	if err := rows.Scan(ptrs...); err != nil {
		return nil, err
	}

	record := make([]string, n)
	for i, v := range values {
		record[i] = formatValue(v)
	}
	return record, nil
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

	verb := strings.ToUpper(strings.Fields(statement)[0])
	switch verb {
	case "INSERT":
		return fmt.Sprintf("INSERT 0 %d", affected), nil
	case "UPDATE", "DELETE":
		return fmt.Sprintf("%s %d", verb, affected), nil
	default:
		return fmt.Sprintf("%s %d", verb, affected), nil
	}
}

var _ CommandRunner = (*SQLRunner)(nil)
