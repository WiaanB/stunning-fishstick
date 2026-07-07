// Package postgres holds infrastructure for talking to the durable write
// model: connection pooling and the transactional outbox. It deliberately
// does not import the trip package — domain events reach it as opaque
// records so this package stays reusable across aggregates.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool opens a connection pool against dsn (e.g.
// "postgres://user:pass@localhost:5432/taxi_platform").
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres: create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	return pool, nil
}
