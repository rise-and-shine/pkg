package pgqueue

import (
	"time"
)

// Task represents a queue task with all its metadata.
type Task struct {
	// ID is the unique identifier for the task.
	ID int64 `bun:"id,pk"`

	// QueueName identifies which queue this task belongs to.
	QueueName string `bun:"queue_name"`

	// TaskGroupID enables FIFO ordering within a group.
	// Tasks with the same group ID are processed sequentially.
	TaskGroupID *string `bun:"task_group_id"`

	// OperationID identifies which handler should process this task.
	OperationID string `bun:"operation_id"`

	// Meta contains trace context and other metadata as JSONB.
	Meta map[string]string `bun:"meta,type:jsonb"`

	// Payload contains the business data as JSONB.
	// Must be a JSON-serializable type.
	Payload any `bun:"payload,type:jsonb"`

	// ScheduledAt indicates when the task becomes available for processing.
	ScheduledAt time.Time `bun:"scheduled_at"`

	// VisibleAt controls visibility timeout. Tasks are invisible to other
	// consumers until this time passes.
	VisibleAt time.Time `bun:"visible_at"`

	// ExpiresAt indicates when the task expires and should be moved to DLQ.
	// If nil, the task never expires.
	ExpiresAt *time.Time `bun:"expires_at"`

	// Priority determines processing order (higher values processed first).
	// Default is 0, valid range is -100 to 100.
	Priority int `bun:"priority"`

	// Attempts tracks how many times this task has been dequeued.
	Attempts int `bun:"attempts"`

	// MaxAttempts defines the maximum retry attempts before moving to DLQ.
	MaxAttempts int `bun:"max_attempts"`

	// IdempotencyKey is used for idempotency. Tasks with the same idempotency key
	// within a queue are considered duplicates.
	IdempotencyKey string `bun:"idempotency_key"`

	// CreatedAt stores when the task was originally enqueued.
	CreatedAt time.Time `bun:"created_at"`

	// UpdatedAt tracks the last modification time.
	UpdatedAt time.Time `bun:"updated_at"`

	// DLQAt indicates when the task was moved to the dead letter queue.
	// If nil, the task is not in the DLQ.
	DLQAt *time.Time `bun:"dlq_at"`

	// DLQReason contains structured error information if the task is in the DLQ.
	DLQReason map[string]any `bun:"dlq_reason,type:jsonb"`

	// Ephemeral indicates whether the task result should be discarded on completion.
	// If true, the task will not be saved to task_results on Ack.
	Ephemeral bool `bun:"ephemeral"`
}

// TaskResult represents a completed task stored in the task_results table.
type TaskResult struct {
	// ID is the original task ID from task_queue.
	ID int64 `bun:"id,pk"`

	// QueueName identifies which queue this task belonged to.
	QueueName string `bun:"queue_name"`

	// TaskGroupID is the FIFO group ID (if any).
	TaskGroupID *string `bun:"task_group_id"`

	// OperationID identifies which handler processed this task.
	OperationID string `bun:"operation_id"`

	// Meta contains trace context and other metadata.
	Meta map[string]string `bun:"meta,type:jsonb"`

	// Payload contains the original business data as JSONB.
	// Must be a JSON-serializable type.
	Payload any `bun:"payload,type:jsonb"`

	// Priority is the original priority.
	Priority int `bun:"priority"`

	// Attempts is how many attempts it took to complete.
	Attempts int `bun:"attempts"`

	// MaxAttempts is the original max attempts limit.
	MaxAttempts int `bun:"max_attempts"`

	// IdempotencyKey is for reference/lookup.
	IdempotencyKey string `bun:"idempotency_key"`

	// ScheduledAt is when the task was scheduled.
	ScheduledAt time.Time `bun:"scheduled_at"`

	// CreatedAt is when the task was originally enqueued.
	CreatedAt time.Time `bun:"created_at"`

	// CompletedAt is when the task was acknowledged.
	CompletedAt time.Time `bun:"completed_at"`
}

// ListResultsParams contains parameters for listing task results.
type ListResultsParams struct {
	// QueueName filters by queue (optional).
	QueueName *string

	// TaskGroupID filters by FIFO group (optional).
	TaskGroupID *string

	// CompletedAfter filters results completed after this time (optional).
	CompletedAfter *time.Time

	// CompletedBefore filters results completed before this time (optional).
	CompletedBefore *time.Time

	// Limit is the maximum number of results to return.
	// Default: 100, Max: 1000.
	Limit int

	// Offset is the pagination offset.
	Offset int
}

