package postgres

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func insertOutboxRow(t *testing.T, pool *pgxpool.Pool, eventType string, createdAt time.Time) uuid.UUID {
	t.Helper()
	return insertOutboxRowForAggregate(t, pool, uuid.New(), eventType, createdAt)
}

func insertOutboxRowForAggregate(t *testing.T, pool *pgxpool.Pool, aggregateID uuid.UUID, eventType string, createdAt time.Time) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO outbox_events (id, aggregate_id, event_type, payload, created_at) VALUES ($1, $2, $3, $4, $5)`,
		id, aggregateID, eventType, []byte(`{}`), createdAt,
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

// TestPollOnceDoesNotMarkDispatchedUntilHandlerCompletes pins the fix for
// the ack-before-handler-completion bug: dispatched_at must stay NULL while
// publish (which now runs the handler synchronously) is still in flight.
func TestPollOnceDoesNotMarkDispatchedUntilHandlerCompletes(t *testing.T) {
	pool := newTestPool(t)
	id := insertOutboxRow(t, pool, "trip.requested", time.Now().UTC())

	handlerStarted := make(chan struct{})
	release := make(chan struct{})
	publish := func(ctx context.Context, r OutboxRecord) error {
		close(handlerStarted)
		<-release
		return nil
	}
	d := NewDispatcher(pool, publish, 100, time.Second)

	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := d.pollOnce(context.Background()); err != nil {
			t.Errorf("pollOnce: %v", err)
		}
	}()

	select {
	case <-handlerStarted:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}

	assertDispatched(t, pool, id, false)

	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("pollOnce did not return after handler completed")
	}

	assertDispatched(t, pool, id, true)
}

// TestPollOnceRetriesFailedRecordOnNextPoll pins that a handler failure
// leaves the row immediately retryable (claim released), not stuck behind
// a lease timeout.
func TestPollOnceRetriesFailedRecordOnNextPoll(t *testing.T) {
	pool := newTestPool(t)
	id := insertOutboxRow(t, pool, "trip.requested", time.Now().UTC())

	var attempt int32
	publish := func(ctx context.Context, r OutboxRecord) error {
		if atomic.AddInt32(&attempt, 1) == 1 {
			return errors.New("boom")
		}
		return nil
	}
	d := NewDispatcher(pool, publish, 100, time.Second)

	if _, err := d.pollOnce(context.Background()); err != nil {
		t.Fatalf("first pollOnce: %v", err)
	}
	assertDispatched(t, pool, id, false)

	n, err := d.pollOnce(context.Background())
	if err != nil {
		t.Fatalf("second pollOnce: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected the retried record to dispatch on the second poll, got %d", n)
	}
	assertDispatched(t, pool, id, true)
}

// TestPollOnceBreaksCreatedAtTiesBySeq pins deterministic ordering for two
// rows on the same aggregate sharing an identical created_at: insertion
// order (seq) must decide, not database-undefined tie ordering.
func TestPollOnceBreaksCreatedAtTiesBySeq(t *testing.T) {
	pool := newTestPool(t)
	aggregateID := uuid.New()
	base := time.Now().UTC()
	id1 := insertOutboxRowForAggregate(t, pool, aggregateID, "trip.requested", base)
	id2 := insertOutboxRowForAggregate(t, pool, aggregateID, "trip.quoted", base)

	var published []uuid.UUID
	publish := func(ctx context.Context, r OutboxRecord) error {
		published = append(published, r.ID)
		return nil
	}
	d := NewDispatcher(pool, publish, 100, time.Second)

	// Same aggregate: at most one row claims per pollOnce, so drive two rounds.
	if _, err := d.pollOnce(context.Background()); err != nil {
		t.Fatalf("first pollOnce: %v", err)
	}
	if _, err := d.pollOnce(context.Background()); err != nil {
		t.Fatalf("second pollOnce: %v", err)
	}

	if len(published) != 2 || published[0] != id1 || published[1] != id2 {
		t.Fatalf("expected [%s %s] in insertion order despite equal created_at, got %v", id1, id2, published)
	}
}

// TestConcurrentPollOnceNeverClaimsSameAggregateTwice pins the cross-instance
// ordering fix: while one dispatcher's claimed row for an aggregate is still
// being handled, a second dispatcher instance must not claim any other row
// for that same aggregate.
func TestConcurrentPollOnceNeverClaimsSameAggregateTwice(t *testing.T) {
	pool := newTestPool(t)
	aggregateID := uuid.New()
	base := time.Now().UTC()
	id1 := insertOutboxRowForAggregate(t, pool, aggregateID, "trip.requested", base)
	insertOutboxRowForAggregate(t, pool, aggregateID, "trip.quoted", base.Add(time.Second))

	handlerStarted := make(chan struct{})
	release := make(chan struct{})
	var startOnce sync.Once
	publish := func(ctx context.Context, r OutboxRecord) error {
		if r.ID == id1 {
			startOnce.Do(func() { close(handlerStarted) })
			<-release
		}
		return nil
	}
	d1 := NewDispatcher(pool, publish, 100, time.Second)
	d2 := NewDispatcher(pool, publish, 100, time.Second)

	done1 := make(chan struct{})
	go func() {
		defer close(done1)
		if _, err := d1.pollOnce(context.Background()); err != nil {
			t.Errorf("d1 pollOnce: %v", err)
		}
	}()

	select {
	case <-handlerStarted:
	case <-time.After(time.Second):
		t.Fatal("d1's handler did not start")
	}

	n2, err := d2.pollOnce(context.Background())
	if err != nil {
		t.Fatalf("d2 pollOnce: %v", err)
	}
	if n2 != 0 {
		t.Fatalf("expected d2 to claim nothing while d1 has the same aggregate in flight, got %d", n2)
	}

	close(release)
	select {
	case <-done1:
	case <-time.After(time.Second):
		t.Fatal("d1 pollOnce did not return")
	}
}

// TestSlowHandlerDoesNotBlockOtherAggregates pins the fix for the
// lock-contention bug: the original single-transaction pollOnce locked
// every selected row (across all aggregates) for the whole batch's publish
// loop, so a slow handler for one aggregate starved dispatch of unrelated
// aggregates too. A second dispatcher instance must still be able to claim
// and dispatch an unrelated aggregate's row while the first is stuck.
func TestSlowHandlerDoesNotBlockOtherAggregates(t *testing.T) {
	pool := newTestPool(t)
	base := time.Now().UTC()
	slowID := insertOutboxRow(t, pool, "trip.requested", base)
	otherID := insertOutboxRow(t, pool, "trip.requested", base.Add(time.Second))

	handlerStarted := make(chan struct{})
	release := make(chan struct{})
	var startOnce sync.Once
	publish := func(ctx context.Context, r OutboxRecord) error {
		if r.ID == slowID {
			startOnce.Do(func() { close(handlerStarted) })
			<-release
		}
		return nil
	}
	// d1's batch size is capped to 1 so it claims only the slow aggregate's
	// row, leaving the unrelated aggregate's row for d2 to claim itself --
	// otherwise d1 would claim both in one poll and d2 would correctly see
	// nothing left, which wouldn't exercise the lock-contention fix at all.
	d1 := NewDispatcher(pool, publish, 1, time.Second)
	d2 := NewDispatcher(pool, publish, 100, time.Second)

	done1 := make(chan struct{})
	go func() {
		defer close(done1)
		if _, err := d1.pollOnce(context.Background()); err != nil {
			t.Errorf("d1 pollOnce: %v", err)
		}
	}()

	select {
	case <-handlerStarted:
	case <-time.After(time.Second):
		t.Fatal("d1's handler did not start")
	}

	done2 := make(chan struct{})
	go func() {
		defer close(done2)
		n, err := d2.pollOnce(context.Background())
		if err != nil {
			t.Errorf("d2 pollOnce: %v", err)
		}
		if n != 1 {
			t.Errorf("expected d2 to dispatch the unrelated aggregate's row, got %d", n)
		}
	}()

	select {
	case <-done2:
	case <-time.After(time.Second):
		t.Fatal("d2 pollOnce blocked on an unrelated aggregate's slow handler")
	}
	assertDispatched(t, pool, otherID, true)

	close(release)
	select {
	case <-done1:
	case <-time.After(time.Second):
		t.Fatal("d1 pollOnce did not return")
	}
	assertDispatched(t, pool, slowID, true)
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
