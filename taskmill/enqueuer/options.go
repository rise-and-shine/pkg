package enqueuer

import (
	"time"

	"github.com/google/uuid"
)

// Option is a functional option for customizing task enqueueing.
type Option func(*options)

// options contains options for enqueueing tasks.
type options struct {
	priority       int
	maxAttempts    int
	scheduledAt    time.Time
	expiresAt      *time.Time
	taskGroupID    *string
	idempotencyKey string
	ephemeral      bool
}

func defaultOptions() *options {
	return &options{
		priority:       0,
		maxAttempts:    3,
		scheduledAt:    time.Now(),
		expiresAt:      nil,
		taskGroupID:    nil,
		idempotencyKey: uuid.NewString(),
	}
}

// WithPriority specifies the task priority (-100 to 100).
// Default is 0.
func WithPriority(priority int) Option {
	return func(opts *options) {
		opts.priority = priority
	}
}

// WithMaxAttempts specifies the retry limit.
// Default is 3.
func WithMaxAttempts(maxAttempts int) Option {
	return func(opts *options) {
		opts.maxAttempts = maxAttempts
	}
}

// WithScheduledAt specifies when the task becomes available.
// Default is now.
func WithScheduledAt(scheduledAt time.Time) Option {
	return func(opts *options) {
		opts.scheduledAt = scheduledAt
	}
}

// WithExpiresAt specifies when the task expires.
// Default is nil.
func WithExpiresAt(expiresAt time.Time) Option {
	return func(opts *options) {
		opts.expiresAt = &expiresAt
	}
}

// WithTaskGroupID specifies the FIFO group ID.
// Default is nil.
func WithTaskGroupID(groupID string) Option {
	return func(opts *options) {
		opts.taskGroupID = &groupID
	}
}

// WithIdempotencyKey specifies a custom idempotency key for deduplication.
// If not specified, a key is generated from uuid.NewString().
// Use this when you need deterministic deduplication based on business logic.
func WithIdempotencyKey(key string) Option {
	return func(opts *options) {
		opts.idempotencyKey = key
	}
}

// WithEphemeral marks the task as ephemeral.
// Ephemeral tasks will not be saved to task_results on completion.
// Default is false (results are saved).
func WithEphemeral() Option {
	return func(opts *options) {
		opts.ephemeral = true
	}
}

// BatchTask represents a single task in a batch enqueue operation.
type BatchTask struct {
	// OperationID identifies which handler should process this task.
	OperationID string

	// Payload contains the business data.
	Payload any

	// Options are the enqueue options for this specific task.
	Options []Option
}
