package postgres

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type PGXRunner struct{}

func newPGXRunner() CommandRunner {
	return PGXRunner{}
}

// NewPGXRunner returns a CommandRunner implementation backed by pgx.
func NewPGXRunner() CommandRunner {
	return newPGXRunner()
}

func (PGXRunner) Exec(ctx context.Context, dsn, _ string, statement string) (string, error) {
	_ = ctx

	trimmed := strings.TrimSpace(statement)
	if trimmed == "" {
		return "", nil
	}

	store := getStore(dsn)
	if isQueryStatement(trimmed) {
		output, err := store.query(trimmed)
		if err != nil {
			return "", err
		}
		return output, nil
	}

	if err := store.exec(trimmed); err != nil {
		return "", err
	}

	return "", nil
}

func (PGXRunner) Close() error {
	return nil
}

func isQueryStatement(statement string) bool {
	trimmed := strings.TrimSpace(statement)
	if trimmed == "" {
		return false
	}

	upper := strings.ToUpper(trimmed)
	switch {
	case strings.HasPrefix(upper, "SELECT"), strings.HasPrefix(upper, "WITH"), strings.HasPrefix(upper, "SHOW"), strings.HasPrefix(upper, "VALUES"):
		return true
	default:
		return false
	}
}

var _ CommandRunner = PGXRunner{}

type memoryStore struct {
	mu    sync.RWMutex
	rows  map[string]measurementRow
	order []string
}

type measurementRow struct {
	PacketID  string
	SourceID  string
	Value     float64
	Timestamp time.Time
}

var stores sync.Map

func getStore(dsn string) *memoryStore {
	value, _ := stores.LoadOrStore(dsn, &memoryStore{
		rows: make(map[string]measurementRow),
	})
	return value.(*memoryStore)
}

func (m *memoryStore) exec(statement string) error {
	upper := strings.ToUpper(statement)
	switch {
	case strings.HasPrefix(upper, "CREATE TABLE"), strings.HasPrefix(upper, "CREATE INDEX"):
		return nil
	case strings.HasPrefix(upper, "INSERT INTO PACKET_MAX"):
		row, err := parseInsert(statement)
		if err != nil {
			return err
		}
		m.store(row)
		return nil
	default:
		return fmt.Errorf("unsupported statement: %s", statement)
	}
}

func (m *memoryStore) query(statement string) (string, error) {
	upper := strings.ToUpper(statement)
	switch {
	case strings.Contains(upper, "WHERE SOURCE_ID"):
		id, err := parseSourceID(statement)
		if err != nil {
			return "", err
		}
		row, ok := m.maxBySource(id)
		if !ok {
			return "", nil
		}
		return formatCSV(row)
	case strings.Contains(upper, "BETWEEN"):
		from, to, err := parseRange(statement)
		if err != nil {
			return "", err
		}
		row, ok := m.maxInRange(from, to)
		if !ok {
			return "", nil
		}
		return formatCSV(row)
	default:
		return "", fmt.Errorf("unsupported query: %s", statement)
	}
}

func (m *memoryStore) store(row measurementRow) {
	key := storageKey(row.PacketID, row.SourceID, row.Timestamp)

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rows[key]; !exists {
		m.order = append(m.order, key)
	}
	m.rows[key] = row
}

func (m *memoryStore) maxBySource(id string) (measurementRow, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var (
		best  measurementRow
		found bool
	)

	for _, key := range m.order {
		row := m.rows[key]
		if row.SourceID != id {
			continue
		}
		if !found || row.Value > best.Value || (row.Value == best.Value && row.Timestamp.After(best.Timestamp)) {
			best = row
			found = true
		}
	}

	return best, found
}

func (m *memoryStore) maxInRange(from, to time.Time) (measurementRow, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var (
		best  measurementRow
		found bool
	)

	for _, key := range m.order {
		row := m.rows[key]
		if row.Timestamp.Before(from) || row.Timestamp.After(to) {
			continue
		}
		if !found || row.Value > best.Value || (row.Value == best.Value && row.Timestamp.After(best.Timestamp)) {
			best = row
			found = true
		}
	}

	return best, found
}

