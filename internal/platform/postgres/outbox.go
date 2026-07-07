package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OutboxRecord is a serialized domain event awaiting dispatch.
type OutboxRecord struct {
	ID          uuid.UUID
	AggregateID uuid.UUID
	EventType   string
	Payload     json.RawMessage
	CreatedAt   time.Time
}

// InsertOutbox writes a batch of outbox rows within tx. Call it in the same
// transaction as the aggregate's state-changing write (e.g. the trips row
// update) so the two commit or roll back together — this is the atomicity
// guarantee the outbox pattern exists for.
func InsertOutbox(ctx context.Context, tx pgx.Tx, aggregateID uuid.UUID, eventType string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("postgres: marshal outbox payload for %s: %w", eventType, err)
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO outbox_events (aggregate_id, event_type, payload) VALUES ($1, $2, $3)`,
		aggregateID, eventType, body,
	)
	if err != nil {
		return fmt.Errorf("postgres: insert outbox row for %s: %w", eventType, err)
	}
	return nil
}

// PublishFunc hands a dispatched record to whatever downstream consumer
// applies it — typically eventbus.Bus.Publish via a small adapter, kept out
// of this package to avoid a dependency on event types.
type PublishFunc func(ctx context.Context, r OutboxRecord) error

// Dispatcher polls outbox_events for undispatched rows and hands each to
// PublishFunc, marking it dispatched once that call succeeds. Failed
// publishes are left undispatched and retried on the next poll — delivery
// is at-least-once, so downstream handlers must be idempotent.
type Dispatcher struct {
	pool      *pgxpool.Pool
	publish   PublishFunc
	batchSize int
	interval  time.Duration
}

func NewDispatcher(pool *pgxpool.Pool, publish PublishFunc, batchSize int, interval time.Duration) *Dispatcher {
	if batchSize < 1 {
		batchSize = 100
	}
	if interval <= 0 {
		interval = time.Second
	}
	return &Dispatcher{pool: pool, publish: publish, batchSize: batchSize, interval: interval}
}

// Run polls until ctx is cancelled. It's meant to be launched in its own
// goroutine from cmd/api/main.go.
func (d *Dispatcher) Run(ctx context.Context) error {
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			n, err := d.pollOnce(ctx)
			if err != nil {
				log.Printf("outbox: poll failed: %v", err)
				continue
			}
			if n > 0 {
				log.Printf("outbox: dispatched %d event(s)", n)
			}
		}
	}
}

// pollOnce claims up to batchSize undispatched rows with FOR UPDATE SKIP
// LOCKED (so multiple dispatcher instances can run concurrently without
// double-claiming a row), publishes each, and marks successes dispatched
// within the same transaction.
func (d *Dispatcher) pollOnce(ctx context.Context) (int, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("postgres: begin dispatch tx: %w", err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT id, aggregate_id, event_type, payload, created_at
		FROM outbox_events
		WHERE dispatched_at IS NULL
		ORDER BY created_at
		LIMIT $1
		FOR UPDATE SKIP LOCKED`,
		d.batchSize,
	)
	if err != nil {
		return 0, fmt.Errorf("postgres: query outbox batch: %w", err)
	}

	var records []OutboxRecord
	for rows.Next() {
		var r OutboxRecord
		if err := rows.Scan(&r.ID, &r.AggregateID, &r.EventType, &r.Payload, &r.CreatedAt); err != nil {
			rows.Close()
			return 0, fmt.Errorf("postgres: scan outbox row: %w", err)
		}
		records = append(records, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("postgres: iterate outbox rows: %w", err)
	}

	dispatched := 0
	for _, r := range records {
		if err := d.publish(ctx, r); err != nil {
			// Fail loud in the log, but leave the row undispatched rather
			// than aborting the whole batch — one bad event shouldn't
			// block the rest.
			log.Printf("outbox: publish %s (event %s) failed, will retry: %v", r.ID, r.EventType, err)
			continue
		}
		if _, err := tx.Exec(ctx, `UPDATE outbox_events SET dispatched_at = now() WHERE id = $1`, r.ID); err != nil {
			return dispatched, fmt.Errorf("postgres: mark outbox row %s dispatched: %w", r.ID, err)
		}
		dispatched++
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("postgres: commit dispatch tx: %w", err)
	}
	return dispatched, nil
}
