// Package pgqueue provides a production-ready PostgreSQL-based message queue implementation.
//
// It leverages PostgreSQL's SKIP LOCKED feature for efficient concurrent message processing,
// supports features like message groups (FIFO ordering), priority queues, delayed messages,
// idempotency, and automatic retries with exponential backoff.
package pgqueue

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/code19m/errx"
	"github.com/uptrace/bun"
)

// Queue defines the interface for queue operations.
type Queue interface {
	// Enqueue adds a message to the queue.
	Enqueue(ctx context.Context, queueName string, payload map[string]any, idempotencyKey string, opts ...EnqueueOption) (int64, error)

	// EnqueueTx adds a message to the queue within a transaction.
	EnqueueTx(ctx context.Context, tx *bun.Tx, queueName string, payload map[string]any, idempotencyKey string, opts ...EnqueueOption) (int64, error)

	// EnqueueBatch adds multiple messages to the queue in a single transaction.
	EnqueueBatch(ctx context.Context, queueName string, messages []BatchMessage) ([]int64, error)

	// Dequeue retrieves messages from the queue.
	Dequeue(ctx context.Context, queueName string, opts ...DequeueOption) ([]*Message, error)

	// DequeueTx retrieves messages from the queue within a transaction.
	DequeueTx(ctx context.Context, tx *bun.Tx, queueName string, opts ...DequeueOption) ([]*Message, error)

	// Ack acknowledges a message, removing it from the queue.
	Ack(ctx context.Context, messageID int64) error

	// AckTx acknowledges a message within a transaction.
	AckTx(ctx context.Context, tx *bun.Tx, messageID int64) error

	// Nack negatively acknowledges a message, triggering retry or DLQ.
	Nack(ctx context.Context, messageID int64, reason string) error

	// NackTx negatively acknowledges a message within a transaction.
	NackTx(ctx context.Context, tx *bun.Tx, messageID int64, reason string) error

	// Extend extends the visibility timeout of a message.
	Extend(ctx context.Context, messageID int64, duration time.Duration) error

	// ExtendTx extends the visibility timeout within a transaction.
	ExtendTx(ctx context.Context, tx *bun.Tx, messageID int64, duration time.Duration) error

	// Stats returns statistics about a queue.
	Stats(ctx context.Context, queueName string) (*QueueStats, error)

	// Purge removes all messages from a queue.
	Purge(ctx context.Context, queueName string) error

	// PurgeDLQ removes all DLQ messages from a queue.
	PurgeDLQ(ctx context.Context, queueName string) error

	// RequeueFromDLQ moves a message from DLQ back to the queue.
	RequeueFromDLQ(ctx context.Context, messageID int64) error
}

// New creates a new queue instance with functional options.
func New(db *bun.DB, opts ...Option) (Queue, error) {
	if db == nil {
		return nil, errx.New("database connection is required", errx.WithDetails(errx.D{
			"field": "db",
		}))
	}

	// Apply options
	options := defaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	// Run migrations if enabled
	if options.autoMigrate {
		if err := migrate(context.Background(), db, options.schema); err != nil {
			return nil, errx.Wrap(err)
		}
	}

	return &queue{
		db:                       db,
		schema:                   options.schema,
		retryStrategy:            options.retryStrategy,
		defaultVisibilityTimeout: options.defaultVisibilityTimeout,
		defaultMaxAttempts:       options.defaultMaxAttempts,
	}, nil
}

// queue is the concrete implementation of the Queue interface.
type queue struct {
	db                       *bun.DB
	schema                   string
	retryStrategy            RetryStrategy
	defaultVisibilityTimeout time.Duration
	defaultMaxAttempts       int
}

// getDB returns either the transaction or the default database.
func getDB(db *bun.DB, tx *bun.Tx) bun.IDB {
	if tx != nil {
		return tx
	}
	return db
}

// calculateLockID generates a lock ID for message group advisory locking.
func calculateLockID(queueName, messageGroupID string) int64 {
	// Simple hash function using FNV-1a algorithm
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)

	hash := uint64(offset64)
	data := queueName + ":" + messageGroupID

	for i := 0; i < len(data); i++ {
		hash ^= uint64(data[i])
		hash *= prime64
	}

	return int64(hash)
}

// validateEnqueueOptions validates enqueue options.
func (q *queue) validateEnqueueOptions(opts EnqueueOptions) error {
	if opts.QueueName == "" {
		return ErrQueueNameRequired
	}

	if len(opts.Payload) == 0 {
		return ErrPayloadRequired
	}

	if opts.IdempotencyKey == "" {
		return ErrIdempotencyKeyRequired
	}

	if opts.Priority < -100 || opts.Priority > 100 {
		return ErrInvalidPriority
	}

	return nil
}

// validateDequeueOptions validates dequeue options.
func (q *queue) validateDequeueOptions(opts DequeueOptions) error {
	if opts.QueueName == "" {
		return ErrQueueNameRequired
	}

	if opts.BatchSize < 1 || opts.BatchSize > 100 {
		return ErrInvalidBatchSize
	}

	if opts.VisibilityTimeout <= 0 {
		return ErrInvalidVisibilityTimeout
	}

	return nil
}

// applyDefaults applies default values to enqueue options.
func (q *queue) applyDefaults(opts *EnqueueOptions) {
	if opts.MaxAttempts == 0 {
		opts.MaxAttempts = q.defaultMaxAttempts
	}
}

// applyDequeueDefaults applies default values to dequeue options.
func (q *queue) applyDequeueDefaults(opts *DequeueOptions) {
	if opts.BatchSize == 0 {
		opts.BatchSize = 1
	}

	if opts.VisibilityTimeout == 0 {
		opts.VisibilityTimeout = q.defaultVisibilityTimeout
	}
}

// getMessage retrieves a message by ID.
func (q *queue) getMessage(ctx context.Context, db bun.IDB, messageID int64) (*Message, error) {
	msg := new(Message)
	err := db.NewSelect().
		Model(msg).
		Table(q.qualifiedTableName(TableNameQueueMessages)).
		Where("id = ?", messageID).
		Scan(ctx)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrMessageNotFound
		}
		return nil, errx.Wrap(err)
	}

	return msg, nil
}

// qualifiedTableName returns the fully qualified table name (schema.table).
func (q *queue) qualifiedTableName(table string) string {
	return fmt.Sprintf("%s.%s", q.schema, table)
}
