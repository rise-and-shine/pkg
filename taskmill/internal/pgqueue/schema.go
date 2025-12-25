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
	// tableNameTaskQueue is the name of the main task queue table.
	tableNameTaskQueue = "task_queue"

	// tableNameTaskResults is the name of the task results table.
	tableNameTaskResults = "task_results"

	// tableNameTaskSchedules is the name of the task schedules table.
	tableNameTaskSchedules = "task_schedules"

	// migrationTimeout is the maximum time allowed for the database schema migration.
	migrationTimeout = time.Second * 10
)

// generateSchemaSQL generates the schema SQL with the given schema name.
func generateSchemaSQL(schema string) string {
	taskQueueTable := schema + "." + tableNameTaskQueue
	taskResultsTable := schema + "." + tableNameTaskResults
	taskSchedulesTable := schema + "." + tableNameTaskSchedules

	var sql strings.Builder

	writeSection(&sql, createSchema(schema))
	writeSection(&sql, createTaskTable(taskQueueTable))
	writeSection(&sql, createTaskResultsTable(taskResultsTable))
	writeSection(&sql, createTaskSchedulesTable(taskSchedulesTable))
	writeSection(&sql, createIndexes(taskQueueTable))
	writeSection(&sql, createTaskResultsIndexes(taskResultsTable))
	writeSection(&sql, createTaskSchedulesIndexes(taskSchedulesTable))
	writeSection(&sql, createTriggers(schema, taskQueueTable))
	writeSection(&sql, createStatsView(schema, taskQueueTable))

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

func createTaskTable(table string) string {
	return fmt.Sprintf(`
-- Main task queue table
CREATE TABLE IF NOT EXISTS %s (
    id BIGSERIAL PRIMARY KEY,

    -- Routing
    queue_name VARCHAR(255) NOT NULL,
    task_group_id VARCHAR(255),
    operation_id VARCHAR(255) NOT NULL,

    -- Content
    meta JSONB,
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
    dlq_reason JSONB,

    -- Ephemeral flag
    ephemeral BOOLEAN NOT NULL DEFAULT FALSE
);`, table)
}

func createTaskResultsTable(table string) string {
	return fmt.Sprintf(`
-- Task results table for completed tasks
CREATE TABLE IF NOT EXISTS %s (
    id BIGINT PRIMARY KEY,

    -- Routing
    queue_name VARCHAR(255) NOT NULL,
    task_group_id VARCHAR(255),
    operation_id VARCHAR(255) NOT NULL,

    -- Content
    meta JSONB,
    payload JSONB NOT NULL,

    -- Processing
    priority INT NOT NULL,
    attempts INT NOT NULL,
    max_attempts INT NOT NULL,

    -- Idempotency
    idempotency_key VARCHAR(255) NOT NULL,

    -- Timing
    scheduled_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);`, table)
}

func createTaskSchedulesTable(table string) string {
	return fmt.Sprintf(`
-- Task schedules table for cron-based scheduling
CREATE TABLE IF NOT EXISTS %s (
    id BIGSERIAL PRIMARY KEY,

    -- Identity
    operation_id VARCHAR(255) NOT NULL UNIQUE,
    queue_name VARCHAR(255) NOT NULL,
    cron_pattern VARCHAR(100) NOT NULL,

    -- State
    next_run_at TIMESTAMPTZ NOT NULL,

    -- Execution tracking
    last_run_at TIMESTAMPTZ,
    last_run_status VARCHAR(20),
    last_error TEXT,
    run_count BIGINT NOT NULL DEFAULT 0,

    -- Audit
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);`, table)
}

func createIndexes(table string) string {
	return fmt.Sprintf(`
-- Critical indexes for performance

-- Dequeue index (hot path) - optimized for the main dequeue query
-- Partial index only on active tasks (not in DLQ)
CREATE INDEX IF NOT EXISTS idx_task_queue_dequeue
ON %s (queue_name, priority DESC, visible_at, scheduled_at, id ASC)
WHERE dlq_at IS NULL;

-- Task group index for FIFO ordering
CREATE INDEX IF NOT EXISTS idx_task_queue_group
ON %s (task_group_id, priority DESC, id ASC)
WHERE task_group_id IS NOT NULL
  AND dlq_at IS NULL;

-- Idempotency index for duplicate detection (unique constraint)
CREATE UNIQUE INDEX IF NOT EXISTS idx_task_queue_idempotency
ON %s (queue_name, idempotency_key)
WHERE dlq_at IS NULL;

-- Scheduled tasks index
CREATE INDEX IF NOT EXISTS idx_task_queue_scheduled
ON %s (scheduled_at)
WHERE dlq_at IS NULL;

-- DLQ index for querying dead letter queue
CREATE INDEX IF NOT EXISTS idx_task_queue_dlq
ON %s (queue_name, dlq_at DESC)
WHERE dlq_at IS NOT NULL;

-- Operation ID index for filtering by task type
CREATE INDEX IF NOT EXISTS idx_task_queue_operation
ON %s (queue_name, operation_id)
WHERE dlq_at IS NULL;`, table, table, table, table, table, table)
}

func createTaskResultsIndexes(table string) string {
	return fmt.Sprintf(`
-- Indexes for task_results table

-- Index for querying results by queue and completion time
CREATE INDEX IF NOT EXISTS idx_task_results_queue_completed
ON %s (queue_name, completed_at DESC);

-- Index for cleanup operations
CREATE INDEX IF NOT EXISTS idx_task_results_completed
ON %s (completed_at);

-- Index for querying by idempotency key
CREATE INDEX IF NOT EXISTS idx_task_results_idempotency
ON %s (queue_name, idempotency_key);

-- Index for querying by operation ID
CREATE INDEX IF NOT EXISTS idx_task_results_operation
ON %s (queue_name, operation_id, completed_at DESC);`, table, table, table, table)
}

func createTaskSchedulesIndexes(table string) string {
	return fmt.Sprintf(`
-- Index for finding due schedules
CREATE INDEX IF NOT EXISTS idx_task_schedules_due
ON %s (next_run_at);

-- Index for querying by queue
CREATE INDEX IF NOT EXISTS idx_task_schedules_queue
ON %s (queue_name);`, table, table)
}

func createTriggers(schema, table string) string {
	return fmt.Sprintf(`
-- Update timestamp trigger
CREATE OR REPLACE FUNCTION %s.update_task_queue_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_task_queue_updated_at ON %s;
CREATE TRIGGER trigger_task_queue_updated_at
    BEFORE UPDATE ON %s
    FOR EACH ROW
    EXECUTE FUNCTION %s.update_task_queue_updated_at();`, schema, table, table, schema)
}

func createStatsView(schema, table string) string {
	return fmt.Sprintf(`
-- Stats view for monitoring
CREATE OR REPLACE VIEW %s.task_queue_stats AS
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
    MIN(created_at) FILTER (WHERE dlq_at IS NULL) as oldest_task,
    AVG(attempts) FILTER (WHERE dlq_at IS NULL) as avg_attempts,
    PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY attempts) FILTER (WHERE dlq_at IS NULL) as p95_attempts
FROM %s
GROUP BY queue_name;`, schema, table)
}

// Migrate creates the queue schema and tables.
func (q *queue) Migrate(ctx context.Context, db bun.IDB, schema string) error {
	ctx, cancel := context.WithTimeout(ctx, migrationTimeout)
	defer cancel()

	sql := generateSchemaSQL(schema)
	_, err := db.ExecContext(ctx, sql)
	return errx.Wrap(err)
}
