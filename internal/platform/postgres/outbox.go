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

// PublishFunc hands a claimed record to whatever downstream consumer
// applies it — typically eventbus.Bus.Dispatch via a small adapter, kept
// out of this package to avoid a dependency on event types. It must not
// return until the handler(s) have actually run: pollOnce only acks a row
// once PublishFunc returns nil, so an async fire-and-forget PublishFunc
// would defeat that guarantee.
type PublishFunc func(ctx context.Context, r OutboxRecord) error

// claimLease bounds how long a claimed-but-undispatched row stays invisible
// to other poll cycles. It only matters if the process crashes between
// claiming a row and acking or releasing it; a normal failed publish
// releases the claim immediately rather than waiting this out.
const claimLease = 30 * time.Second

// claimFetchMultiplier over-fetches raw candidate rows beyond batchSize
// before deduping to one row per aggregate in Go, since multiple fetched
// rows can belong to the same (not-yet-claimed) aggregate.
const claimFetchMultiplier = 5

// Dispatcher polls outbox_events for undispatched rows and hands each to
// PublishFunc, marking it dispatched once PublishFunc actually returns
// (i.e. once the handler has run, not merely enqueued). Failed publishes
// release their claim so they're retried on the next poll — delivery is
// at-least-once, so downstream handlers must be idempotent.
//
// pollOnce claims rows in a short transaction, publishes each outside any
// transaction (so a slow handler holds no DB row lock), then acks in a
// separate small statement. At most one row per aggregate is ever claimed
// at a time across all Dispatcher instances sharing this table, which
// keeps per-aggregate delivery order intact even under concurrent pollers.
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

// pollOnce claims up to batchSize rows (at most one per aggregate) via
// claimBatch, publishes each outside any transaction, and acks or releases
// the claim depending on the result.
func (d *Dispatcher) pollOnce(ctx context.Context) (int, error) {
	claimed, err := d.claimBatch(ctx)
	if err != nil {
		return 0, err
	}

	dispatched := 0
	for _, r := range claimed {
		if err := d.publish(ctx, r); err != nil {
			// Fail loud in the log, but release the claim rather than
			// aborting the whole batch — one bad event shouldn't block the
			// rest, and this aggregate's row becomes claimable again on the
			// very next poll.
			log.Printf("outbox: publish %s (event %s) failed, will retry: %v", r.ID, r.EventType, err)
			if err := d.releaseClaim(ctx, r.ID); err != nil {
				return dispatched, fmt.Errorf("postgres: release claim on %s: %w", r.ID, err)
			}
			continue
		}
		if err := d.ack(ctx, r.ID); err != nil {
			return dispatched, fmt.Errorf("postgres: ack outbox row %s: %w", r.ID, err)
		}
		dispatched++
	}
	return dispatched, nil
}

// claimBatch selects candidate rows with FOR UPDATE SKIP LOCKED (so
// multiple dispatcher instances can run concurrently without contending on
// the same rows), keeps only the oldest row per aggregate so at most one
// row per aggregate is ever claimed at a time, and marks those claimed_at
// before committing this short transaction — no row lock is held once
// claimBatch returns, so a slow publish later can't block other work.
func (d *Dispatcher) claimBatch(ctx context.Context) ([]OutboxRecord, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: begin claim tx: %w", err)
	}
	defer tx.Rollback(ctx)

	leaseCutoff := time.Now().UTC().Add(-claimLease)
	rows, err := tx.Query(ctx, `
		SELECT id, aggregate_id, event_type, payload, created_at
		FROM outbox_events o
		WHERE dispatched_at IS NULL
		  AND (claimed_at IS NULL OR claimed_at < $1)
		  AND NOT EXISTS (
		      SELECT 1 FROM outbox_events o2
		      WHERE o2.aggregate_id = o.aggregate_id
		        AND o2.dispatched_at IS NULL
		        AND o2.claimed_at IS NOT NULL
		        AND o2.claimed_at >= $1
		  )
		ORDER BY created_at, seq
		LIMIT $2
		FOR UPDATE SKIP LOCKED`,
		leaseCutoff, d.batchSize*claimFetchMultiplier,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: query outbox batch: %w", err)
	}

	var candidates []OutboxRecord
	for rows.Next() {
		var r OutboxRecord
		if err := rows.Scan(&r.ID, &r.AggregateID, &r.EventType, &r.Payload, &r.CreatedAt); err != nil {
			rows.Close()
			return nil, fmt.Errorf("postgres: scan outbox row: %w", err)
		}
		candidates = append(candidates, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate outbox rows: %w", err)
	}

	seenAggregates := make(map[uuid.UUID]bool, len(candidates))
	claimed := make([]OutboxRecord, 0, d.batchSize)
	for _, r := range candidates {
		if seenAggregates[r.AggregateID] {
			continue
		}
		seenAggregates[r.AggregateID] = true
		claimed = append(claimed, r)
		if len(claimed) == d.batchSize {
			break
		}
	}

	for _, r := range claimed {
		if _, err := tx.Exec(ctx, `UPDATE outbox_events SET claimed_at = now() WHERE id = $1`, r.ID); err != nil {
			return nil, fmt.Errorf("postgres: claim outbox row %s: %w", r.ID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("postgres: commit claim tx: %w", err)
	}
	return claimed, nil
}

func (d *Dispatcher) ack(ctx context.Context, id uuid.UUID) error {
	_, err := d.pool.Exec(ctx, `UPDATE outbox_events SET dispatched_at = now() WHERE id = $1`, id)
	return err
}

func (d *Dispatcher) releaseClaim(ctx context.Context, id uuid.UUID) error {
	_, err := d.pool.Exec(ctx, `UPDATE outbox_events SET claimed_at = NULL WHERE id = $1`, id)
	return err
}