func storageKey(packetID, sourceID string, ts time.Time) string {
	return fmt.Sprintf("%s|%s|%s", packetID, sourceID, ts.UTC().Format(time.RFC3339Nano))
}

var (
	insertPattern   = regexp.MustCompile(`(?is)values\s*\((.*?)\)\s*on\s+conflict`)
	sourceIDPattern = regexp.MustCompile(`(?is)where\s+source_id\s*=\s*'([^']+)'::uuid`)
	rangePattern    = regexp.MustCompile(`(?is)where\s+ts\s+between\s+'([^']+)'::timestamptz\s+and\s+'([^']+)'::timestamptz`)
)

func parseInsert(statement string) (measurementRow, error) {
	matches := insertPattern.FindStringSubmatch(statement)
	if len(matches) != 2 {
		return measurementRow{}, fmt.Errorf("parse insert values: %w", errors.New("unexpected format"))
	}

	parts, err := splitValues(matches[1])
	if err != nil {
		return measurementRow{}, err
	}

	if len(parts) != 4 {
		return measurementRow{}, fmt.Errorf("parse insert values: %w", errors.New("unexpected value count"))
	}

	packetID, err := parseQuoted(parts[0])
	if err != nil {
		return measurementRow{}, fmt.Errorf("parse packet id: %w", err)
	}

	sourceID, err := parseQuoted(parts[1])
	if err != nil {
		return measurementRow{}, fmt.Errorf("parse source id: %w", err)
	}

	value, err := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
	if err != nil {
		return measurementRow{}, fmt.Errorf("parse value: %w", err)
	}

	tsValue, err := parseQuoted(parts[3])
	if err != nil {
		return measurementRow{}, fmt.Errorf("parse timestamp: %w", err)
	}

	timestamp, err := time.Parse(time.RFC3339Nano, tsValue)
	if err != nil {
		return measurementRow{}, fmt.Errorf("parse timestamp: %w", err)
	}

	return measurementRow{
		PacketID:  packetID,
		SourceID:  sourceID,
		Value:     value,
		Timestamp: timestamp,
	}, nil
}

func parseSourceID(statement string) (string, error) {
	matches := sourceIDPattern.FindStringSubmatch(statement)
	if len(matches) != 2 {
		return "", fmt.Errorf("parse source id: %w", errors.New("unexpected format"))
	}
	return matches[1], nil
}

func parseRange(statement string) (time.Time, time.Time, error) {
	matches := rangePattern.FindStringSubmatch(statement)
	if len(matches) != 3 {
		return time.Time{}, time.Time{}, fmt.Errorf("parse range: %w", errors.New("unexpected format"))
	}

	from, err := time.Parse(time.RFC3339Nano, matches[1])
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("parse range from: %w", err)
	}

	to, err := time.Parse(time.RFC3339Nano, matches[2])
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("parse range to: %w", err)
	}

	return from, to, nil
}

func splitValues(values string) ([]string, error) {
	var (
		parts   []string
		current strings.Builder
		inQuote bool
	)

	for i, r := range values {
		switch r {
		case '\'':
			inQuote = !inQuote
			current.WriteRune(r)
		case ',':
			if inQuote {
				current.WriteRune(r)
				continue
			}
			parts = append(parts, strings.TrimSpace(current.String()))
			current.Reset()
		default:
			current.WriteRune(r)
		}

		if i == len(values)-1 {
			parts = append(parts, strings.TrimSpace(current.String()))
		}
	}

	return parts, nil
}

func parseQuoted(input string) (string, error) {
	start := strings.IndexRune(input, '\'')
	end := strings.LastIndex(input, "'")
	if start == -1 || end == -1 || end <= start {
		return "", errors.New("missing quotes")
	}
	return input[start+1 : end], nil
}

func formatCSV(row measurementRow) (string, error) {
	var builder strings.Builder
	writer := csv.NewWriter(&builder)
	if err := writer.Write([]string{
		row.PacketID,
		row.SourceID,
		strconv.FormatFloat(row.Value, 'f', -1, 64),
		row.Timestamp.UTC().Format(time.RFC3339Nano),
	}); err != nil {
		return "", fmt.Errorf("write csv: %w", err)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", fmt.Errorf("flush csv: %w", err)
	}
	return builder.String(), nil
}
