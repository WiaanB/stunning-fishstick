package eventbus

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
)

// ErrBusClosed is returned by Publish after the bus has been closed.
var ErrBusClosed = errors.New("eventbus: bus closed")

// Event mirrors trip.Event's shape without importing the trip package, so
// this package stays a leaf dependency usable by any domain.
type Event interface {
	EventType() string
}

// Handler processes a single event. Handlers are expected to be idempotent:
// the outbox dispatcher guarantees at-least-once delivery, not exactly-once.
type Handler func(ctx context.Context, e Event) error

// Bus is an async, worker-pool-backed event bus. Publish enqueues work and
// returns immediately; handlers run on background workers. Errors are
// never swallowed — a handler error is always surfaced via ErrorHandler
// (fail loud), since silent handler failures are how domain state and
// side effects (notifications, payments) drift apart.
type Bus struct {
	handlers     map[string][]Handler
	mu           sync.RWMutex
	queue        chan job
	wg           sync.WaitGroup
	closed       bool
	errMu        sync.Mutex
	errorHandler func(eventType string, err error)
}

type job struct {
	ctx   context.Context
	event Event
}

// Option configures a Bus at construction time.
type Option func(*Bus)

// WithErrorHandler overrides the default (log.Printf) handler-error
// reporter. It is called synchronously from the worker goroutine that
// observed the failure.
func WithErrorHandler(fn func(eventType string, err error)) Option {
	return func(b *Bus) { b.errorHandler = fn }
}

// New starts a Bus with the given number of worker goroutines consuming
// from a buffered queue.
func New(workers, queueSize int, opts ...Option) *Bus {
	if workers < 1 {
		workers = 1
	}
	if queueSize < 1 {
		queueSize = 1
	}
	b := &Bus{
		handlers: make(map[string][]Handler),
		queue:    make(chan job, queueSize),
	}
	b.errorHandler = func(eventType string, err error) {
		log.Printf("eventbus: handler error for %s: %v", eventType, err)
	}
	for _, opt := range opts {
		opt(b)
	}
	for i := 0; i < workers; i++ {
		b.wg.Add(1)
		go b.worker()
	}
	return b
}

func (b *Bus) worker() {
	defer b.wg.Done()
	for j := range b.queue {
		b.dispatch(j.ctx, j.event)
	}
}

func (b *Bus) dispatch(ctx context.Context, e Event) {
	b.mu.RLock()
	handlers := b.handlers[e.EventType()]
	b.mu.RUnlock()

	for _, h := range handlers {
		if err := h(ctx, e); err != nil {
			b.errorHandler(e.EventType(), err)
		}
	}
}

// Subscribe registers a handler for the given event type. Must be called
// before the matching events are published; there is no replay.
func (b *Bus) Subscribe(eventType string, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], h)
}

// Publish enqueues events for async dispatch. It blocks only if the queue
// is full, which is treated as backpressure rather than an error. After the
// bus has been closed, Publish returns ErrBusClosed.
func (b *Bus) Publish(ctx context.Context, events ...Event) error {
	b.errMu.Lock()
	if b.closed {
		b.errMu.Unlock()
		return ErrBusClosed
	}
	b.errMu.Unlock()

	for _, e := range events {
		select {
		case b.queue <- job{ctx: ctx, event: e}:
		case <-ctx.Done():
			return fmt.Errorf("eventbus: publish %s: %w", e.EventType(), ctx.Err())
		}
	}
	return nil
}

// Close stops accepting new events and waits for in-flight handlers to
// finish. Safe to call multiple times; subsequent calls are no-ops.
func (b *Bus) Close() {
	b.errMu.Lock()
	if b.closed {
		b.errMu.Unlock()
		return
	}
	b.closed = true
	b.errMu.Unlock()

	close(b.queue)
	b.wg.Wait()
}
