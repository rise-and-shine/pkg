package taskmill

import (
	"time"
)

// Schedule defines a cron-based task schedule.
type Schedule struct {
	// ID is a unique identifier for this schedule.
	ID string

	// TaskName is the registered task's OperationID.
	TaskName string

	// CronPattern is a standard cron expression (e.g., "0 0 * * *" for daily at midnight).
	// Supports: minute, hour, day of month, month, day of week.
	CronPattern string

	// Payload is the data passed to the task (must be JSON-serializable).
	Payload any

	// QueueName specifies which pgqueue queue to use (optional).
	// If empty, defaults to WorkerConfig.QueueName.
	QueueName string

	// Priority determines task processing order (-100 to 100).
	Priority int

	// MaxAttempts is the retry limit before DLQ.
	// If 0, defaults to WorkerConfig.DefaultMaxAttempts.
	MaxAttempts int

	// Timezone specifies the timezone for cron evaluation (e.g., "America/New_York").
	// Default: UTC.
	Timezone string

	// Enabled indicates if this schedule is active.
	Enabled bool
}

// EnqueueOption is a functional option for customizing task enqueueing.
type EnqueueOption func(*enqueueOptions)

// enqueueOptions contains options for enqueueing tasks.
type enqueueOptions struct {
	queueName      string
	priority       int
	maxAttempts    int
	scheduledAt    time.Time
	expiresAt      *time.Time
	messageGroupID *string
}

// WithQueue specifies the queue name.
func WithQueue(queueName string) EnqueueOption {
	return func(opts *enqueueOptions) {
		opts.queueName = queueName
	}
}

// WithPriority specifies the task priority (-100 to 100).
func WithPriority(priority int) EnqueueOption {
	return func(opts *enqueueOptions) {
		opts.priority = priority
	}
}

// WithMaxAttempts specifies the retry limit.
func WithMaxAttempts(maxAttempts int) EnqueueOption {
	return func(opts *enqueueOptions) {
		opts.maxAttempts = maxAttempts
	}
}

// WithScheduledAt specifies when the task becomes available.
func WithScheduledAt(scheduledAt time.Time) EnqueueOption {
	return func(opts *enqueueOptions) {
		opts.scheduledAt = scheduledAt
	}
}

// WithExpiresAt specifies when the task expires.
func WithExpiresAt(expiresAt time.Time) EnqueueOption {
	return func(opts *enqueueOptions) {
		opts.expiresAt = &expiresAt
	}
}

// WithMessageGroupID specifies the FIFO group ID.
func WithMessageGroupID(groupID string) EnqueueOption {
	return func(opts *enqueueOptions) {
		opts.messageGroupID = &groupID
	}
}
