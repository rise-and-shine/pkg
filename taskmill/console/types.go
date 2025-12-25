package console

import "time"

// QueueStats contains queue monitoring statistics.
type QueueStats struct {
	// QueueName identifies the queue.
	QueueName string

	// Total is the total number of tasks (excluding DLQ).
	Total int64

	// Available is the number of tasks ready for processing.
	Available int64

	// InFlight is the number of tasks currently being processed.
	InFlight int64

	// Scheduled is the number of tasks scheduled for future processing.
	Scheduled int64

	// InDLQ is the number of tasks in the dead letter queue.
	InDLQ int64

	// OldestTask is the creation time of the oldest task.
	OldestTask *time.Time

	// AvgAttempts is the average number of attempts across all tasks.
	AvgAttempts float64

	// P95Attempts is the 95th percentile of attempts.
	P95Attempts float64
}

// ScheduleInfo contains schedule information from the database.
type ScheduleInfo struct {
	// OperationID is the unique identifier for the scheduled task.
	OperationID string

	// QueueName is the queue to enqueue tasks to.
	QueueName string

	// CronPattern is the cron expression for scheduling.
	CronPattern string

	// NextRunAt is when the schedule will next execute.
	NextRunAt time.Time

	// LastRunAt is when the schedule last executed.
	LastRunAt *time.Time

	// LastRunStatus is the status of the last run ('success' or 'failed').
	LastRunStatus *string

	// LastError contains the error message if the last run failed.
	LastError *string

	// RunCount is the total number of successful runs.
	RunCount int64
}

// TaskResult represents a completed task from history.
type TaskResult struct {
	// ID is the original task ID.
	ID int64

	// QueueName identifies which queue this task belonged to.
	QueueName string

	// TaskGroupID is the FIFO group ID (if any).
	TaskGroupID *string

	// OperationID identifies which handler processed this task.
	OperationID string

	// Payload contains the original business data.
	Payload any

	// Priority is the original priority.
	Priority int

	// Attempts is how many attempts it took to complete.
	Attempts int

	// MaxAttempts is the original max attempts limit.
	MaxAttempts int

	// IdempotencyKey is for reference/lookup.
	IdempotencyKey string

	// ScheduledAt is when the task was scheduled.
	ScheduledAt time.Time

	// CreatedAt is when the task was originally enqueued.
	CreatedAt time.Time

	// CompletedAt is when the task was acknowledged.
	CompletedAt time.Time
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
