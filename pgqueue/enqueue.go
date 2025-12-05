package pgqueue

import (
	"context"
	"database/sql"
	"time"

	"github.com/code19m/errx"
	"github.com/uptrace/bun"
)

// Enqueue adds a message to the queue.
func (q *queue) Enqueue(ctx context.Context, queueName string, payload map[string]any, idempotencyKey string, opts ...EnqueueOption) (int64, error) {
	return q.EnqueueTx(ctx, nil, queueName, payload, idempotencyKey, opts...)
}

// EnqueueTx adds a message to the queue within a transaction.
func (q *queue) EnqueueTx(ctx context.Context, tx *bun.Tx, queueName string, payload map[string]any, idempotencyKey string, opts ...EnqueueOption) (int64, error) {
	// Validate required parameters
	if queueName == "" {
		return 0, ErrQueueNameRequired
	}
	if len(payload) == 0 {
		return 0, ErrPayloadRequired
	}
	if idempotencyKey == "" {
		return 0, ErrIdempotencyKeyRequired
	}

	// Apply functional options
	config := enqueueConfig{
		maxAttempts: q.defaultMaxAttempts, // Default from queue config
	}
	for _, opt := range opts {
		opt(&config)
	}

	// Validate priority range
	if config.priority < -100 || config.priority > 100 {
		return 0, ErrInvalidPriority
	}

	db := getDB(q.db, tx)

	// Handle deduplication (idempotency)
	var existingID int64
	err := db.NewSelect().
		Model((*Message)(nil)).
		Table(q.qualifiedTableName(TableNameQueueMessages)).
		Column("id").
		Where("queue_name = ?", queueName).
		Where("idempotency_key = ?", idempotencyKey).
		Where("dlq_at IS NULL").
		Scan(ctx, &existingID)

	if err != nil && err != sql.ErrNoRows {
		return 0, errx.Wrap(err)
	}

	// If exists, return the existing ID (idempotent)
	if existingID != 0 {
		return existingID, nil
	}

	// Calculate scheduled_at
	scheduledAt := time.Now()
	if config.scheduledAt != nil {
		scheduledAt = *config.scheduledAt
	} else if config.delay > 0 {
		scheduledAt = time.Now().Add(config.delay)
	}

	// Create the message
	msg := &Message{
		QueueName:      queueName,
		MessageGroupID: config.messageGroupID,
		Payload:        payload,
		ScheduledAt:    scheduledAt,
		VisibleAt:      scheduledAt, // Initially visible at scheduled time
		Priority:       config.priority,
		MaxAttempts:    config.maxAttempts,
		ExpiresAt:      config.expiresAt,
		IdempotencyKey: idempotencyKey,
		Attempts:       0,
	}

	// Insert the message
	_, err = db.NewInsert().
		Model(msg).
		Table(q.qualifiedTableName(TableNameQueueMessages)).
		Exec(ctx)

	if err != nil {
		return 0, errx.Wrap(err)
	}

	return msg.ID, nil
}

// EnqueueBatch adds multiple messages to the queue in a single transaction.
// All messages in the batch will be enqueued to the same queue.
func (q *queue) EnqueueBatch(ctx context.Context, queueName string, messages []BatchMessage) ([]int64, error) {
	if queueName == "" {
		return nil, ErrQueueNameRequired
	}

	if len(messages) == 0 {
		return []int64{}, nil
	}

	var messageIDs []int64

	// Execute in a transaction
	err := q.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		messageIDs = make([]int64, 0, len(messages))

		for _, msg := range messages {
			id, err := q.EnqueueTx(ctx, &tx, queueName, msg.Payload, msg.IdempotencyKey, msg.Options...)
			if err != nil {
				return errx.Wrap(err)
			}
			messageIDs = append(messageIDs, id)
		}

		return nil
	})

	if err != nil {
		return nil, errx.Wrap(err)
	}

	return messageIDs, nil
}
