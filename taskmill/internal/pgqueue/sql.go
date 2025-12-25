package pgqueue

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/code19m/errx"
	"github.com/rise-and-shine/pkg/pg"
	"github.com/uptrace/bun"
)

const (
	idempotencyKeyUniqueConstraint = "idx_task_queue_idempotency"
)

// tableName returns the fully qualified table name (schema.task_queue).
func (q *queue) tableName() string {
	return fmt.Sprintf("%s.%s", q.schema, tableNameTaskQueue)
}

// resultsTableName returns the fully qualified task_results table name.
func (q *queue) resultsTableName() string {
	return fmt.Sprintf("%s.%s", q.schema, tableNameTaskResults)
}

// insertTasks inserts multiple tasks in a single batch and returns their IDs.
func (q *queue) insertTasks(ctx context.Context, db bun.IDB, tasks []Task) ([]int64, error) {
	if len(tasks) == 0 {
		return []int64{}, nil
	}

	// Build the VALUES clause with placeholders
	var args []any
	valuesPlaceholders := make([]string, 0, len(tasks))

	for _, task := range tasks {
		valuesPlaceholders = append(valuesPlaceholders, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args,
			task.QueueName,
			task.TaskGroupID,
			task.OperationID,
			task.Meta,
			task.Payload,
			task.ScheduledAt,
			task.VisibleAt,
			task.ExpiresAt,
			task.Priority,
			task.MaxAttempts,
			task.IdempotencyKey,
			task.Attempts,
			task.Ephemeral,
		)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (
			queue_name,
			task_group_id,
			operation_id,
			meta,
			payload,
			scheduled_at,
			visible_at,
			expires_at,
			priority,
			max_attempts,
			idempotency_key,
			attempts,
			ephemeral
		) VALUES %s
		RETURNING id
	`, q.tableName(), strings.Join(valuesPlaceholders, ", "))

	var ids []int64
	err := db.NewRaw(query, args...).Scan(ctx, &ids)
	if pg.ConstraintName(err) == idempotencyKeyUniqueConstraint {
		return nil, errx.Wrap(err, errx.WithCode(CodeDuplicateTask))
	}
	if err != nil {
		return nil, errx.Wrap(err)
	}

	return ids, nil
}

// dequeueTasks retrieves and locks tasks from the queue.
func (q *queue) dequeueTasks(
	ctx context.Context,
	db bun.IDB,
	queueName string,
	taskGroupID *string,
	batchSize int,
	visibilityTimeout time.Duration,
) ([]Task, error) {
	var (
		tasks []Task
	)

	query := fmt.Sprintf(`
		WITH selected AS (
			SELECT id
			FROM %s
			WHERE queue_name = ?
			  AND visible_at <= NOW()
			  AND scheduled_at <= NOW()
			  AND dlq_at IS NULL
			  AND (? IS NULL OR task_group_id = ?)
			ORDER BY priority DESC, id ASC
			LIMIT ?
			FOR UPDATE SKIP LOCKED
		)
		UPDATE %s t
		SET
			visible_at = NOW() + INTERVAL '1 second' * ?,
			attempts = attempts + 1,
			updated_at = NOW()
		FROM selected s
		WHERE t.id = s.id
		RETURNING t.*
	`, q.tableName(), q.tableName())

	_, err := db.NewRaw(query,
		queueName,
		taskGroupID,
		taskGroupIDToAny(taskGroupID),
		batchSize,
		int(visibilityTimeout.Seconds()),
	).Exec(ctx, &tasks)

	return tasks, errx.Wrap(err)
}

// moveToDLQ moves a task to the DLQ.
func (q *queue) moveToDLQ(
	ctx context.Context,
	db bun.IDB,
	taskID int64,
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

	_, err := db.ExecContext(ctx, query, dlqAt, dlqReason, taskID)
	return errx.Wrap(err)
}

// deleteTask deletes a task by ID.
func (q *queue) deleteTask(ctx context.Context, db bun.IDB, taskID int64) (int64, error) {
	query := fmt.Sprintf(`
		DELETE FROM %s
		WHERE id = ?
	`, q.tableName())

	result, err := db.ExecContext(ctx, query, taskID)
	if err != nil {
		return 0, errx.Wrap(err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, errx.Wrap(err)
	}

	return rowsAffected, nil
}

// updateTaskVisibility updates the visibility timeout of a task.
func (q *queue) updateTaskVisibility(
	ctx context.Context,
	db bun.IDB,
	taskID int64,
	visibleAt time.Time,
) (int64, error) {
	query := fmt.Sprintf(`
		UPDATE %s
		SET visible_at = ?,
		    updated_at = NOW()
		WHERE id = ?
	`, q.tableName())

	result, err := db.ExecContext(ctx, query, visibleAt, taskID)
	if err != nil {
		return 0, errx.Wrap(err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, errx.Wrap(err)
	}

	return rowsAffected, nil
}

// selectTaskByID retrieves a task by its ID.
func (q *queue) selectTaskByID(ctx context.Context, db bun.IDB, taskID int64) (*Task, error) {
	query := fmt.Sprintf(`
		SELECT *
		FROM %s
		WHERE id = ?
	`, q.tableName())

	task := new(Task)
	err := db.NewRaw(query, taskID).Scan(ctx, task)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	return task, nil
}

// getQueueStats retrieves statistics for a queue.
func (q *queue) getQueueStats(ctx context.Context, db bun.IDB, queueName string) (*QueueStats, error) {
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
			MIN(created_at) FILTER (WHERE dlq_at IS NULL) as oldest_task,
			AVG(attempts) FILTER (WHERE dlq_at IS NULL) as avg_attempts,
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY attempts) FILTER (WHERE dlq_at IS NULL) as p95_attempts
		FROM %s
		WHERE queue_name = ?
	`, q.tableName())

	stats := &QueueStats{}
	err := db.NewRaw(query, queueName, queueName).Scan(ctx, stats)
	return stats, errx.Wrap(err)
}

