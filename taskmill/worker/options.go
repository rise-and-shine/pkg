package worker

import "time"

// Option is a functional option for customizing a Worker instance.
type Option func(*options)

type options struct {
	concurrency       int
	pollInterval      time.Duration
	visibilityTimeout time.Duration
	batchSize         int
	processTimeout    time.Duration
	taskGroupID       *string
}

func defaultOptions() options {
	return options{
		concurrency:       3,
		pollInterval:      time.Second,
		visibilityTimeout: 1 * time.Minute,
		batchSize:         1,
		processTimeout:    30 * time.Second,
		taskGroupID:       nil,
	}
}

// WithConcurrency sets the concurrency level for the worker.
// Default: 3.
func WithConcurrency(concurrency int) Option {
	return func(o *options) {
		o.concurrency = concurrency
	}
}

// WithPollInterval sets the polling interval for the worker.
// Default: 1s.
func WithPollInterval(pollInterval time.Duration) Option {
	return func(o *options) {
		o.pollInterval = pollInterval
	}
}

// WithVisibilityTimeout sets the visibility timeout for the worker.
// Default: 1m.
func WithVisibilityTimeout(visibilityTimeout time.Duration) Option {
	return func(o *options) {
		o.visibilityTimeout = visibilityTimeout
	}
}

// WithBatchSize sets the batch size for the worker.
// Default: 1.
func WithBatchSize(batchSize int) Option {
	return func(o *options) {
		o.batchSize = batchSize
	}
}

// WithProcessTimeout sets the process timeout for the worker.
// Default: 30s.
func WithProcessTimeout(processTimeout time.Duration) Option {
	return func(o *options) {
		o.processTimeout = processTimeout
	}
}

// WithTaskGroup sets the task group ID for the worker to ensure FIFO ordering.
// Default: nil.
func WithTaskGroup(taskGroupID *string) Option {
	return func(o *options) {
		o.taskGroupID = taskGroupID
	}
}
