// Package pgqueue provides a production-ready PostgreSQL-based message queue implementation
// with a transaction-first design.
//
// It leverages PostgreSQL's SKIP LOCKED feature for efficient concurrent message processing,
// supports features like message groups (FIFO ordering), priority queues, delayed messages,
// idempotency, and automatic retries with exponential backoff.
//
// # IMPORTANT: Deadlock Prevention
//
// This package uses PostgreSQL advisory locks for FIFO message group ordering.
// If a worker process crashes while holding an advisory lock, other workers waiting
// to dequeue from the same message group will block indefinitely, causing a deadlock.
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
//	statement_timeout = '30s' to '5m' (depends on your message processing time)
//	idle_in_transaction_session_timeout = '1m' to '10m'
package pgqueue

import (
	"context"

	"github.com/code19m/errx"
	"github.com/uptrace/bun"
)

// QueueConfig configures a Queue instance.
type QueueConfig struct {
	// DB is the bun DB instance (required).
	DB *bun.DB

	// Schema is the PostgreSQL schema name (required).
	// Must be valid identifier: alphanumeric + underscore, 1-63 chars.
	Schema string

	// RetryStrategy determines retry behavior (required).
	// Use NewExponentialBackoffStrategy(), NewFixedDelayStrategy(), or NoRetryStrategy().
	RetryStrategy RetryStrategy

	// AutoMigrate indicates if the queue schema should be created if it doesn't exist.
	AutoMigrate bool
}

// Queue defines the interface for queue operations with transaction-first design.
type Queue interface {
	// EnqueueBatchTx adds multiple messages to the queue.
	EnqueueBatchTx(ctx context.Context, tx *bun.Tx, queueName string, messages []SingleMessage) ([]int64, error)

	// DequeueTx retrieves messages from the queue.
	DequeueTx(ctx context.Context, tx *bun.Tx, params DequeueParams) ([]Message, error)

	// AckTx acknowledges a message within a transaction, removing it from the queue.
	AckTx(ctx context.Context, tx *bun.Tx, messageID int64) error

	// NackTx negatively acknowledges a message within a transaction, triggering retry or DLQ.
	NackTx(ctx context.Context, tx *bun.Tx, messageID int64, reason map[string]any) error

	// Purge removes all messages from a queue (excluding DLQ messages).
	Purge(ctx context.Context, queueName string) error

	// PurgeDLQ removes all DLQ messages from a queue.
	PurgeDLQ(ctx context.Context, queueName string) error

	// RequeueFromDLQ moves a message from DLQ back to the queue.
	RequeueFromDLQ(ctx context.Context, messageID int64) error

	// Stats returns statistics about a queue.
	Stats(ctx context.Context, queueName string) (*QueueStats, error)
}

// NewQueue creates a new queue instance.
func NewQueue(config *QueueConfig) (Queue, error) {
	err := validateQueueConfig(config)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	if config.AutoMigrate {
		err = migrate(config.DB, config.Schema)
		if err != nil {
			return nil, errx.Wrap(err)
		}
	}

	return &queue{
		db:            config.DB,
		schema:        config.Schema,
		retryStrategy: config.RetryStrategy,
	}, nil
}

// queue is the concrete implementation of the Queue interface.
type queue struct {
	db            *bun.DB
	schema        string
	retryStrategy RetryStrategy
}