// deleteQueueTasks deletes all non-DLQ tasks from a queue.
func (q *queue) deleteQueueTasks(ctx context.Context, db bun.IDB, queueName string) error {
	query := fmt.Sprintf(`
		DELETE FROM %s
		WHERE queue_name = ?
		  AND dlq_at IS NULL
	`, q.tableName())

	_, err := db.ExecContext(ctx, query, queueName)
	return errx.Wrap(err)
}

// deleteDLQTasks deletes all DLQ tasks from a queue.
func (q *queue) deleteDLQTasks(ctx context.Context, db bun.IDB, queueName string) error {
	query := fmt.Sprintf(`
		DELETE FROM %s
		WHERE queue_name = ?
		  AND dlq_at IS NOT NULL
	`, q.tableName())

	_, err := db.ExecContext(ctx, query, queueName)
	return errx.Wrap(err)
}

// requeueTaskFromDLQ moves a task from DLQ back to the queue.
func (q *queue) requeueTaskFromDLQ(ctx context.Context, db bun.IDB, taskID int64) error {
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

	_, err := db.ExecContext(ctx, query, taskID)
	return errx.Wrap(err)
}

// insertTaskResult inserts a completed task into task_results.
func (q *queue) insertTaskResult(ctx context.Context, db bun.IDB, task *Task, completedAt time.Time) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (
			id,
			queue_name,
			task_group_id,
			operation_id,
			meta,
			payload,
			priority,
			attempts,
			max_attempts,
			idempotency_key,
			scheduled_at,
			created_at,
			completed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, q.resultsTableName())

	_, err := db.ExecContext(ctx, query,
		task.ID,
		task.QueueName,
		task.TaskGroupID,
		task.OperationID,
		task.Meta,
		task.Payload,
		task.Priority,
		task.Attempts,
		task.MaxAttempts,
		task.IdempotencyKey,
		task.ScheduledAt,
		task.CreatedAt,
		completedAt,
	)
	return errx.Wrap(err)
}

// listTaskResults queries task_results with optional filters.
func (q *queue) listTaskResults(ctx context.Context, db bun.IDB, params ListResultsParams) ([]TaskResult, error) {
	// Apply defaults
	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	// Build query with optional filters
	var conditions []string
	var args []any

	if params.QueueName != nil {
		conditions = append(conditions, "queue_name = ?")
		args = append(args, *params.QueueName)
	}

	if params.TaskGroupID != nil {
		conditions = append(conditions, "task_group_id = ?")
		args = append(args, *params.TaskGroupID)
	}

	if params.CompletedAfter != nil {
		conditions = append(conditions, "completed_at >= ?")
		args = append(args, *params.CompletedAfter)
	}

	if params.CompletedBefore != nil {
		conditions = append(conditions, "completed_at < ?")
		args = append(args, *params.CompletedBefore)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT *
		FROM %s
		%s
		ORDER BY completed_at DESC
		LIMIT ?
		OFFSET ?
	`, q.resultsTableName(), whereClause)

	args = append(args, limit, params.Offset)

	var results []TaskResult
	_, err := db.NewRaw(query, args...).Exec(ctx, &results)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	return results, nil
}

// cleanupTaskResults deletes old task results.
func (q *queue) cleanupTaskResults(ctx context.Context, db bun.IDB, params CleanupResultsParams) (int64, error) {
	var conditions []string
	var args []any

	// CompletedBefore is required
	conditions = append(conditions, "completed_at < ?")
	args = append(args, params.CompletedBefore)

	if params.QueueName != nil {
		conditions = append(conditions, "queue_name = ?")
		args = append(args, *params.QueueName)
	}

	query := fmt.Sprintf(`
		DELETE FROM %s
		WHERE %s
	`, q.resultsTableName(), strings.Join(conditions, " AND "))

	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, errx.Wrap(err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, errx.Wrap(err)
	}

	return rowsAffected, nil
}
