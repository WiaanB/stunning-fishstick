// cmd/api wires the API process: Postgres pool, in-process event bus, and
// the outbox dispatcher that bridges them. HTTP handlers for trip commands
// land here as the roadmap's sqlc-backed repository is implemented; today
// this exposes only a health endpoint. See taxi-platform/04 Backend
// Scaffolding.md and taxi-platform/05 Roadmap.md.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"taxi-platform/internal/platform/eventbus"
	"taxi-platform/internal/platform/postgres"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("api: %v", err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dsn := getEnvOrDefault("DATABASE_URL", "postgres://localhost:5432/taxi_platform")

	pool, err := postgres.NewPool(ctx, dsn)
	if err != nil {
		return err
	}

	bus := eventbus.New(4, 256)
	defer closeInOrder(bus, pool)

	dispatcher := postgres.NewDispatcher(pool, publishToBus(bus), 100, time.Second)
	dispatcherErrs := make(chan error, 1)
	go func() {
		if err := dispatcher.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			dispatcherErrs <- err
			return
		}
		dispatcherErrs <- nil
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler(pool))

	addr := getEnvOrDefault("API_ADDR", ":8080")
	server := &http.Server{Addr: addr, Handler: mux}

	serverErrs := make(chan error, 1)
	go func() {
		log.Printf("api: listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrs <- err
			return
		}
		serverErrs <- nil
	}()

	return awaitShutdown(ctx, server.Shutdown, serverErrs, dispatcherErrs)
}

// awaitShutdown blocks until the process should stop: either ctx is
// cancelled, triggering a graceful HTTP shutdown via shutdown (a 5s timeout
// is applied) followed by waiting for the dispatcher to observe the
// cancellation, or the server/dispatcher goroutines report an error first.
// A shutdown error is returned immediately without waiting on dispatcherErrs.
func awaitShutdown(ctx context.Context, shutdown func(context.Context) error, serverErrs, dispatcherErrs <-chan error) error {
	select {
	case <-ctx.Done():
		log.Println("api: shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdown(shutdownCtx); err != nil {
			return err
		}
		return <-dispatcherErrs
	case err := <-serverErrs:
		return err
	case err := <-dispatcherErrs:
		return err
	}
}

// closeInOrder closes each closer in the given order. Used instead of
// separate defer statements so shutdown sequencing (stop consuming events
// before closing the DB the dispatcher reads from) is explicit rather than
// relying on Go's implicit LIFO defer order.
func closeInOrder(closers ...interface{ Close() }) {
	for _, c := range closers {
		c.Close()
	}
}

// getEnvOrDefault returns the environment variable named key, or fallback
// if it's unset or empty.
func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// outboxEvent adapts a postgres.OutboxRecord to eventbus.Event so the
// dispatcher can hand records to the bus without either package depending
// on the other's concrete types.
type outboxEvent struct {
	postgres.OutboxRecord
}

func (e outboxEvent) EventType() string { return e.OutboxRecord.EventType }

func publishToBus(bus *eventbus.Bus) postgres.PublishFunc {
	return func(ctx context.Context, r postgres.OutboxRecord) error {
		return bus.Publish(ctx, outboxEvent{r})
	}
}

func healthHandler(pool interface{ Ping(context.Context) error }) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "unavailable", "error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}
