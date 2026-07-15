package postgres

import (
	"context"
	"testing"
)

func TestNewPoolPingsSuccessfully(t *testing.T) {
	if !dockerAvailable {
		t.Skip("Docker is not available; skipping Postgres integration test")
	}

	pool, err := NewPool(context.Background(), testDSN)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestNewPoolUnreachableDSNReturnsError(t *testing.T) {
	if !dockerAvailable {
		t.Skip("Docker is not available; skipping Postgres integration test")
	}

	// Port 1 is a reserved, unlisted port: nothing accepts connections there,
	// so the ping is guaranteed to fail without a real network dependency.
	_, err := NewPool(context.Background(), "postgres://user:pass@127.0.0.1:1/nope")
	if err == nil {
		t.Fatal("expected an error connecting to an unreachable DSN")
	}
}
