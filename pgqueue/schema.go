package pgqueue

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/code19m/errx"
	"github.com/uptrace/bun"
)

const (
	// tableNameQueueMessages is the name of the main queue messages table.
	tableNameQueueMessages = "queue_messages"

	// migrationTimeout is the maximum time allowed for the database schema migration.
	migrationTimeout = time.Second * 10
)

// generateSchemaSQL generates the schema SQL with the given schema name.
func generateSchemaSQL(schema string) string {
	table := schema + "." + tableNameQueueMessages

	var sql strings.Builder

	writeSection(&sql, createSchema(schema))
	writeSection(&sql, createMessagesTable(table))
	writeSection(&sql, createIndexes(table))
	writeSection(&sql, createTriggers(schema, table))
	writeSection(&sql, createStatsView(schema, table))

	return sql.String()
}

func writeSection(b *strings.Builder, content string) {
	b.WriteString(content)
	b.WriteString("\n")
}

func createSchema(schema string) string {
	return fmt.Sprintf(`
-- Create schema if not exists
CREATE SCHEMA IF NOT EXISTS %s;`, schema)
}

func createMessagesTable(table string) string {
	return fmt.Sprintf(`
-- Main queue messages table
CREATE TABLE IF NOT EXISTS %s (
    id BIGSERIAL PRIMARY KEY,

    -- Routing
    queue_name VARCHAR(255) NOT NULL,
    message_group_id VARCHAR(255),

    -- Content
    payload JSONB NOT NULL,

    -- Timing Control
    scheduled_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    visible_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMPTZ,

    -- Processing
    priority INT NOT NULL DEFAULT 0,
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 3,

    -- Idempotency
    idempotency_key VARCHAR(255) NOT NULL,

    -- Audit
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- DLQ
    dlq_at TIMESTAMPTZ,
    dlq_reason JSONB
);`, table)
}

func createIndexes(table string) string {
	return fmt.Sprintf(`
-- Critical indexes for performance

-- Dequeue index (hot path) - optimized for the main dequeue query
-- Partial index only on active messages (not in DLQ)
CREATE INDEX IF NOT EXISTS idx_queue_messages_dequeue
ON %s (queue_name, priority DESC, visible_at, scheduled_at, id ASC)
WHERE dlq_at IS NULL;

-- Message group index for FIFO ordering
CREATE INDEX IF NOT EXISTS idx_queue_messages_group
ON %s (message_group_id, priority DESC, id ASC)
WHERE message_group_id IS NOT NULL
  AND dlq_at IS NULL;

-- Idempotency index for duplicate detection (unique constraint)
CREATE UNIQUE INDEX IF NOT EXISTS idx_queue_messages_idempotency
ON %s (queue_name, idempotency_key)
WHERE dlq_at IS NULL;

-- Scheduled messages index
CREATE INDEX IF NOT EXISTS idx_queue_messages_scheduled
ON %s (scheduled_at)
WHERE dlq_at IS NULL;

-- DLQ index for querying dead letter queue
CREATE INDEX IF NOT EXISTS idx_queue_messages_dlq
ON %s (queue_name, dlq_at DESC)
WHERE dlq_at IS NOT NULL;`, table, table, table, table, table)
}

func createTriggers(schema, table string) string {
	return fmt.Sprintf(`
-- Update timestamp trigger
CREATE OR REPLACE FUNCTION %s.update_queue_messages_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_queue_messages_updated_at ON %s;
CREATE TRIGGER trigger_queue_messages_updated_at
    BEFORE UPDATE ON %s
    FOR EACH ROW
    EXECUTE FUNCTION %s.update_queue_messages_updated_at();`, schema, table, table, schema)
}

func createStatsView(schema, table string) string {
	return fmt.Sprintf(`
-- Stats view for monitoring
CREATE OR REPLACE VIEW %s.queue_stats AS
SELECT
    queue_name,
    COUNT(*) FILTER (WHERE dlq_at IS NULL) as total,
    COUNT(*) FILTER (WHERE visible_at <= CURRENT_TIMESTAMP
                     AND scheduled_at <= CURRENT_TIMESTAMP
                     AND dlq_at IS NULL) as available,
    COUNT(*) FILTER (WHERE visible_at > CURRENT_TIMESTAMP
                     AND dlq_at IS NULL) as in_flight,
    COUNT(*) FILTER (WHERE scheduled_at > CURRENT_TIMESTAMP
                     AND dlq_at IS NULL) as scheduled,
    COUNT(*) FILTER (WHERE dlq_at IS NOT NULL) as in_dlq,
    MIN(created_at) FILTER (WHERE dlq_at IS NULL) as oldest_message,
    AVG(attempts) FILTER (WHERE dlq_at IS NULL) as avg_attempts,
    PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY attempts) FILTER (WHERE dlq_at IS NULL) as p95_attempts
FROM %s
GROUP BY queue_name;`, schema, table)
}

// migrate creates the queue schema and tables.
func migrate(db *bun.DB, schema string) error {
	ctx, cancel := context.WithTimeout(context.Background(), migrationTimeout)
	defer cancel()

	sql := generateSchemaSQL(schema)
	_, err := db.ExecContext(ctx, sql)
	return errx.Wrap(err)
}
