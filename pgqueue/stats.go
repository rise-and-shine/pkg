package pgqueue

import (
	"context"
	"fmt"

	"github.com/code19m/errx"
)

// Stats returns statistics about a queue.
func (q *queue) Stats(ctx context.Context, queueName string) (*QueueStats, error) {
	if queueName == "" {
		return nil, ErrQueueNameRequired
	}

	stats := &QueueStats{}

	// Query the stats using custom query
	err := q.db.NewRaw(fmt.Sprintf(`
		SELECT
			? as queue_name,
			COUNT(*) FILTER (WHERE dlq_at IS NULL) as total,
			COUNT(*) FILTER (WHERE visible_at <= NOW()
							 AND scheduled_at <= NOW()
							 AND dlq_at IS NULL) as available,
			COUNT(*) FILTER (WHERE visible_at > NOW()
							 AND dlq_at IS NULL) as in_flight,
			COUNT(*) FILTER (WHERE scheduled_at > NOW()
							 AND dlq_at IS NULL) as scheduled,
			COUNT(*) FILTER (WHERE dlq_at IS NOT NULL) as in_dlq,
			MIN(created_at) FILTER (WHERE dlq_at IS NULL) as oldest_message,
			AVG(attempts) FILTER (WHERE dlq_at IS NULL) as avg_attempts,
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY attempts) FILTER (WHERE dlq_at IS NULL) as p95_attempts
		FROM %s
		WHERE queue_name = ?
	`, q.qualifiedTableName(TableNameQueueMessages)), queueName, queueName).Scan(ctx, stats)

	if err != nil {
		return nil, errx.Wrap(err)
	}

	return stats, nil
}

// Purge removes all messages from a queue (excluding DLQ messages).
func (q *queue) Purge(ctx context.Context, queueName string) error {
	if queueName == "" {
		return ErrQueueNameRequired
	}

	_, err := q.db.NewDelete().
		Model((*Message)(nil)).
		Table(q.qualifiedTableName(TableNameQueueMessages)).
		Where("queue_name = ?", queueName).
		Where("dlq_at IS NULL").
		Exec(ctx)

	if err != nil {
		return errx.Wrap(err)
	}

	return nil
}

// PurgeDLQ removes all DLQ messages from a queue.
func (q *queue) PurgeDLQ(ctx context.Context, queueName string) error {
	if queueName == "" {
		return ErrQueueNameRequired
	}

	_, err := q.db.NewDelete().
		Model((*Message)(nil)).
		Table(q.qualifiedTableName(TableNameQueueMessages)).
		Where("queue_name = ?", queueName).
		Where("dlq_at IS NOT NULL").
		Exec(ctx)

	if err != nil {
		return errx.Wrap(err)
	}

	return nil
}

// RequeueFromDLQ moves a message from DLQ back to the queue.
func (q *queue) RequeueFromDLQ(ctx context.Context, messageID int64) error {
	// Load the message
	msg, err := q.getMessage(ctx, q.db, messageID)
	if err != nil {
		return errx.Wrap(err)
	}

	// Check if message is in DLQ
	if msg.DLQAt == nil {
		return errx.New("message is not in DLQ", errx.WithDetails(errx.D{
			"messageID": messageID,
		}))
	}

	// Reset message state
	_, err = q.db.NewUpdate().
		Model((*Message)(nil)).
		Table(q.qualifiedTableName(TableNameQueueMessages)).
		Set("dlq_at = NULL").
		Set("dlq_reason = NULL").
		Set("attempts = 0").
		Set("visible_at = NOW()").
		Set("scheduled_at = NOW()").
		Where("id = ?", messageID).
		Exec(ctx)

	if err != nil {
		return errx.Wrap(err)
	}

	return nil
}
