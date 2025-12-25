package taskmill

import "time"

// WorkerOption is a functional option for customizing a Worker instance.
type WorkerOption func(*workerOptions)

type workerOptions struct {
	concurrency       int
	pollInterval      time.Duration
	visibilityTimeout time.Duration
	batchSize         int
	processTimeout    time.Duration
	taskGroupID       *string
}

// WithConcurrency sets the concurrency level for the worker.
// Default: 3.
func WithConcurrency(concurrency int) WorkerOption {
	return func(o *workerOptions) {
		o.concurrency = concurrency
	}
}

// WithPollInterval sets the polling interval for the worker.
// Default: 1s.
func WithPollInterval(pollInterval time.Duration) WorkerOption {
	return func(o *workerOptions) {
		o.pollInterval = pollInterval
	}
}

// WithVisibilityTimeout sets the visibility timeout for the worker.
// Default: 1m.
func WithVisibilityTimeout(visibilityTimeout time.Duration) WorkerOption {
	return func(o *workerOptions) {
		o.visibilityTimeout = visibilityTimeout
	}
}

// WithBatchSize sets the batch size for the worker.
// Default: 1.
func WithBatchSize(batchSize int) WorkerOption {
	return func(o *workerOptions) {
		o.batchSize = batchSize
	}
}

// WithProcessTimeout sets the process timeout for the worker.
// Default: 30s.
func WithProcessTimeout(processTimeout time.Duration) WorkerOption {
	return func(o *workerOptions) {
		o.processTimeout = processTimeout
	}
}

// WithTaskGroup sets the task group ID for the worker to ensure FIFO ordering.
// Default: nil.
func WithTaskGroup(taskGroupID *string) WorkerOption {
	return func(o *workerOptions) {
		o.taskGroupID = taskGroupID
	}
}
