package pgqueue

import (
	"context"
	"fmt"
	"time"

	"github.com/code19m/errx"
	"github.com/uptrace/bun"
)

// Dequeue retrieves messages from the queue.
func (q *queue) Dequeue(ctx context.Context, queueName string, opts ...DequeueOption) ([]*Message, error) {
	return q.DequeueTx(ctx, nil, queueName, opts...)
}

// DequeueTx retrieves messages from the queue within a transaction.
func (q *queue) DequeueTx(ctx context.Context, tx *bun.Tx, queueName string, opts ...DequeueOption) ([]*Message, error) {
	// Validate required parameters
	if queueName == "" {
		return nil, ErrQueueNameRequired
	}

	// Apply functional options with defaults
	config := dequeueConfig{
		visibilityTimeout: q.defaultVisibilityTimeout,
		batchSize:         1,
	}
	for _, opt := range opts {
		opt(&config)
	}

	// Validate configuration
	if config.batchSize < 1 || config.batchSize > 100 {
		return nil, ErrInvalidBatchSize
	}
	if config.visibilityTimeout <= 0 {
		return nil, ErrInvalidVisibilityTimeout
	}

	db := getDB(q.db, tx)

	// Move expired messages to DLQ before dequeuing
	if err := q.moveExpiredMessagesToDLQ(ctx, db, queueName, config.messageGroupID); err != nil {
		return nil, errx.Wrap(err)
	}

	// Move messages that have reached max attempts to DLQ before dequeuing
	if err := q.moveMaxAttemptsMessagesToDLQ(ctx, db, queueName, config.messageGroupID); err != nil {
		return nil, errx.Wrap(err)
	}

	// For message groups, acquire advisory lock to ensure FIFO ordering
	if config.messageGroupID != nil && *config.messageGroupID != "" {
		lockID := calculateLockID(queueName, *config.messageGroupID)
		_, err := db.ExecContext(ctx, "SELECT pg_advisory_xact_lock(?)", lockID)
		if err != nil {
			return nil, errx.Wrap(err)
		}
	}

	// Build the dequeue query using CTE and UPDATE...RETURNING
	// This ensures atomic select and update using SKIP LOCKED
	query := fmt.Sprintf(`
		WITH selected AS (
			SELECT id
			FROM %s
			WHERE queue_name = ?
			  AND visible_at <= NOW()
			  AND scheduled_at <= NOW()
			  AND dlq_at IS NULL
			  AND (? IS NULL OR message_group_id = ?)
			ORDER BY priority DESC, id ASC
			LIMIT ?
			FOR UPDATE SKIP LOCKED
		)
		UPDATE %s m
		SET
			visible_at = NOW() + INTERVAL '1 second' * ?,
			attempts = attempts + 1,
			updated_at = NOW()
		FROM selected s
		WHERE m.id = s.id
		RETURNING m.*
	`, q.qualifiedTableName(TableNameQueueMessages), q.qualifiedTableName(TableNameQueueMessages))

	var messages []*Message

	// Execute the query with parameters
	messageGroupID := config.messageGroupID
	var messageGroupIDVal any
	if messageGroupID != nil {
		messageGroupIDVal = *messageGroupID
	}

	visibilitySeconds := int(config.visibilityTimeout.Seconds())

	_, err := db.NewRaw(query,
		queueName,
		messageGroupID,
		messageGroupIDVal,
		config.batchSize,
		visibilitySeconds,
	).Exec(ctx, &messages)

	if err != nil {
		return nil, errx.Wrap(err)
	}

	if len(messages) == 0 {
		return nil, ErrNoMessages
	}

	return messages, nil
}

// moveExpiredMessagesToDLQ moves expired messages to the DLQ.
// This is a rare edge case that requires manual intervention by developers.
func (q *queue) moveExpiredMessagesToDLQ(
	ctx context.Context,
	db bun.IDB,
	queueName string,
	messageGroupID *string,
) error {
	now := time.Now()

	dlqReason := `Message expired: The message's expires_at timestamp has been reached before it could be processed. This is a rare condition indicating either:
1. The message was scheduled too far in the future with an expiration time
2. The queue processing is severely delayed or stalled
3. The expiration time was set incorrectly during enqueueing

Action required: Review the message payload and determine if it should be:
- Requeued with a new expiration time (if still relevant)
- Discarded (if no longer applicable)
- Processed manually with appropriate business logic adjustments

This message has been moved to DLQ for manual review and cannot be automatically retried.`

	query := fmt.Sprintf(`
		UPDATE %s
		SET
			dlq_at = ?,
			dlq_reason = ?,
			updated_at = NOW()
		WHERE queue_name = ?
		  AND expires_at IS NOT NULL
		  AND expires_at <= ?
		  AND dlq_at IS NULL
		  AND (? IS NULL OR message_group_id = ?)
	`, q.qualifiedTableName(TableNameQueueMessages))

	var messageGroupIDVal any
	if messageGroupID != nil {
		messageGroupIDVal = *messageGroupID
	}

	_, err := db.ExecContext(ctx, query, now, dlqReason, queueName, now, messageGroupID, messageGroupIDVal)
	return errx.Wrap(err)
}

// moveMaxAttemptsMessagesToDLQ moves messages that have reached max attempts to the DLQ.
// This is a rare edge case where a message's attempts counter already equals or exceeds max_attempts
// before dequeuing, which shouldn't happen in normal operation but requires manual intervention.
func (q *queue) moveMaxAttemptsMessagesToDLQ(
	ctx context.Context,
	db bun.IDB,
	queueName string,
	messageGroupID *string,
) error {
	now := time.Now()

	dlqReason := `Message reached maximum retry attempts before dequeue: The message's attempt counter has already reached or exceeded max_attempts limit. This is an exceptional condition that indicates:
1. A race condition occurred during previous processing
2. The message was previously nacked multiple times and reached the limit
3. The max_attempts value was modified after the message was partially processed
4. Data inconsistency in the queue state

Action required: Investigate why this message reached max attempts before being dequeued:
- Review application logs for processing errors related to this message
- Check if there were concurrent workers causing race conditions
- Verify the message payload is valid and can be processed
- Determine if the business logic should be adjusted
- Consider if max_attempts configuration is appropriate for this queue

This message has been moved to DLQ for manual review. To retry, you must explicitly requeue it from DLQ with updated parameters.`

	query := fmt.Sprintf(`
		UPDATE %s
		SET
			dlq_at = ?,
			dlq_reason = ?,
			updated_at = NOW()
		WHERE queue_name = ?
		  AND attempts >= max_attempts
		  AND dlq_at IS NULL
		  AND (? IS NULL OR message_group_id = ?)
	`, q.qualifiedTableName(TableNameQueueMessages))

	var messageGroupIDVal any
	if messageGroupID != nil {
		messageGroupIDVal = *messageGroupID
	}

	_, err := db.ExecContext(ctx, query, now, dlqReason, queueName, messageGroupID, messageGroupIDVal)
	return errx.Wrap(err)
}
