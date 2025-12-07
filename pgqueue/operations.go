package pgqueue

import (
	"context"
	"time"

	"github.com/code19m/errx"
	"github.com/uptrace/bun"
)

// AckTx acknowledges a message within a transaction, removing it from the queue.
func (q *queue) AckTx(ctx context.Context, tx *bun.Tx, messageID int64) error {
	// Delete the message
	rowsAffected, err := q.deleteMessage(ctx, tx, messageID)
	if err != nil {
		return errx.Wrap(err)
	}

	if rowsAffected == 0 {
		return errx.New("[pgqueue]: no rows affected")
	}

	return nil
}

// NackTx negatively acknowledges a message within a transaction, triggering retry or DLQ.
func (q *queue) NackTx(ctx context.Context, tx *bun.Tx, messageID int64, reason map[string]any) error {
	// Load the message
	msg, err := q.selectMessageByID(ctx, tx, messageID)
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
		_, err = q.updateMessageVisibility(ctx, tx, messageID, newVisibleAt)
		if err != nil {
			return errx.Wrap(err)
		}
	} else {
		// Move to DLQ
		now := time.Now()
		err = q.moveToDLQ(ctx, tx, messageID, now, reason)
		if err != nil {
			return errx.Wrap(err)
		}
	}

	return nil
}

// Purge removes all messages from a queue (excluding DLQ messages).
func (q *queue) Purge(ctx context.Context, queueName string) error {
	return q.deleteQueueMessages(ctx, queueName)
}

// PurgeDLQ removes all DLQ messages from a queue.
func (q *queue) PurgeDLQ(ctx context.Context, queueName string) error {
	return q.deleteDLQMessages(ctx, queueName)
}

// RequeueFromDLQ moves a message from DLQ back to the queue.
func (q *queue) RequeueFromDLQ(ctx context.Context, messageID int64) error {
	// Load the message
	msg, err := q.selectMessageByID(ctx, q.db, messageID)
	if err != nil {
		return errx.Wrap(err)
	}

	// Check if message is in DLQ
	if msg.DLQAt == nil {
		return errx.New("[pgqueue]: message is not in DLQ")
	}

	return q.requeueFromDLQ(ctx, messageID)
}

// Stats returns statistics about a queue.
func (q *queue) Stats(ctx context.Context, queueName string) (*QueueStats, error) {
	return q.getQueueStats(ctx, queueName)
}
