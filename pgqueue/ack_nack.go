package pgqueue

import (
	"context"
	"time"

	"github.com/code19m/errx"
	"github.com/uptrace/bun"
)

// Ack acknowledges a message, removing it from the queue.
func (q *queue) Ack(ctx context.Context, messageID int64) error {
	return q.AckTx(ctx, nil, messageID)
}

// AckTx acknowledges a message within a transaction.
func (q *queue) AckTx(ctx context.Context, tx *bun.Tx, messageID int64) error {
	db := getDB(q.db, tx)

	// Delete the message
	result, err := db.NewDelete().
		Model((*Message)(nil)).
		Table(q.qualifiedTableName(TableNameQueueMessages)).
		Where("id = ?", messageID).
		Exec(ctx)

	if err != nil {
		return errx.Wrap(err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errx.Wrap(err)
	}

	if rowsAffected == 0 {
		return ErrMessageNotFound
	}

	return nil
}

// Nack negatively acknowledges a message, triggering retry or DLQ.
func (q *queue) Nack(ctx context.Context, messageID int64, reason string) error {
	return q.NackTx(ctx, nil, messageID, reason)
}

// NackTx negatively acknowledges a message within a transaction.
func (q *queue) NackTx(ctx context.Context, tx *bun.Tx, messageID int64, reason string) error {
	db := getDB(q.db, tx)

	// Load the message
	msg, err := q.getMessage(ctx, db, messageID)
	if err != nil {
		return errx.Wrap(err)
	}

	// Check if already in DLQ
	if msg.DLQAt != nil {
		return ErrMessageInDLQ
	}

	// Determine if should retry
	shouldRetry := q.retryStrategy.ShouldRetry(msg.Attempts, msg.MaxAttempts)

	if shouldRetry {
		// Calculate next retry delay
		delay := q.retryStrategy.NextRetryDelay(msg.Attempts)
		newVisibleAt := time.Now().Add(delay)

		// Update message for retry
		_, err = db.NewUpdate().
			Model((*Message)(nil)).
			Table(q.qualifiedTableName(TableNameQueueMessages)).
			Set("visible_at = ?", newVisibleAt).
			Where("id = ?", messageID).
			Exec(ctx)

		if err != nil {
			return errx.Wrap(err)
		}
	} else {
		// Move to DLQ
		now := time.Now()

		_, err = db.NewUpdate().
			Model((*Message)(nil)).
			Table(q.qualifiedTableName(TableNameQueueMessages)).
			Set("dlq_at = ?", now).
			Set("dlq_reason = ?", reason).
			Where("id = ?", messageID).
			Exec(ctx)

		if err != nil {
			return errx.Wrap(err)
		}
	}

	return nil
}

// Extend extends the visibility timeout of a message.
func (q *queue) Extend(ctx context.Context, messageID int64, duration time.Duration) error {
	return q.ExtendTx(ctx, nil, messageID, duration)
}

// ExtendTx extends the visibility timeout within a transaction.
func (q *queue) ExtendTx(ctx context.Context, tx *bun.Tx, messageID int64, duration time.Duration) error {
	db := getDB(q.db, tx)

	newVisibleAt := time.Now().Add(duration)

	result, err := db.NewUpdate().
		Model((*Message)(nil)).
		Table(q.qualifiedTableName(TableNameQueueMessages)).
		Set("visible_at = ?", newVisibleAt).
		Where("id = ?", messageID).
		Where("dlq_at IS NULL").
		Exec(ctx)

	if err != nil {
		return errx.Wrap(err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errx.Wrap(err)
	}

	if rowsAffected == 0 {
		return ErrMessageNotFound
	}

	return nil
}
