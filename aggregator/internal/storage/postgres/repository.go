package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aggregator/internal/domain"
)

var ErrRepositoryClosed = errors.New("postgres repository is closed")

const (
	defaultBatchSize     = 100
	defaultFlushInterval = 250 * time.Millisecond
	defaultQueueSize     = defaultBatchSize * 4
)

type tickerFactory func(time.Duration) *time.Ticker

type PostgresRepository struct {
	db            *sql.DB
	batchSize     int
	flushInterval time.Duration
	queue         chan domain.PacketMax
	done          chan struct{}
	closeOnce     sync.Once
	dbCloseOnce   sync.Once
	mu            sync.Mutex
	lastErr       error
	closeCtx      context.Context
	closing       uint32
	makeTicker    tickerFactory
}

type Option func(*options)

type options struct {
	batchSize     int
	flushInterval time.Duration
	queueSize     int
	tickerFn      tickerFactory
}

func WithBatchSize(size int) Option {
	return func(o *options) {
		if size > 0 {
			o.batchSize = size
		}
	}
}

func WithFlushInterval(interval time.Duration) Option {
	return func(o *options) {
		if interval > 0 {
			o.flushInterval = interval
		}
	}
}

func WithQueueSize(size int) Option {
	return func(o *options) {
		if size > 0 {
			o.queueSize = size
		}
	}
}

func NewRepository(db *sql.DB, opts ...Option) (*PostgresRepository, error) {
	if db == nil {
		return nil, errors.New("postgres repository requires db instance")
	}

	db.SetConnMaxIdleTime(5 * time.Minute)
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("postgres ping failed: %w", err)
	}

	options := options{
		batchSize:     defaultBatchSize,
		flushInterval: defaultFlushInterval,
		queueSize:     defaultQueueSize,
		tickerFn: func(d time.Duration) *time.Ticker {
			return time.NewTicker(d)
		},
	}

	for _, opt := range opts {
		opt(&options)
	}

	if options.queueSize < options.batchSize {
		options.queueSize = options.batchSize
	}

	repo := &PostgresRepository{
		db:            db,
		batchSize:     options.batchSize,
		flushInterval: options.flushInterval,
		queue:         make(chan domain.PacketMax, options.queueSize),
		done:          make(chan struct{}),
		makeTicker:    options.tickerFn,
	}

	go repo.run()

	return repo, nil
}

func (r *PostgresRepository) Save(ctx context.Context, packets []domain.PacketMax) error {
	if len(packets) == 0 {
		return nil
	}

	if atomic.LoadUint32(&r.closing) == 1 {
		return ErrRepositoryClosed
	}

	if err := r.getLastError(); err != nil {
		return err
	}

	for i := range packets {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case r.queue <- packets[i]:
		}
	}

	return r.getLastError()
}

func (r *PostgresRepository) GetByID(ctx context.Context, id string) (*domain.PacketMax, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, timestamp, max_value FROM packet_max WHERE id = $1`, id)

	var result domain.PacketMax
	if err := row.Scan(&result.ID, &result.Timestamp, &result.MaxValue); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &result, nil
}

func (r *PostgresRepository) GetByTimeRange(ctx context.Context, from, to time.Time) ([]domain.PacketMax, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, timestamp, max_value FROM packet_max WHERE timestamp >= $1 AND timestamp < $2 ORDER BY timestamp`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.PacketMax
	for rows.Next() {
		var item domain.PacketMax
		if err := rows.Scan(&item.ID, &item.Timestamp, &item.MaxValue); err != nil {
			return nil, err
		}
		results = append(results, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

func (r *PostgresRepository) Close(ctx context.Context) error {
	if !atomic.CompareAndSwapUint32(&r.closing, 0, 1) {
		select {
		case <-r.done:
			return r.closeDB()
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	r.setCloseContext(ctx)
	r.closeOnce.Do(func() {
		close(r.queue)
	})

	select {
	case <-r.done:
		return r.closeDB()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *PostgresRepository) closeDB() error {
	var err error
	r.dbCloseOnce.Do(func() {
		err = r.db.Close()
	})
	return err
}

func (r *PostgresRepository) run() {
	defer close(r.done)

	buffer := make([]domain.PacketMax, 0, r.batchSize)

	var ticker *time.Ticker
	var tickerCh <-chan time.Time
	if r.flushInterval > 0 {
		ticker = r.makeTicker(r.flushInterval)
		tickerCh = ticker.C
		defer ticker.Stop()
	}

	for {
		select {
		case packet, ok := <-r.queue:
			if !ok {
				r.flushAndReset(&buffer)
				return
			}
			buffer = append(buffer, packet)
			if len(buffer) >= r.batchSize {
				r.flushAndReset(&buffer)
			}
		case <-tickerCh:
			r.flushAndReset(&buffer)
		}
	}
}

func (r *PostgresRepository) flushAndReset(buffer *[]domain.PacketMax) {
	if len(*buffer) == 0 {
		return
	}
	if err := r.flush(*buffer); err != nil {
		r.setLastError(err)
	}
	*buffer = (*buffer)[:0]
}

func (r *PostgresRepository) flush(batch []domain.PacketMax) error {
	ctx := r.getFlushContext()
	if ctx == nil {
		ctx = context.Background()
	}

	query, args := buildInsert(batch)
	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

func buildInsert(batch []domain.PacketMax) (string, []any) {
	var sb strings.Builder
	sb.WriteString("INSERT INTO packet_max (id, timestamp, max_value) VALUES ")

	args := make([]any, 0, len(batch)*3)
	for i, packet := range batch {
		if i > 0 {
			sb.WriteString(",")
		}
		base := i*3 + 1
		sb.WriteString(fmt.Sprintf("($%d,$%d,$%d)", base, base+1, base+2))
		args = append(args, packet.ID, packet.Timestamp.UTC(), packet.MaxValue)
	}

	sb.WriteString(" ON CONFLICT (id) DO UPDATE SET timestamp = EXCLUDED.timestamp, max_value = EXCLUDED.max_value")

	return sb.String(), args
}

func (r *PostgresRepository) getLastError() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastErr
}

func (r *PostgresRepository) setLastError(err error) {
	if err == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lastErr == nil {
		r.lastErr = err
	}
}

func (r *PostgresRepository) setCloseContext(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closeCtx == nil {
		r.closeCtx = ctx
	}
}

func (r *PostgresRepository) getFlushContext() context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closeCtx
}
