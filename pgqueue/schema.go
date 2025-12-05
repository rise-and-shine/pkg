package pgqueue

import (
	"context"
	"fmt"

	"github.com/code19m/errx"
	"github.com/uptrace/bun"
)

// generateSchemaSQL generates the schema SQL with the given schema name.
func generateSchemaSQL(schema string) string { //nolint: funlen // can't be divided into smaller functions
	return fmt.Sprintf(`
-- PostgreSQL Queue Schema
-- This schema implements a production-ready message queue using PostgreSQL

-- Create schema if not exists
CREATE SCHEMA IF NOT EXISTS %s;

-- Main queue messages table
CREATE TABLE IF NOT EXISTS %s.%s (
    id BIGSERIAL PRIMARY KEY,

    -- Routing
    queue_name VARCHAR(255) NOT NULL,
    message_group_id VARCHAR(255),

    -- Content
    payload JSONB NOT NULL,

    -- Timing Control
    scheduled_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    visible_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Processing
    priority INT NOT NULL DEFAULT 0,
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 3,

    -- Idempotency
    idempotency_key VARCHAR(255) NOT NULL,

    -- Audit
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Expiration
    expires_at TIMESTAMPTZ,

    -- DLQ
    dlq_at TIMESTAMPTZ,
    dlq_reason TEXT
);

-- Critical indexes for performance

-- Dequeue index (hot path) - optimized for the main dequeue query
-- Partial index only on active messages (not in DLQ)
CREATE INDEX IF NOT EXISTS idx_queue_messages_dequeue
ON %s.%s (queue_name, priority DESC, id ASC)
WHERE visible_at <= CURRENT_TIMESTAMP
  AND scheduled_at <= CURRENT_TIMESTAMP
  AND dlq_at IS NULL;

-- Message group index for FIFO ordering
CREATE INDEX IF NOT EXISTS idx_queue_messages_group
ON %s.%s (message_group_id, priority DESC, id ASC)
WHERE message_group_id IS NOT NULL
  AND dlq_at IS NULL;

-- Idempotency index for duplicate detection
CREATE INDEX IF NOT EXISTS idx_queue_messages_idempotency
ON %s.%s (queue_name, idempotency_key)
WHERE dlq_at IS NULL;

-- Scheduled messages index
CREATE INDEX IF NOT EXISTS idx_queue_messages_scheduled
ON %s.%s (scheduled_at)
WHERE scheduled_at > CURRENT_TIMESTAMP
  AND dlq_at IS NULL;

-- DLQ index for querying dead letter queue
CREATE INDEX IF NOT EXISTS idx_queue_messages_dlq
ON %s.%s (queue_name, dlq_at DESC)
WHERE dlq_at IS NOT NULL;

-- Update timestamp trigger
CREATE OR REPLACE FUNCTION %s.update_queue_messages_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_queue_messages_updated_at
    BEFORE UPDATE ON %s.%s
    FOR EACH ROW
    EXECUTE FUNCTION %s.update_queue_messages_updated_at();

-- Optional: Dead letter queue archive table
-- Use this to keep a permanent record of failed messages
CREATE TABLE IF NOT EXISTS %s.queue_dlq (
    id BIGSERIAL PRIMARY KEY,
    original_id BIGINT,
    queue_name VARCHAR(255) NOT NULL,
    message_group_id VARCHAR(255),
    payload JSONB NOT NULL,
    priority INT NOT NULL,
    attempts INT NOT NULL,
    max_attempts INT NOT NULL,
    idempotency_key VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    dlq_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    dlq_reason TEXT
);

CREATE INDEX IF NOT EXISTS idx_queue_dlq_queue_name
ON %s.queue_dlq (queue_name, dlq_at DESC);

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
FROM %s.%s
GROUP BY queue_name;
`,
		schema,                 // CREATE SCHEMA
		schema,                 // CREATE TABLE schema
		TableNameQueueMessages, // CREATE TABLE table name
		schema,                 // idx_queue_messages_dequeue schema
		TableNameQueueMessages, // idx_queue_messages_dequeue table
		schema,                 // idx_queue_messages_group schema
		TableNameQueueMessages, // idx_queue_messages_group table
		schema,                 // idx_queue_messages_idempotency schema
		TableNameQueueMessages, // idx_queue_messages_idempotency table
		schema,                 // idx_queue_messages_scheduled schema
		TableNameQueueMessages, // idx_queue_messages_scheduled table
		schema,                 // idx_queue_messages_dlq schema
		TableNameQueueMessages, // idx_queue_messages_dlq table
		schema,                 // CREATE FUNCTION
		schema,                 // CREATE TRIGGER table schema
		TableNameQueueMessages, // CREATE TRIGGER table name
		schema,                 // EXECUTE FUNCTION
		schema,                 // CREATE TABLE queue_dlq
		schema,                 // idx_queue_dlq_queue_name
		schema,                 // CREATE VIEW
		schema,                 // FROM schema
		TableNameQueueMessages, // FROM table name
	)
}

// migrate executes the database schema migration.
func migrate(ctx context.Context, db *bun.DB, schema string) error {
	sql := generateSchemaSQL(schema)
	_, err := db.ExecContext(ctx, sql)
	return errx.Wrap(err)
}
