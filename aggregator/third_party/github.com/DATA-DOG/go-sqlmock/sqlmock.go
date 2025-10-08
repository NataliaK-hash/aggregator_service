package sqlmock

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"sync"
	"sync/atomic"
)

type Sqlmock interface {
	ExpectPing() Sqlmock
	ExpectExec(pattern string) *ExpectedExec
	ExpectQuery(pattern string) *ExpectedQuery
	ExpectClose() Sqlmock
	ExpectationsWereMet() error
}

type AnyArgMarker struct{}

func AnyArg() AnyArgMarker { return AnyArgMarker{} }

var driverCounter atomic.Int64

func New() (*sql.DB, Sqlmock, error) {
	name := fmt.Sprintf("sqlmock_driver_%d", driverCounter.Add(1))
	m := &mock{}
	sql.Register(name, &mockDriver{mock: m})
	db, err := sql.Open(name, "sqlmock")
	if err != nil {
		return nil, nil, err
	}
	m.db = db
	return db, m, nil
}

type mockDriver struct {
	mock *mock
}

func (d *mockDriver) Open(name string) (driver.Conn, error) {
	return &mockConn{mock: d.mock}, nil
}

type mock struct {
	mu           sync.Mutex
	expectations []expectation
	db           *sql.DB
}

func (m *mock) ExpectPing() Sqlmock {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expectations = append(m.expectations, &pingExpectation{})
	return m
}

func (m *mock) ExpectExec(pattern string) *ExpectedExec {
	m.mu.Lock()
	defer m.mu.Unlock()
	exec := &ExpectedExec{pattern: regexp.MustCompile(pattern)}
	m.expectations = append(m.expectations, exec)
	return exec
}

func (m *mock) ExpectQuery(pattern string) *ExpectedQuery {
	m.mu.Lock()
	defer m.mu.Unlock()
	query := &ExpectedQuery{pattern: regexp.MustCompile(pattern)}
	m.expectations = append(m.expectations, query)
	return query
}

func (m *mock) ExpectClose() Sqlmock {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expectations = append(m.expectations, &closeExpectation{})
	return m
}

func (m *mock) ExpectationsWereMet() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, exp := range m.expectations {
		if !exp.done() {
			return fmt.Errorf("there are remaining expectations: %s", exp.describe())
		}
	}
	return nil
}

type expectation interface {
	mark()
	done() bool
	describe() string
}

type pingExpectation struct {
	satisfied bool
}

func (p *pingExpectation) mark()            { p.satisfied = true }
func (p *pingExpectation) done() bool       { return p.satisfied }
func (p *pingExpectation) describe() string { return "ping" }

type closeExpectation struct {
	satisfied bool
}

func (c *closeExpectation) mark()            { c.satisfied = true }
func (c *closeExpectation) done() bool       { return c.satisfied }
func (c *closeExpectation) describe() string { return "close" }

type ExpectedExec struct {
	pattern   *regexp.Regexp
	args      []interface{}
	result    driver.Result
	satisfied bool
}

func (e *ExpectedExec) WithArgs(args ...interface{}) *ExpectedExec {
	e.args = append([]interface{}(nil), args...)
	return e
}

func (e *ExpectedExec) WillReturnResult(result driver.Result) *ExpectedExec {
	e.result = result
	return e
}

func (e *ExpectedExec) mark()            { e.satisfied = true }
func (e *ExpectedExec) done() bool       { return e.satisfied }
func (e *ExpectedExec) describe() string { return "exec " + e.pattern.String() }

type ExpectedQuery struct {
	pattern   *regexp.Regexp
	args      []interface{}
	rows      *Rows
	satisfied bool
}

func (q *ExpectedQuery) WithArgs(args ...interface{}) *ExpectedQuery {
	q.args = append([]interface{}(nil), args...)
	return q
}

func (q *ExpectedQuery) WillReturnRows(rows *Rows) *ExpectedQuery {
	q.rows = rows
	return q
}

func (q *ExpectedQuery) mark()            { q.satisfied = true }
func (q *ExpectedQuery) done() bool       { return q.satisfied }
func (q *ExpectedQuery) describe() string { return "query " + q.pattern.String() }

type Rows struct {
	columns []string
	data    [][]driver.Value
}

func NewRows(columns []string) *Rows {
	return &Rows{columns: append([]string(nil), columns...)}
}

func (r *Rows) AddRow(values ...interface{}) *Rows {
	row := make([]driver.Value, len(values))
	for i, v := range values {
		row[i] = v
	}
	r.data = append(r.data, row)
	return r
}

func NewResult(lastInsertID, rowsAffected int64) driver.Result {
	return &result{lastID: lastInsertID, rows: rowsAffected}
}

