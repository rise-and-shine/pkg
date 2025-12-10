package pgqueue

import (
	"context"
	"time"

	"github.com/code19m/errx"
	"github.com/uptrace/bun"
)

// Ack acknowledges a message, removing it from the queue.
func (q *queue) Ack(ctx context.Context, db bun.IDB, messageID int64) error {
	// Delete the message
	rowsAffected, err := q.deleteMessage(ctx, db, messageID)
	if err != nil {
		return errx.Wrap(err)
	}

	if rowsAffected == 0 {
		return errx.New("[pgqueue]: no rows affected")
	}

	return nil
}

// Nack negatively acknowledges a message, triggering retry or DLQ.
func (q *queue) Nack(ctx context.Context, db bun.IDB, messageID int64, reason map[string]any) error {
	// Load the message
	msg, err := q.selectMessageByID(ctx, db, messageID)
	if err != nil {
		return errx.Wrap(err)
	}

	// Check if already in DLQ
	if msg.DLQAt != nil {
		return errx.New("[pgqueue]: message is already in dead letter queue")
	}

	// Determine if should retry
	shouldRetry := q.retryStrategy.ShouldRetry(msg.Attempts, msg.MaxAttempts)

	if shouldRetry {
		// Calculate next retry delay
		delay := q.retryStrategy.NextRetryDelay(msg.Attempts)
		newVisibleAt := time.Now().Add(delay)

		// Update message for retry
		_, err = q.updateMessageVisibility(ctx, db, messageID, newVisibleAt)
		if err != nil {
			return errx.Wrap(err)
		}
	} else {
		// Move to DLQ
		now := time.Now()
		err = q.moveToDLQ(ctx, db, messageID, now, reason)
		if err != nil {
			return errx.Wrap(err)
		}
	}

	return nil
}

// Purge removes all messages from a queue (excluding DLQ messages).
func (q *queue) Purge(ctx context.Context, db bun.IDB, queueName string) error {
	return q.deleteQueueMessages(ctx, db, queueName)
}

// PurgeDLQ removes all DLQ messages from a queue.
func (q *queue) PurgeDLQ(ctx context.Context, db bun.IDB, queueName string) error {
	return q.deleteDLQMessages(ctx, db, queueName)
}

// RequeueFromDLQ moves a message from DLQ back to the queue.
func (q *queue) RequeueFromDLQ(ctx context.Context, db bun.IDB, messageID int64) error {
	// Load the message
	msg, err := q.selectMessageByID(ctx, db, messageID)
	if err != nil {
		return errx.Wrap(err)
	}

	// Check if message is in DLQ
	if msg.DLQAt == nil {
		return errx.New("[pgqueue]: message is not in DLQ")
	}

	return q.requeueFromDLQ(ctx, db, messageID)
}

// Stats returns statistics about a queue.
func (q *queue) Stats(ctx context.Context, db bun.IDB, queueName string) (*QueueStats, error) {
	return q.getQueueStats(ctx, db, queueName)
}
