package pgqueue

import (
	"context"
	"time"

	"github.com/code19m/errx"
	"github.com/uptrace/bun"
)

// DequeueParams contains parameters for dequeueing messages.
type DequeueParams struct {
	// QueueName identifies the queue (required, non-empty).
	QueueName string

	// MessageGroupID filters to a specific FIFO group (optional).
	// nil = dequeue from any group.
	MessageGroupID *string

	// VisibilityTimeout controls how long message remains invisible (required).
	// Must be > 0.
	VisibilityTimeout time.Duration

	// BatchSize is how many messages to dequeue (required).
	// Range: 1 to 100.
	BatchSize int
}

// DequeueTx retrieves messages from the queue.
func (q *queue) DequeueTx(ctx context.Context, tx *bun.Tx, params DequeueParams) ([]Message, error) {
	// Validate parameters
	err := validateDequeueParams(params)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	// For message groups, acquire advisory lock to ensure FIFO ordering
	if params.MessageGroupID != nil && *params.MessageGroupID != "" {
		lockID := calculateLockID(params.QueueName, *params.MessageGroupID)
		_, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(?)", lockID)
		if err != nil {
			return nil, errx.Wrap(err)
		}
	}

	// Dequeue messages
	messages, err := q.dequeueMessages(
		ctx,
		tx,
		params.QueueName,
		params.MessageGroupID,
		params.BatchSize,
		params.VisibilityTimeout,
	)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	if len(messages) == 0 {
		return nil, errx.New("[pgqueue]: no messages available", errx.WithCode(CodeNoMessages))
	}

	// Check for each message if it has expired or reached max attempts
	for _, msg := range messages {
		if msg.ExpiresAt != nil && msg.ExpiresAt.Before(time.Now()) {
			// Move expired message to DLQ
			err = q.moveToDLQ(ctx, tx, msg.ID, time.Now(), map[string]any{
				"reason": "message's expires_at timestamp has been reached before it could be processed",
			})
			if err != nil {
				return nil, errx.Wrap(err)
			}
		}

		if msg.MaxAttempts > 0 && msg.Attempts >= msg.MaxAttempts {
			// Move message with max attempts to DLQ
			err = q.moveToDLQ(ctx, tx, msg.ID, time.Now(), map[string]any{
				"reason": "message's attempt counter has already reached or exceeded max_attempts limit",
			})
			if err != nil {
				return nil, errx.Wrap(err)
			}
		}
	}

	return messages, nil
}
