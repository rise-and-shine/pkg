// Package testdb opens the PostgreSQL database taskmill's tests run against.
//
// The tests need a real PostgreSQL: the attempt counter is advanced by the
// dequeue query itself, so no fake can stand in for it. The database is named by
// the TEST_POSTGRES_DSN environment variable, and a test without it is skipped:
//
//	TEST_POSTGRES_DSN='postgres://postgres:postgres@localhost:5432/taskmill_test?sslmode=disable' go test ./taskmill/...
//
// The database must exist. The first test creates the taskmill schema in it.
// Tests keep to queue names of their own, so they can run in parallel and
// leave nothing that another run reads. A database whose taskmill schema is
// older than the code under test must be dropped and created again.
package testdb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/rise-and-shine/pkg/taskmill/internal/config"
	"github.com/rise-and-shine/pkg/taskmill/internal/pgqueue"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// DSNEnv names the environment variable that holds the test database's DSN.
const DSNEnv = "TEST_POSTGRES_DSN"

const migrateTimeout = 30 * time.Second

// Open connects to the test database and migrates the taskmill schema in it, or
// skips the test when DSNEnv is unset.
func Open(t *testing.T) *bun.DB {
	t.Helper()

	dsn := os.Getenv(DSNEnv)
	if dsn == "" {
		t.Skipf("%s is not set; this test needs a PostgreSQL database", DSNEnv)
	}

	sqldb, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("testdb: open %s: %v", DSNEnv, err)
	}

	db := bun.NewDB(sqldb, pgdialect.New())
	t.Cleanup(func() { _ = db.Close() })

	ctx, cancel := context.WithTimeout(t.Context(), migrateTimeout)
	defer cancel()

	// Every test opens the database, and parallel tests (and test binaries) run
	// against one schema. Migrating while other tests poll the queue deadlocks:
	// the migration replaces the queue's trigger and view. So the schema is
	// created once, by the first test, under a transaction-scoped advisory lock
	// that every other test waits on, and never migrated again while in use.
	err = db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, lockErr := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtext('taskmill-testdb-migrate'))")
		if lockErr != nil {
			return lockErr
		}

		var exists bool
		existsErr := tx.NewRaw("SELECT to_regclass(?) IS NOT NULL", config.SchemaName()+".task_queue").
			Scan(ctx, &exists)
		if existsErr != nil || exists {
			return existsErr
		}

		queue, queueErr := pgqueue.NewQueue(config.SchemaName(), config.RetryStrategy())
		if queueErr != nil {
			return queueErr
		}

		return queue.Migrate(ctx, tx, config.SchemaName())
	})
	if err != nil {
		t.Fatalf("testdb: migrate taskmill schema: %v", err)
	}

	return db
}

// QueueName returns a queue name no other test or run uses.
func QueueName(t *testing.T) string {
	t.Helper()

	name := strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
	return fmt.Sprintf("test-%s-%s", name, uuid.NewString()[:8])
}
