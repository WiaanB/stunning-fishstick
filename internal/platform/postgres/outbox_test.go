package postgres

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func insertOutboxRow(t *testing.T, pool *pgxpool.Pool, eventType string, createdAt time.Time) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO outbox_events (id, aggregate_id, event_type, payload, created_at) VALUES ($1, $2, $3, $4, $5)`,
		id, uuid.New(), eventType, []byte(`{}`), createdAt,
	)
	if err != nil {
		t.Fatalf("insert outbox row: %v", err)
	}
	return id
}

func assertDispatched(t *testing.T, pool *pgxpool.Pool, id uuid.UUID, want bool) {
	t.Helper()
	var dispatchedAt *time.Time
	if err := pool.QueryRow(context.Background(),
		`SELECT dispatched_at FROM outbox_events WHERE id = $1`, id).Scan(&dispatchedAt); err != nil {
		t.Fatalf("query dispatched_at: %v", err)
	}
	if got := dispatchedAt != nil; got != want {
		t.Fatalf("expected dispatched=%v for %s, got %v", want, id, got)
	}
}

func TestInsertOutboxWithinCommittedTransactionIsVisible(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	aggregateID := uuid.New()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := InsertOutbox(ctx, tx, aggregateID, "trip.requested", map[string]int{"seat_count": 2}); err != nil {
		t.Fatalf("InsertOutbox: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var count int
	err = pool.QueryRow(ctx,
		`SELECT count(*) FROM outbox_events WHERE aggregate_id = $1 AND event_type = $2`,
		aggregateID, "trip.requested").Scan(&count)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row, got %d", count)
	}
}

func TestInsertOutboxRolledBackTransactionLeavesNoRow(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	aggregateID := uuid.New()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := InsertOutbox(ctx, tx, aggregateID, "trip.requested", map[string]int{"seat_count": 2}); err != nil {
		t.Fatalf("InsertOutbox: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE aggregate_id = $1`, aggregateID).Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows after rollback, got %d", count)
	}
}

func TestNewDispatcherDefaultsInvalidBatchSizeAndInterval(t *testing.T) {
	d := NewDispatcher(nil, nil, 0, 0)
	if d.batchSize != 100 {
		t.Fatalf("expected default batchSize 100, got %d", d.batchSize)
	}
	if d.interval != time.Second {
		t.Fatalf("expected default interval 1s, got %s", d.interval)
	}
}

func TestPollOnceDispatchesUndispatchedRowsInCreatedOrder(t *testing.T) {
	pool := newTestPool(t)
	base := time.Now().UTC()
	id1 := insertOutboxRow(t, pool, "trip.requested", base)
	id2 := insertOutboxRow(t, pool, "trip.quoted", base.Add(time.Second))

	var published []uuid.UUID
	publish := func(ctx context.Context, r OutboxRecord) error {
		published = append(published, r.ID)
		return nil
	}
	d := NewDispatcher(pool, publish, 100, time.Second)

	n, err := d.pollOnce(context.Background())
	if err != nil {
		t.Fatalf("pollOnce: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 dispatched, got %d", n)
	}
	if len(published) != 2 || published[0] != id1 || published[1] != id2 {
		t.Fatalf("expected [%s %s] in order, got %v", id1, id2, published)
	}
	assertDispatched(t, pool, id1, true)
	assertDispatched(t, pool, id2, true)
}

func TestPollOnceRespectsBatchSize(t *testing.T) {
	pool := newTestPool(t)
	base := time.Now().UTC()
	for i := 0; i < 3; i++ {
		insertOutboxRow(t, pool, "trip.requested", base.Add(time.Duration(i)*time.Second))
	}

	var calls int32
	publish := func(ctx context.Context, r OutboxRecord) error {
		atomic.AddInt32(&calls, 1)
		return nil
	}
	d := NewDispatcher(pool, publish, 2, time.Second)

	n, err := d.pollOnce(context.Background())
	if err != nil {
		t.Fatalf("pollOnce: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 dispatched (batchSize), got %d", n)
	}
	if calls != 2 {
		t.Fatalf("expected publish called twice, got %d", calls)
	}
}

func TestPollOnceLeavesFailedRecordsUndispatchedAndContinues(t *testing.T) {
	pool := newTestPool(t)
	base := time.Now().UTC()
	failID := insertOutboxRow(t, pool, "trip.requested", base)
	okID := insertOutboxRow(t, pool, "trip.quoted", base.Add(time.Second))

	publish := func(ctx context.Context, r OutboxRecord) error {
		if r.ID == failID {
			return errors.New("boom")
		}
		return nil
	}
	d := NewDispatcher(pool, publish, 100, time.Second)

	n, err := d.pollOnce(context.Background())
	if err != nil {
		t.Fatalf("pollOnce: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 dispatched, got %d", n)
	}
	assertDispatched(t, pool, failID, false)
	assertDispatched(t, pool, okID, true)
}

func TestDispatcherRunPublishesAndReturnsCanceledOnCancel(t *testing.T) {
	pool := newTestPool(t)
	insertOutboxRow(t, pool, "trip.requested", time.Now().UTC())

	published := make(chan uuid.UUID, 1)
	publish := func(ctx context.Context, r OutboxRecord) error {
		published <- r.ID
		return nil
	}
	d := NewDispatcher(pool, publish, 100, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()

	select {
	case <-published:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the row to be published")
	}

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancellation")
	}
}

// TestDispatcherRunLogsAndContinuesOnPollError uses its own pool (closed
// immediately) rather than newTestPool's shared one, so closing it early
// here can't affect other tests.
func TestDispatcherRunLogsAndContinuesOnPollError(t *testing.T) {
	if !dockerAvailable {
		t.Skip("Docker is not available; skipping Postgres integration test")
	}
	pool, err := NewPool(context.Background(), testDSN)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	pool.Close() // every pollOnce's pool.Begin will now fail

	publish := func(ctx context.Context, r OutboxRecord) error { return nil }
	d := NewDispatcher(pool, publish, 100, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()

	select {
	case err := <-done:
		t.Fatalf("Run returned early instead of continuing past poll errors: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancellation")
	}
}
