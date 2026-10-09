// Package testdb opens the PostgreSQL database taskmill's tests run against.
//
// The tests need a real PostgreSQL: the attempt counter is advanced by the
// dequeue query itself, so no fake can stand in for it. The database is named by
// the TEST_POSTGRES_DSN environment variable, and a test without it is skipped:
//
//	TEST_POSTGRES_DSN='postgres://postgres:postgres@localhost:5432/taskmill_test?sslmode=disable' go test ./taskmill/...
//
// The database must exist. Tests create the taskmill schema in it and keep to
// queue names of their own, so they can run in parallel and leave nothing that
// another run reads.
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
	"github.com/rise-and-shine/pkg/taskmill"
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

	// Parallel tests migrate at once, and CREATE ... IF NOT EXISTS still races
	// on the catalog. One transaction-scoped advisory lock serializes them.
	err = db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, lockErr := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtext('taskmill-testdb-migrate'))")
		if lockErr != nil {
			return lockErr
		}

		return taskmill.Migrate(ctx, db)
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
