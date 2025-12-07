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

	// DEADLOCK RISK: Acquire advisory lock for message group FIFO ordering.
	//
	// Advisory locks are necessary to enforce strict FIFO ordering within message groups.
	// However, if this process crashes while holding the lock, other workers attempting
	// to dequeue from the same message group will block indefinitely.
	//
	// MITIGATION: Ensure the PostgreSQL connection is configured with timeouts:
	//   - statement_timeout: Forces lock release if any statement takes too long
	//   - idle_in_transaction_session_timeout: Forces transaction abort if idle too long
	//
	// These timeouts ensure that even if a worker crashes, the lock will be
	// automatically released within the configured timeout period, preventing deadlocks.
	// See the pgqueue package documentation for recommended timeout values.
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
