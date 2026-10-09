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

// Lease names one pickup of a task: the attempt count and the lease end the
// dequeue returned for it. Take both from the Task as Dequeue returned it, never
// from a clock: VisibleAt is compared with the stored value exactly, to the
// microsecond. The count alone is not enough, because a requeue from the DLQ
// resets it, so an old run and a newer one can hold the same count.
type Lease struct {
	Attempts  int
	VisibleAt time.Time
}

// LeaseOf returns the lease of a task as Dequeue returned it.
func LeaseOf(task Task) Lease {
	return Lease{Attempts: task.Attempts, VisibleAt: task.VisibleAt}
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
		// Cast to int64: pg_advisory_xact_lock takes bigint, not numeric.
		// The uint64 hash wraps around to negative values for large hashes, but
		// this is fine — the lock ID is still unique per (queue, group) pair.
		lockID := int64(calculateLockID(params.QueueName, *params.TaskGroupID)) //nolint:gosec // wraps, see above
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

	// Check for each task if it has expired or used up its attempts.
	// Filter out tasks that are moved to DLQ so they're not returned to the caller.
	validTasks := make([]Task, 0, len(tasks))

	for _, task := range tasks {
		if task.ExpiresAt != nil && task.ExpiresAt.Before(time.Now()) {
			// Move expired task to DLQ
			err = q.moveToDLQ(ctx, db, task.ID, time.Now(), map[string]any{
				"code":   CodeTaskExpired,
				"reason": "task's expires_at timestamp has been reached before it could be processed",
			})
			if err != nil {
				return nil, errx.Wrap(err)
			}
			continue // Don't include in returned tasks
		}

		// The dequeue query has already counted this pickup, so Attempts is the
		// number of the run about to start, and runs 1..MaxAttempts are allowed.
		// A failed last run never gets here: Nack parks it with its own error.
		// Arriving past the limit means the last run never reported a result the
		// queue could record, so the task is parked instead of being run again.
		if task.Attempts > effectiveMaxAttempts(task.MaxAttempts) {
			err = q.moveToDLQ(ctx, db, task.ID, time.Now(), map[string]any{
				"code": CodeAttemptsExhausted,
				"reason": "task used all of its max_attempts, and its last attempt never reported a result " +
					"the queue could record (worker crash or stop, visibility timeout, no handler registered, " +
					"missing operation_id, or a failed ack/nack)",
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
