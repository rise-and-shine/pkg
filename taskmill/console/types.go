package console

import "time"

// QueueStats contains queue monitoring statistics.
type QueueStats struct {
	// QueueName identifies the queue.
	QueueName string `json:"queue_name"`

	// Total is the total number of tasks (excluding DLQ).
	Total int64 `json:"total"`

	// Available is the number of tasks ready for processing.
	Available int64 `json:"available"`

	// InFlight is the number of tasks currently being processed.
	InFlight int64 `json:"in_flight"`

	// Scheduled is the number of tasks scheduled for future processing.
	Scheduled int64 `json:"scheduled"`

	// InDLQ is the number of tasks in the dead letter queue.
	InDLQ int64 `json:"in_dlq"`

	// OldestTask is the creation time of the oldest task.
	OldestTask *time.Time `json:"oldest_task,omitempty"`

	// AvgAttempts is the average number of attempts across all tasks.
	AvgAttempts float64 `json:"avg_attempts"`

	// P95Attempts is the 95th percentile of attempts.
	P95Attempts float64 `json:"p95_attempts"`
}

// ScheduleInfo contains schedule information from the database.
type ScheduleInfo struct {
	// OperationID is the unique identifier for the scheduled task.
	OperationID string `json:"operation_id"`

	// QueueName is the queue to enqueue tasks to.
	QueueName string `json:"queue_name"`

	// CronPattern is the cron expression for scheduling.
	CronPattern string `json:"cron_pattern"`

	// NextRunAt is when the schedule will next execute.
	NextRunAt time.Time `json:"next_run_at"`

	// LastRunAt is when the schedule last executed.
	LastRunAt *time.Time `json:"last_run_at,omitempty"`

	// LastRunStatus is the status of the last run ('success' or 'failed').
	LastRunStatus *string `json:"last_run_status,omitempty"`

	// LastError contains the error message if the last run failed.
	LastError *string `json:"last_error,omitempty"`

	// RunCount is the total number of successful runs.
	RunCount int64 `json:"run_count"`
}

// TaskResult represents a completed task from history.
type TaskResult struct {
	// ID is the original task ID.
	ID int64 `json:"id"`

	// QueueName identifies which queue this task belonged to.
	QueueName string `json:"queue_name"`

	// TaskGroupID is the FIFO group ID (if any).
	TaskGroupID *string `json:"task_group_id,omitempty"`

	// OperationID identifies which handler processed this task.
	OperationID string `json:"operation_id"`

	// Payload contains the original business data.
	Payload any `json:"payload"`

	// Priority is the original priority.
	Priority int `json:"priority"`

	// Attempts is how many attempts it took to complete.
	Attempts int `json:"attempts"`

	// MaxAttempts is the original max attempts limit.
	MaxAttempts int `json:"max_attempts"`

	// IdempotencyKey is for reference/lookup.
	IdempotencyKey string `json:"idempotency_key"`

	// ScheduledAt is when the task was scheduled.
	ScheduledAt time.Time `json:"scheduled_at"`

	// CreatedAt is when the task was originally enqueued.
	CreatedAt time.Time `json:"created_at"`

	// CompletedAt is when the task was acknowledged.
	CompletedAt time.Time `json:"completed_at"`
}

// ListResultsParams contains parameters for listing task results.
type ListResultsParams struct {
	// QueueName filters by queue (optional).
	QueueName *string `json:"queue_name,omitempty"`

	// TaskGroupID filters by FIFO group (optional).
	TaskGroupID *string `json:"task_group_id,omitempty"`

	// CompletedAfter filters results completed after this time (optional).
	CompletedAfter *time.Time `json:"completed_after,omitempty"`

	// CompletedBefore filters results completed before this time (optional).
	CompletedBefore *time.Time `json:"completed_before,omitempty"`

	// Limit is the maximum number of results to return.
	// Default: 100, Max: 1000.
	Limit int `json:"limit"`

	// Offset is the pagination offset.
	Offset int `json:"offset"`
}

// CleanupResultsParams contains parameters for cleaning up old task results.
type CleanupResultsParams struct {
	// CompletedBefore deletes results where completed_at < this time (required).
	CompletedBefore time.Time `json:"completed_before"`

	// QueueName filters cleanup to a specific queue (optional).
	QueueName *string `json:"queue_name,omitempty"`
}

// ListDLQTasksParams contains parameters for listing DLQ tasks.
type ListDLQTasksParams struct {
	// QueueName filters by queue (optional).
	QueueName *string `json:"queue_name,omitempty"`

	// OperationID filters by operation (optional).
	OperationID *string `json:"operation_id,omitempty"`

	// DLQAfter filters tasks moved to DLQ after this time (optional).
	DLQAfter *time.Time `json:"dlq_after,omitempty"`

	// DLQBefore filters tasks moved to DLQ before this time (optional).
	DLQBefore *time.Time `json:"dlq_before,omitempty"`

	// Limit is the maximum number of tasks to return.
	// Default: 100, Max: 1000.
	Limit int `json:"limit"`

	// Offset is the pagination offset.
	Offset int `json:"offset"`
}

// DLQTask represents a task in the dead letter queue.
type DLQTask struct {
	// ID is the unique identifier for the task.
	ID int64 `json:"id"`

	// QueueName identifies which queue this task belongs to.
	QueueName string `json:"queue_name"`

	// TaskGroupID enables FIFO ordering within a group.
	TaskGroupID *string `json:"task_group_id,omitempty"`

	// OperationID identifies which handler should process this task.
	OperationID string `json:"operation_id"`

	// Payload contains the business data.
	Payload any `json:"payload"`

	// Priority determines processing order.
	Priority int `json:"priority"`

	// Attempts tracks how many times this task has been dequeued.
	Attempts int `json:"attempts"`

	// MaxAttempts defines the maximum retry attempts.
	MaxAttempts int `json:"max_attempts"`

	// IdempotencyKey is used for idempotency.
	IdempotencyKey string `json:"idempotency_key"`

	// CreatedAt stores when the task was originally enqueued.
	CreatedAt time.Time `json:"created_at"`

	// DLQAt indicates when the task was moved to the dead letter queue.
	DLQAt time.Time `json:"dlq_at"`

	// DLQReason contains structured error information.
	DLQReason map[string]any `json:"dlq_reason"`
}
