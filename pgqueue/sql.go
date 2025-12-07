package pgqueue

import (
	"context"
	"fmt"
	"time"

	"github.com/code19m/errx"
	"github.com/rise-and-shine/pkg/pg"
	"github.com/uptrace/bun"
)

const (
	idempotencyKeyUniqueConstraint = "idx_queue_messages_idempotency"
)

// tableName returns the fully qualified table name (schema.table).
func (q *queue) tableName() string {
	return fmt.Sprintf("%s.%s", q.schema, tableNameQueueMessages)
}

// insertMessage inserts a new message and returns its ID.
func (q *queue) insertMessage(ctx context.Context, db bun.IDB, msg *Message) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (
			queue_name, 
			message_group_id, 
			payload, 
			scheduled_at, 
			visible_at,
			expires_at, 
			priority, 
			max_attempts, 
			idempotency_key, 
			attempts
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		) RETURNING id
	`, q.tableName())

	err := db.NewRaw(query,
		msg.QueueName,
		msg.MessageGroupID,
		msg.Payload,
		msg.ScheduledAt,
		msg.VisibleAt,
		msg.ExpiresAt,
		msg.Priority,
		msg.MaxAttempts,
		msg.IdempotencyKey,
		msg.Attempts,
	).Scan(ctx, &msg.ID)

	if pg.ConstraintName(err) == idempotencyKeyUniqueConstraint {
		return errx.Wrap(err, errx.WithCode(CodeDuplicateMessage))
	}

	return errx.Wrap(err)
}

// dequeueMessages retrieves and locks messages from the queue.
func (q *queue) dequeueMessages(
	ctx context.Context,
	db bun.IDB,
	queueName string,
	messageGroupID *string,
	batchSize int,
	visibilityTimeout time.Duration,
) ([]Message, error) {
	var (
		messages []Message
	)

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
	`, q.tableName(), q.tableName())

	_, err := db.NewRaw(query,
		queueName,
		messageGroupID,
		messageGroupIDToAny(messageGroupID),
		batchSize,
		int(visibilityTimeout.Seconds()),
	).Exec(ctx, &messages)

	return messages, errx.Wrap(err)
}

// moveToDLQ moves a message to the DLQ.
func (q *queue) moveToDLQ(
	ctx context.Context,
	db bun.IDB,
	messageID int64,
	dlqAt time.Time,
	dlqReason map[string]any,
) error {
	query := fmt.Sprintf(`
		UPDATE %s
		SET
			dlq_at = ?,
			dlq_reason = ?,
			updated_at = NOW()
		WHERE id = ?
	`, q.tableName())

	_, err := db.ExecContext(ctx, query, dlqAt, dlqReason, messageID)
	return errx.Wrap(err)
}

// deleteMessage deletes a message by ID.
func (q *queue) deleteMessage(ctx context.Context, db bun.IDB, messageID int64) (int64, error) {
	query := fmt.Sprintf(`
		DELETE FROM %s
		WHERE id = ?
	`, q.tableName())

	result, err := db.ExecContext(ctx, query, messageID)
	if err != nil {
		return 0, errx.Wrap(err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, errx.Wrap(err)
	}

	return rowsAffected, nil
}

// updateMessageVisibility updates the visibility timeout of a message.
func (q *queue) updateMessageVisibility(
	ctx context.Context,
	db bun.IDB,
	messageID int64,
	visibleAt time.Time,
) (int64, error) {
	query := fmt.Sprintf(`
		UPDATE %s
		SET visible_at = ?,
		    updated_at = NOW()
		WHERE id = ?
	`, q.tableName())

	result, err := db.ExecContext(ctx, query, visibleAt, messageID)
	if err != nil {
		return 0, errx.Wrap(err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, errx.Wrap(err)
	}

	return rowsAffected, nil
}

// selectMessageByID retrieves a message by its ID.
func (q *queue) selectMessageByID(ctx context.Context, db bun.IDB, messageID int64) (*Message, error) {
	query := fmt.Sprintf(`
		SELECT *
		FROM %s
		WHERE id = ?
	`, q.tableName())

	msg := new(Message)
	err := db.NewRaw(query, messageID).Scan(ctx, msg)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	return msg, nil
}

// getQueueStats retrieves statistics for a queue.
func (q *queue) getQueueStats(ctx context.Context, queueName string) (*QueueStats, error) {
	query := fmt.Sprintf(`
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
	`, q.tableName())

	stats := &QueueStats{}
	err := q.db.NewRaw(query, queueName, queueName).Scan(ctx, stats)
	return stats, errx.Wrap(err)
}

// deleteQueueMessages deletes all non-DLQ messages from a queue.
func (q *queue) deleteQueueMessages(ctx context.Context, queueName string) error {
	query := fmt.Sprintf(`
		DELETE FROM %s
		WHERE queue_name = ?
		  AND dlq_at IS NULL
	`, q.tableName())

	_, err := q.db.ExecContext(ctx, query, queueName)
	return errx.Wrap(err)
}

// deleteDLQMessages deletes all DLQ messages from a queue.
func (q *queue) deleteDLQMessages(ctx context.Context, queueName string) error {
	query := fmt.Sprintf(`
		DELETE FROM %s
		WHERE queue_name = ?
		  AND dlq_at IS NOT NULL
	`, q.tableName())

	_, err := q.db.ExecContext(ctx, query, queueName)
	return errx.Wrap(err)
}

// requeueFromDLQ moves a message from DLQ back to the queue.
func (q *queue) requeueFromDLQ(ctx context.Context, messageID int64) error {
	query := fmt.Sprintf(`
		UPDATE %s
		SET
			dlq_at = NULL,
			dlq_reason = NULL,
			attempts = 0,
			visible_at = NOW(),
			scheduled_at = NOW(),
			updated_at = NOW()
		WHERE id = ?
	`, q.tableName())

	_, err := q.db.ExecContext(ctx, query, messageID)
	return errx.Wrap(err)
}