// CleanupResultsParams contains parameters for cleaning up old task results.
type CleanupResultsParams struct {
	// CompletedBefore deletes results where completed_at < this time (required).
	CompletedBefore time.Time

	// QueueName filters cleanup to a specific queue (optional).
	QueueName *string
}

// ListDLQTasksParams contains parameters for listing DLQ tasks.
type ListDLQTasksParams struct {
	// QueueName filters by queue (optional).
	QueueName *string

	// OperationID filters by operation (optional).
	OperationID *string

	// DLQAfter filters tasks moved to DLQ after this time (optional).
	DLQAfter *time.Time

	// DLQBefore filters tasks moved to DLQ before this time (optional).
	DLQBefore *time.Time

	// Limit is the maximum number of tasks to return.
	// Default: 100, Max: 1000.
	Limit int

	// Offset is the pagination offset.
	Offset int
}

// DLQTask represents a task in the dead letter queue.
type DLQTask struct {
	// ID is the unique identifier for the task.
	ID int64 `bun:"id"`

	// QueueName identifies which queue this task belongs to.
	QueueName string `bun:"queue_name"`

	// TaskGroupID enables FIFO ordering within a group.
	TaskGroupID *string `bun:"task_group_id"`

	// OperationID identifies which handler should process this task.
	OperationID string `bun:"operation_id"`

	// Payload contains the business data as JSONB.
	Payload any `bun:"payload,type:jsonb"`

	// Priority determines processing order.
	Priority int `bun:"priority"`

	// Attempts tracks how many times this task has been dequeued.
	Attempts int `bun:"attempts"`

	// MaxAttempts defines the maximum retry attempts.
	MaxAttempts int `bun:"max_attempts"`

	// IdempotencyKey is used for idempotency.
	IdempotencyKey string `bun:"idempotency_key"`

	// CreatedAt stores when the task was originally enqueued.
	CreatedAt time.Time `bun:"created_at"`

	// DLQAt indicates when the task was moved to the dead letter queue.
	DLQAt time.Time `bun:"dlq_at"`

	// DLQReason contains structured error information.
	DLQReason map[string]any `bun:"dlq_reason,type:jsonb"`
}

// TaskSchedule represents a cron-based schedule stored in the database.
type TaskSchedule struct {
	// ID is the unique identifier for the schedule.
	ID int64 `bun:"id,pk"`

	// OperationID uniquely identifies the scheduled task.
	OperationID string `bun:"operation_id"`

	// QueueName is the queue to enqueue tasks to.
	QueueName string `bun:"queue_name"`

	// CronPattern is the cron expression for scheduling.
	CronPattern string `bun:"cron_pattern"`

	// NextRunAt is when the schedule should next execute.
	NextRunAt time.Time `bun:"next_run_at"`

	// LastRunAt is when the schedule last executed.
	LastRunAt *time.Time `bun:"last_run_at"`

	// LastRunStatus is the status of the last run ('success' or 'failed').
	LastRunStatus *string `bun:"last_run_status"`

	// LastError contains the error message if the last run failed.
	LastError *string `bun:"last_error"`

	// RunCount is the total number of successful runs.
	RunCount int64 `bun:"run_count"`

	// CreatedAt is when the schedule was created.
	CreatedAt time.Time `bun:"created_at"`

	// UpdatedAt is when the schedule was last updated.
	UpdatedAt time.Time `bun:"updated_at"`
}

// QueueStats contains statistics about a queue.
type QueueStats struct {
	// QueueName identifies the queue.
	QueueName string `bun:"queue_name"`

	// Total is the total number of tasks (excluding DLQ).
	Total int64 `bun:"total"`

	// Available is the number of tasks ready for processing.
	Available int64 `bun:"available"`

	// InFlight is the number of tasks currently being processed.
	InFlight int64 `bun:"in_flight"`

	// Scheduled is the number of tasks scheduled for future processing.
	Scheduled int64 `bun:"scheduled"`

	// InDLQ is the number of tasks in the dead letter queue.
	InDLQ int64 `bun:"in_dlq"`

	// OldestTask is the creation time of the oldest task.
	OldestTask *time.Time `bun:"oldest_task"`

	// AvgAttempts is the average number of attempts across all tasks.
	AvgAttempts float64 `bun:"avg_attempts"`

	// P95Attempts is the 95th percentile of attempts.
	P95Attempts float64 `bun:"p95_attempts"`
}
