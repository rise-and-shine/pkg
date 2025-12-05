package pgqueue

import (
	"errors"
	"fmt"
	"regexp"
	"time"
)

// Option is a function that configures a Queue.
type Option func(*queueOptions)

// queueOptions holds internal configuration for the queue.
type queueOptions struct {
	schema                   string
	retryStrategy            RetryStrategy
	defaultVisibilityTimeout time.Duration
	defaultMaxAttempts       int
	autoMigrate              bool
}

// WithRetryStrategy sets the retry strategy for the queue.
// Default: ExponentialBackoffStrategy.
func WithRetryStrategy(strategy RetryStrategy) Option {
	return func(o *queueOptions) {
		o.retryStrategy = strategy
	}
}

// WithDefaultVisibilityTimeout sets the default visibility timeout.
// Default: 30 seconds.
func WithDefaultVisibilityTimeout(timeout time.Duration) Option {
	return func(o *queueOptions) {
		o.defaultVisibilityTimeout = timeout
	}
}

// WithDefaultMaxAttempts sets the default maximum retry attempts.
// Default: 3.
func WithDefaultMaxAttempts(attempts int) Option {
	return func(o *queueOptions) {
		o.defaultMaxAttempts = attempts
	}
}

// WithAutoMigrate enables automatic schema migration on queue creation.
// Default: false.
func WithAutoMigrate(enable bool) Option {
	return func(o *queueOptions) {
		o.autoMigrate = enable
	}
}

// WithSchema sets the database schema for all queue tables.
// Default: "queue".
func WithSchema(name string) Option {
	return func(o *queueOptions) {
		if err := validateSchemaName(name); err != nil {
			panic(fmt.Sprintf("invalid schema name: %v", err))
		}
		o.schema = name
	}
}

// defaultOptions returns the default queue options.
func defaultOptions() queueOptions {
	return queueOptions{
		schema:                   "queue",
		retryStrategy:            NewExponentialBackoffStrategy(),
		defaultVisibilityTimeout: 30 * time.Second,
		defaultMaxAttempts:       3,
		autoMigrate:              false,
	}
}

// validateSchemaName validates a PostgreSQL schema name.
func validateSchemaName(name string) error {
	if name == "" {
		return errors.New("schema name cannot be empty")
	}
	if len(name) > 63 {
		return errors.New("schema name too long (max 63 characters)")
	}
	// Allow alphanumeric and underscore, must start with letter or underscore
	matched, _ := regexp.MatchString(`^[a-zA-Z_][a-zA-Z0-9_]*$`, name)
	if !matched {
		return errors.New("invalid schema name: must be alphanumeric with underscores")
	}
	return nil
}

// EnqueueOption configures message enqueueing.
type EnqueueOption func(*enqueueConfig)

// enqueueConfig holds configuration for enqueueing a message.
type enqueueConfig struct {
	messageGroupID *string
	priority       int
	delay          time.Duration
	scheduledAt    *time.Time
	maxAttempts    int
	expiresAt      *time.Time
}

// WithMessageGroupID sets the message group ID for FIFO ordering.
func WithMessageGroupID(groupID string) EnqueueOption {
	return func(c *enqueueConfig) {
		c.messageGroupID = &groupID
	}
}

// WithPriority sets the message priority (-100 to 100, default 0).
// Higher values are processed first.
func WithPriority(priority int) EnqueueOption {
	return func(c *enqueueConfig) {
		c.priority = priority
	}
}

// WithDelay postpones message availability by the specified duration.
func WithDelay(delay time.Duration) EnqueueOption {
	return func(c *enqueueConfig) {
		c.delay = delay
	}
}

// WithScheduledAt sets an explicit time when the message becomes available.
// This takes precedence over WithDelay.
func WithScheduledAt(scheduledAt time.Time) EnqueueOption {
	return func(c *enqueueConfig) {
		c.scheduledAt = &scheduledAt
	}
}

// WithMaxAttempts sets the maximum retry attempts (default from queue config).
func WithMaxAttempts(attempts int) EnqueueOption {
	return func(c *enqueueConfig) {
		c.maxAttempts = attempts
	}
}

// WithExpiresAt sets an explicit expiration time for the message.
func WithExpiresAt(expiresAt time.Time) EnqueueOption {
	return func(c *enqueueConfig) {
		c.expiresAt = &expiresAt
	}
}

// DequeueOption configures message dequeueing.
type DequeueOption func(*dequeueConfig)

// dequeueConfig holds configuration for dequeueing messages.
type dequeueConfig struct {
	messageGroupID    *string
	visibilityTimeout time.Duration
	batchSize         int
}

// WithMessageGroupIDFilter filters messages to a specific group.
func WithMessageGroupIDFilter(groupID string) DequeueOption {
	return func(c *dequeueConfig) {
		c.messageGroupID = &groupID
	}
}

// WithVisibilityTimeout controls how long the message remains invisible
// to other consumers (default from queue config).
func WithVisibilityTimeout(timeout time.Duration) DequeueOption {
	return func(c *dequeueConfig) {
		c.visibilityTimeout = timeout
	}
}

// WithBatchSize determines how many messages to dequeue at once (default 1, max 100).
func WithBatchSize(size int) DequeueOption {
	return func(c *dequeueConfig) {
		c.batchSize = size
	}
}
