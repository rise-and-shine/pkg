// Package pgqueue provides a production-ready PostgreSQL-based task queue implementation
// with a transaction-first design.
//
// It leverages PostgreSQL's SKIP LOCKED feature for efficient concurrent task processing,
// supports features like task groups (FIFO ordering), priority queues, delayed tasks,
// idempotency, and automatic retries with exponential backoff.
//
// # IMPORTANT: Deadlock Prevention
//
// This package uses PostgreSQL advisory locks for FIFO task group ordering.
// If a worker process crashes while holding an advisory lock, other workers waiting
// to dequeue from the same task group will block indefinitely, causing a deadlock.
//
// To prevent deadlocks, configure your PostgreSQL connection with these timeouts:
//
//   - statement_timeout: Maximum duration for any single statement (e.g., '30s').
//     When exceeded, the statement is aborted and the advisory lock is released.
//
//   - idle_in_transaction_session_timeout: Maximum idle time allowed within a transaction
//     (e.g., '60s'). When exceeded, the transaction is aborted and locks are released.
//
// These timeouts ensure that even if a worker crashes, locks will be automatically
// released within the configured timeout period, preventing indefinite hangs.
//
// Recommended configuration:
//
//	statement_timeout = '30s' to '5m' (depends on your task processing time)
//	idle_in_transaction_session_timeout = '1m' to '10m'
package pgqueue

import (
	"context"

	"github.com/code19m/errx"
	"github.com/uptrace/bun"
)

// Queue defines the interface for queue operations.
type Queue interface {
	// EnqueueBatch adds multiple tasks to the queue.
	EnqueueBatch(ctx context.Context, db bun.IDB, queueName string, tasks []TaskParams) ([]int64, error)

	// Dequeue retrieves tasks from the queue.
	Dequeue(ctx context.Context, db bun.IDB, params DequeueParams) ([]Task, error)

	// Ack acknowledges a task, removing it from the queue.
	Ack(ctx context.Context, db bun.IDB, taskID int64) error

	// Nack negatively acknowledges a task, triggering retry or DLQ.
	Nack(ctx context.Context, db bun.IDB, taskID int64, reason map[string]any) error

	// Purge removes all tasks from a queue (excluding DLQ tasks).
	Purge(ctx context.Context, db bun.IDB, queueName string) error

	// PurgeDLQ removes all DLQ tasks from a queue.
	PurgeDLQ(ctx context.Context, db bun.IDB, queueName string) error

	// RequeueFromDLQ moves a task from DLQ back to the queue.
	RequeueFromDLQ(ctx context.Context, db bun.IDB, taskID int64) error

	// Stats returns statistics about a queue.
	Stats(ctx context.Context, db bun.IDB, queueName string) (*QueueStats, error)

	// ListResults queries completed tasks from task_results with optional filters.
	ListResults(ctx context.Context, db bun.IDB, params ListResultsParams) ([]TaskResult, error)

	// CleanupResults deletes old task results and returns the number of deleted rows.
	CleanupResults(ctx context.Context, db bun.IDB, params CleanupResultsParams) (int64, error)

	// Migrate auto creates the queue schema and tables if they don't exist.
	Migrate(ctx context.Context, db bun.IDB, schema string) error
}

// QueueConfig configures a Queue instance.
type QueueConfig struct {
	// Schema is the PostgreSQL schema name (required).
	// Must be valid identifier: alphanumeric + underscore, 1-63 chars.
	Schema string

	// RetryStrategy determines retry behavior (required).
	// Use NewExponentialBackoffStrategy(), NewFixedDelayStrategy(), or NoRetryStrategy().
	RetryStrategy RetryStrategy
}

// NewQueue creates a new queue instance.
func NewQueue(schema string, retryStrategy RetryStrategy) (Queue, error) {
	if schema == "" {
		return nil, errx.New("[pgqueue]: schema is required")
	}
	if retryStrategy == nil {
		return nil, errx.New("[pgqueue]: retry strategy is required")
	}

	return &queue{
		schema:        schema,
		retryStrategy: retryStrategy,
	}, nil
}

// queue is the concrete implementation of the Queue interface.
type queue struct {
	schema        string
	retryStrategy RetryStrategy
}