type result struct {
	lastID int64
	rows   int64
}

func (r *result) LastInsertId() (int64, error) { return r.lastID, nil }
func (r *result) RowsAffected() (int64, error) { return r.rows, nil }

type mockConn struct {
	mock *mock
}

var (
	_ driver.ExecerContext     = (*mockConn)(nil)
	_ driver.QueryerContext    = (*mockConn)(nil)
	_ driver.NamedValueChecker = (*mockConn)(nil)
	_ driver.Pinger            = (*mockConn)(nil)
)

func (c *mockConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("sqlmock: prepared statements not supported")
}
func (c *mockConn) Close() error {
	return c.mock.consumeExpectation("close", func(exp expectation) bool {
		_, ok := exp.(*closeExpectation)
		return ok
	})
}
func (c *mockConn) Begin() (driver.Tx, error) {
	return nil, errors.New("sqlmock: transactions not supported")
}
func (c *mockConn) CheckNamedValue(*driver.NamedValue) error { return nil }
func (c *mockConn) Ping(ctx context.Context) error {
	return c.mock.consumeExpectationWithContext(ctx, "ping", func(exp expectation) bool {
		_, ok := exp.(*pingExpectation)
		return ok
	})
}

func (c *mockConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return c.mock.exec(ctx, query, args)
}

func (c *mockConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return c.mock.query(ctx, query, args)
}

func (m *mock) consumeExpectation(kind string, predicate func(expectation) bool) error {
	return m.consumeExpectationWithContext(context.Background(), kind, predicate)
}

func (m *mock) consumeExpectationWithContext(ctx context.Context, kind string, predicate func(expectation) bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.expectations) == 0 {
		return fmt.Errorf("sqlmock: no expectation for %s", kind)
	}
	exp := m.expectations[0]
	if !predicate(exp) {
		return fmt.Errorf("sqlmock: unexpected %s, expected %s", kind, exp.describe())
	}
	exp.mark()
	m.expectations = m.expectations[1:]
	return nil
}

func (m *mock) exec(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.expectations) == 0 {
		return nil, fmt.Errorf("sqlmock: unexpected exec %s", query)
	}
	exp := m.expectations[0]
	execExp, ok := exp.(*ExpectedExec)
	if !ok {
		return nil, fmt.Errorf("sqlmock: expected %s, got exec", exp.describe())
	}
	if !execExp.pattern.MatchString(query) {
		return nil, fmt.Errorf("sqlmock: query %q does not match %q", query, execExp.pattern.String())
	}
	if !matchArgs(execExp.args, args) {
		return nil, fmt.Errorf("sqlmock: arguments do not match for exec %s", query)
	}
	execExp.mark()
	m.expectations = m.expectations[1:]
	if execExp.result == nil {
		execExp.result = &result{}
	}
	return execExp.result, nil
}

func (m *mock) query(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.expectations) == 0 {
		return nil, fmt.Errorf("sqlmock: unexpected query %s", query)
	}
	exp := m.expectations[0]
	queryExp, ok := exp.(*ExpectedQuery)
	if !ok {
		return nil, fmt.Errorf("sqlmock: expected %s, got query", exp.describe())
	}
	if !queryExp.pattern.MatchString(query) {
		return nil, fmt.Errorf("sqlmock: query %q does not match %q", query, queryExp.pattern.String())
	}
	if !matchArgs(queryExp.args, args) {
		return nil, fmt.Errorf("sqlmock: arguments do not match for query %s", query)
	}
	if queryExp.rows == nil {
		return nil, errors.New("sqlmock: rows not provided")
	}
	queryExp.mark()
	m.expectations = m.expectations[1:]
	return &rowSet{columns: queryExp.rows.columns, data: queryExp.rows.data}, nil
}

func matchArgs(expected []interface{}, actual []driver.NamedValue) bool {
	if len(expected) == 0 {
		return true
	}
	if len(expected) != len(actual) {
		return false
	}
	for i := range expected {
		if _, ok := expected[i].(AnyArgMarker); ok {
			continue
		}
		if !reflect.DeepEqual(expected[i], actual[i].Value) {
			return false
		}
	}
	return true
}

type rowSet struct {
	columns []string
	data    [][]driver.Value
	index   int
}

var _ driver.Rows = (*rowSet)(nil)

func (r *rowSet) Columns() []string { return append([]string(nil), r.columns...) }

func (r *rowSet) Close() error { r.data = nil; return nil }

func (r *rowSet) Next(dest []driver.Value) error {
	if r.index >= len(r.data) {
		return io.EOF
	}
	row := r.data[r.index]
	for i := range dest {
		dest[i] = row[i]
	}
	r.index++
	return nil
}
