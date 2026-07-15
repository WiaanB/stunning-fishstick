package postgres

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// testDSN is set by TestMain once the shared test container is up.
// dockerAvailable gates whether DB-touching tests run or skip themselves —
// there is no *testing.T available in TestMain to call
// testcontainers.SkipIfProviderIsNotHealthy directly, so tests check this
// flag and skip individually instead.
var (
	testDSN         string
	dockerAvailable bool
)

// TestMain starts one Postgres container for the whole package's test run
// (rather than one per test) and applies the real migration file, so the
// test schema can never drift from migrations/0001_init.sql. If Docker
// isn't available, tests still run but skip themselves via newTestPool.
func TestMain(m *testing.M) {
	ctx := context.Background()

	provider, err := testcontainers.ProviderDocker.GetProvider()
	if err != nil || provider.Health(ctx) != nil {
		os.Exit(m.Run())
	}
	dockerAvailable = true

	migrationPath, err := filepath.Abs("../../../migrations/0001_init.sql")
	if err != nil {
		log.Fatalf("postgres: resolve migration path: %v", err)
	}

	ctr, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithInitScripts(migrationPath),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		log.Fatalf("postgres: start test container: %v", err)
	}

	testDSN, err = ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("postgres: test container connection string: %v", err)
	}

	code := m.Run()

	if err := ctr.Terminate(ctx); err != nil {
		log.Printf("postgres: terminate test container: %v", err)
	}
	os.Exit(code)
}

// newTestPool opens a pool against the shared test container, registering
// cleanup, and truncates both tables first so tests are isolated from each
// other regardless of run order. Skips the test if Docker isn't available.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if !dockerAvailable {
		t.Skip("Docker is not available; skipping Postgres integration test")
	}

	pool, err := NewPool(context.Background(), testDSN)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(pool.Close)

	truncateTables(t, pool)
	return pool
}

func truncateTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), "TRUNCATE trips, outbox_events"); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}
