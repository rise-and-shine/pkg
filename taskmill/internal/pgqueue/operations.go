package pgqueue

import (
	"context"
	"time"

	"github.com/code19m/errx"
	"github.com/uptrace/bun"
)

// Ack acknowledges a task, removing it from the queue.
// If the task is not ephemeral, it will be saved to task_results.
func (q *queue) Ack(ctx context.Context, db bun.IDB, taskID int64) error {
	// Load the task first to get its data
	task, err := q.selectTaskByID(ctx, db, taskID)
	if err != nil {
		return errx.Wrap(err)
	}

	// If not ephemeral, save to task_results
	if !task.Ephemeral {
		completedAt := time.Now()
		err = q.insertTaskResult(ctx, db, task, completedAt)
		if err != nil {
			return errx.Wrap(err)
		}
	}

	// Delete the task from queue
	rowsAffected, err := q.deleteTask(ctx, db, taskID)
	if err != nil {
		return errx.Wrap(err)
	}

	if rowsAffected == 0 {
		return errx.New("[pgqueue]: no rows affected")
	}

	return nil
}

// Nack negatively acknowledges a task, triggering retry or DLQ.
func (q *queue) Nack(ctx context.Context, db bun.IDB, taskID int64, reason map[string]any) error {
	// Load the task
	task, err := q.selectTaskByID(ctx, db, taskID)
	if err != nil {
		return errx.Wrap(err)
	}

	// Check if already in DLQ
	if task.DLQAt != nil {
		return errx.New("[pgqueue]: task is already in dead letter queue")
	}

	// Determine if should retry
	shouldRetry := q.retryStrategy.ShouldRetry(task.Attempts, task.MaxAttempts)

	if shouldRetry {
		// Calculate next retry delay
		delay := q.retryStrategy.NextRetryDelay(task.Attempts)
		newVisibleAt := time.Now().Add(delay)

		// Update task for retry
		_, err = q.updateTaskVisibility(ctx, db, taskID, newVisibleAt)
		if err != nil {
			return errx.Wrap(err)
		}
	} else {
		// Move to DLQ
		now := time.Now()
		err = q.moveToDLQ(ctx, db, taskID, now, reason)
		if err != nil {
			return errx.Wrap(err)
		}
	}

	return nil
}

// Purge removes all tasks from a queue (excluding DLQ tasks).
func (q *queue) Purge(ctx context.Context, db bun.IDB, queueName string) error {
	return q.deleteQueueTasks(ctx, db, queueName)
}

// PurgeDLQ removes all DLQ tasks from a queue.
func (q *queue) PurgeDLQ(ctx context.Context, db bun.IDB, queueName string) error {
	return q.deleteDLQTasks(ctx, db, queueName)
}

// RequeueFromDLQ moves a task from DLQ back to the queue.
func (q *queue) RequeueFromDLQ(ctx context.Context, db bun.IDB, taskID int64) error {
	// Load the task
	task, err := q.selectTaskByID(ctx, db, taskID)
	if err != nil {
		return errx.Wrap(err)
	}

	// Check if task is in DLQ
	if task.DLQAt == nil {
		return errx.New("[pgqueue]: task is not in DLQ")
	}

	return q.requeueTaskFromDLQ(ctx, db, taskID)
}

// Stats returns statistics about a queue.
func (q *queue) Stats(ctx context.Context, db bun.IDB, queueName string) (*QueueStats, error) {
	return q.getQueueStats(ctx, db, queueName)
}

// ListQueues returns all distinct queue names.
func (q *queue) ListQueues(ctx context.Context, db bun.IDB) ([]string, error) {
	return q.listQueues(ctx, db)
}

// ListResults queries completed tasks from task_results with optional filters.
func (q *queue) ListResults(ctx context.Context, db bun.IDB, params ListResultsParams) ([]TaskResult, error) {
	return q.listTaskResults(ctx, db, params)
}

// CleanupResults deletes old task results and returns the number of deleted rows.
func (q *queue) CleanupResults(ctx context.Context, db bun.IDB, params CleanupResultsParams) (int64, error) {
	if params.CompletedBefore.IsZero() {
		return 0, errx.New("[pgqueue]: completed_before is required for cleanup")
	}
	return q.cleanupTaskResults(ctx, db, params)
}

// ListDLQTasks queries tasks in the dead letter queue with optional filters.
func (q *queue) ListDLQTasks(ctx context.Context, db bun.IDB, params ListDLQTasksParams) ([]DLQTask, error) {
	return q.listDLQTasks(ctx, db, params)
}

// Schedule operations

// UpsertSchedule inserts or updates a schedule.
func (q *queue) UpsertSchedule(ctx context.Context, db bun.IDB, schedule TaskSchedule) error {
	return q.upsertSchedule(ctx, db, schedule)
}

// DeleteSchedulesNotIn deletes schedules not in the provided operation IDs.
func (q *queue) DeleteSchedulesNotIn(ctx context.Context, db bun.IDB, operationIDs []string) (int64, error) {
	return q.deleteSchedulesNotIn(ctx, db, operationIDs)
}

// ClaimDueSchedule claims a single due schedule using FOR UPDATE SKIP LOCKED.
func (q *queue) ClaimDueSchedule(ctx context.Context, db bun.IDB) (*TaskSchedule, error) {
	return q.claimDueSchedule(ctx, db)
}

// UpdateScheduleSuccess updates a schedule after successful execution.
func (q *queue) UpdateScheduleSuccess(ctx context.Context, db bun.IDB, id int64, nextRunAt time.Time) error {
	return q.updateScheduleSuccess(ctx, db, id, nextRunAt)
}

// UpdateScheduleFailure updates a schedule after failed execution.
func (q *queue) UpdateScheduleFailure(
	ctx context.Context,
	db bun.IDB,
	id int64,
	nextRunAt time.Time,
	errMsg string,
) error {
	return q.updateScheduleFailure(ctx, db, id, nextRunAt, errMsg)
}

// ListSchedules returns schedules, optionally filtered by queue name.
func (q *queue) ListSchedules(ctx context.Context, db bun.IDB, queueName *string) ([]TaskSchedule, error) {
	return q.listSchedules(ctx, db, queueName)
}

// GetScheduleByOperationID returns a schedule by its operation ID.
func (q *queue) GetScheduleByOperationID(ctx context.Context, db bun.IDB, operationID string) (*TaskSchedule, error) {
	return q.getScheduleByOperationID(ctx, db, operationID)
}
