package pgqueue

import (
	"context"
	"time"

	"github.com/code19m/errx"
	"github.com/uptrace/bun"
)

// DequeueParams contains parameters for dequeueing tasks.
type DequeueParams struct {
	// QueueName identifies the queue (required, non-empty).
	QueueName string

	// TaskGroupID filters to a specific FIFO group (optional).
	// nil = dequeue from any group.
	TaskGroupID *string

	// VisibilityTimeout controls how long task remains invisible (required).
	// Must be > 0.
	VisibilityTimeout time.Duration

	// BatchSize is how many tasks to dequeue (required).
	// Range: 1 to 100.
	BatchSize int
}

// Dequeue retrieves tasks from the queue.
func (q *queue) Dequeue(ctx context.Context, db bun.IDB, params DequeueParams) ([]Task, error) {
	// Validate parameters
	err := validateDequeueParams(params)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	// DEADLOCK RISK: Acquire advisory lock for task group FIFO ordering.
	//
	// Advisory locks are necessary to enforce strict FIFO ordering within task groups.
	// However, if this process crashes while holding the lock, other workers attempting
	// to dequeue from the same task group will block indefinitely.
	//
	// MITIGATION: Ensure the PostgreSQL connection is configured with timeouts:
	//   - statement_timeout: Forces lock release if any statement takes too long
	//   - idle_in_transaction_session_timeout: Forces transaction abort if idle too long
	//
	// These timeouts ensure that even if a worker crashes, the lock will be
	// automatically released within the configured timeout period, preventing deadlocks.
	if params.TaskGroupID != nil && *params.TaskGroupID != "" {
		lockID := calculateLockID(params.QueueName, *params.TaskGroupID)
		_, err = db.ExecContext(ctx, "SELECT pg_advisory_xact_lock(?)", lockID)
		if err != nil {
			return nil, errx.Wrap(err)
		}
	}

	// Dequeue tasks
	tasks, err := q.dequeueTasks(
		ctx,
		db,
		params.QueueName,
		params.TaskGroupID,
		params.BatchSize,
		params.VisibilityTimeout,
	)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	// Check for each task if it has expired or reached max attempts.
	// Filter out tasks that are moved to DLQ so they're not returned to the caller.
	validTasks := make([]Task, 0, len(tasks))

	for _, task := range tasks {
		if task.ExpiresAt != nil && task.ExpiresAt.Before(time.Now()) {
			// Move expired task to DLQ
			err = q.moveToDLQ(ctx, db, task.ID, time.Now(), map[string]any{
				"reason": "task's expires_at timestamp has been reached before it could be processed",
			})
			if err != nil {
				return nil, errx.Wrap(err)
			}
			continue // Don't include in returned tasks
		}

		if task.MaxAttempts > 0 && task.Attempts >= task.MaxAttempts {
			// Move task with max attempts to DLQ
			err = q.moveToDLQ(ctx, db, task.ID, time.Now(), map[string]any{
				"reason": "task's attempt counter has already reached or exceeded max_attempts limit",
			})
			if err != nil {
				return nil, errx.Wrap(err)
			}
			continue // Don't include in returned tasks
		}

		validTasks = append(validTasks, task)
	}

	return validTasks, nil
}
