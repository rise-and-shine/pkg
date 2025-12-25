package taskmill

import (
	"time"
)

// EnqueueOption is a functional option for customizing task enqueueing.
type EnqueueOption func(*enqueueOptions)

// enqueueOptions contains options for enqueueing tasks.
type enqueueOptions struct {
	priority       int
	maxAttempts    int
	scheduledAt    time.Time
	expiresAt      *time.Time
	taskGroupID    *string
	idempotencyKey string
	ephemeral      bool
}

// WithPriority specifies the task priority (-100 to 100).
// Default is 0.
func WithPriority(priority int) EnqueueOption {
	return func(opts *enqueueOptions) {
		opts.priority = priority
	}
}

// WithMaxAttempts specifies the retry limit.
// Default is 3.
func WithMaxAttempts(maxAttempts int) EnqueueOption {
	return func(opts *enqueueOptions) {
		opts.maxAttempts = maxAttempts
	}
}

// WithScheduledAt specifies when the task becomes available.
// Default is now.
func WithScheduledAt(scheduledAt time.Time) EnqueueOption {
	return func(opts *enqueueOptions) {
		opts.scheduledAt = scheduledAt
	}
}

// WithExpiresAt specifies when the task expires.
// Default is nil.
func WithExpiresAt(expiresAt time.Time) EnqueueOption {
	return func(opts *enqueueOptions) {
		opts.expiresAt = &expiresAt
	}
}

// WithTaskGroupID specifies the FIFO group ID.
// Default is nil.
func WithTaskGroupID(groupID string) EnqueueOption {
	return func(opts *enqueueOptions) {
		opts.taskGroupID = &groupID
	}
}

// WithIdempotencyKey specifies a custom idempotency key for deduplication.
// If not specified, a key is generated from uuid.NewString().
// Use this when you need deterministic deduplication based on business logic.
func WithIdempotencyKey(key string) EnqueueOption {
	return func(opts *enqueueOptions) {
		opts.idempotencyKey = key
	}
}

// WithEphemeral marks the task as ephemeral.
// Ephemeral tasks will not be saved to task_results on completion.
// Default is false (results are saved).
func WithEphemeral() EnqueueOption {
	return func(opts *enqueueOptions) {
		opts.ephemeral = true
	}
}
