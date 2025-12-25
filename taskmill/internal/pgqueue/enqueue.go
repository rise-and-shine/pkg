package pgqueue

import (
	"context"
	"time"

	"github.com/code19m/errx"
	"github.com/uptrace/bun"
)

const (
	CodeDuplicateTask = "DUPLICATE_TASK"
)

// EnqueueBatch adds multiple tasks to the queue.
// All tasks in the batch will be enqueued to the same queue.
// Returns a list of task IDs for the enqueued tasks.
// If any task has a duplicate idempotency key, the entire batch fails with CodeDuplicateTask.
func (q *queue) EnqueueBatch(
	ctx context.Context,
	db bun.IDB,
	queueName string,
	tasks []TaskParams,
) ([]int64, error) {
	// Validate queue name
	if queueName == "" {
		return nil, errx.New("[pgqueue]: queue name is required")
	}

	if len(tasks) == 0 {
		return []int64{}, nil
	}

	// Validate all tasks first
	for _, task := range tasks {
		err := validateSingleTask(task)
		if err != nil {
			return nil, errx.Wrap(err, errx.WithDetails(errx.D{"task": task}))
		}
	}

	// Convert SingleTask to Task structs
	dbTasks := make([]Task, 0, len(tasks))
	for _, singleTask := range tasks {
		// Normalize payload
		payload := singleTask.Payload
		if payload == nil {
			payload = map[string]any{}
		}

		dbTasks = append(dbTasks, Task{
			QueueName:      queueName,
			TaskGroupID:    singleTask.TaskGroupID,
			OperationID:    singleTask.OperationID,
			Meta:           singleTask.Meta,
			Payload:        payload,
			ScheduledAt:    singleTask.ScheduledAt,
			VisibleAt:      singleTask.ScheduledAt, // Initially visible at scheduled time
			Priority:       singleTask.Priority,
			MaxAttempts:    singleTask.MaxAttempts,
			ExpiresAt:      singleTask.ExpiresAt,
			IdempotencyKey: singleTask.IdempotencyKey,
			Attempts:       0,
			Ephemeral:      singleTask.Ephemeral,
		})
	}

	// Insert all tasks in a single batch
	ids, err := q.insertTasks(ctx, db, dbTasks)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	return ids, nil
}

// TaskParams represents a parameters of a task to be enqueued.
type TaskParams struct {
	// OperationID identifies which handler should process this task (required).
	OperationID string

	// Meta contains trace context and other metadata (optional).
	Meta map[string]string

	// Payload contains the business data (optional, can be empty map).
	Payload map[string]any

	// IdempotencyKey prevents duplicate tasks (required, non-empty).
	IdempotencyKey string

	// TaskGroupID enables FIFO ordering within a group (optional).
	// Tasks with same group ID are processed sequentially.
	// nil = standard (non-FIFO) queue.
	TaskGroupID *string

	// Priority determines processing order (required).
	// Range: -100 (lowest) to 100 (highest), 0 = normal.
	Priority int

	// ScheduledAt is when the task becomes available (required).
	// Use time.Now() for immediate availability.
	ScheduledAt time.Time

	// MaxAttempts is the retry limit before moving to DLQ (required).
	// Must be >= 1.
	MaxAttempts int

	// ExpiresAt is the expiration time (optional).
	// If set, task moves to DLQ after this time.
	// nil = never expires.
	ExpiresAt *time.Time

	// Ephemeral indicates whether the task result should be discarded on completion.
	// If true, the task will not be saved to task_results on Ack.
	// Default is false (results are saved).
	Ephemeral bool
}
