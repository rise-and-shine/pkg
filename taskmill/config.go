package taskmill

import (
	"time"

	"github.com/code19m/errx"
	"github.com/rise-and-shine/pkg/pgqueue"
	"github.com/uptrace/bun"
)

// WorkerConfig configures a Worker instance.
type WorkerConfig struct {
	// DB is the bun DB instance (required).
	DB *bun.DB

	// Queue is the pgqueue instance (required).
	Queue pgqueue.Queue

	// QueueName is the name of the queue to poll (required).
	QueueName string

	// ServiceName is the name of the service for meta injection (required).
	ServiceName string

	// ServiceVersion is the version of the service for meta injection (required).
	ServiceVersion string

	// MessageGroupID is an optional FIFO group ID filter.
	// If set, the worker will only process messages with this group ID.
	MessageGroupID *string

	// Concurrency is the number of concurrent worker goroutines.
	// Default: 10, Range: 1-1000.
	Concurrency int

	// PollInterval is the time between polling attempts when the queue is empty.
	// Default: 1s.
	PollInterval time.Duration

	// VisibilityTimeout is how long a message is invisible after dequeue.
	// Must be >= ProcessTimeout to avoid duplicate processing.
	// Default: 30s.
	VisibilityTimeout time.Duration

	// BatchSize is the number of messages to fetch per poll.
	// Default: 10, Range: 1-100.
	BatchSize int

	// ProcessTimeout is the maximum time allowed for task processing.
	// Default: 25s.
	ProcessTimeout time.Duration
}

func (c *WorkerConfig) setDefaults() {
	if c.Concurrency == 0 {
		c.Concurrency = 10
	}
	if c.PollInterval == 0 {
		c.PollInterval = time.Second
	}
	if c.VisibilityTimeout == 0 {
		c.VisibilityTimeout = 30 * time.Second
	}
	if c.BatchSize == 0 {
		c.BatchSize = 10
	}
	if c.ProcessTimeout == 0 {
		c.ProcessTimeout = 25 * time.Second
	}
}

func (c *WorkerConfig) validate() error {
	if c.DB == nil {
		return errx.New("[taskmill]: DB is required")
	}
	if c.Queue == nil {
		return errx.New("[taskmill]: Queue is required")
	}
	if c.QueueName == "" {
		return errx.New("[taskmill]: QueueName is required")
	}
	if c.ServiceName == "" {
		return errx.New("[taskmill]: ServiceName is required")
	}
	if c.ServiceVersion == "" {
		return errx.New("[taskmill]: ServiceVersion is required")
	}
	if c.Concurrency < 1 || c.Concurrency > 1000 {
		return errx.New("[taskmill]: Concurrency must be between 1 and 1000")
	}
	if c.PollInterval <= 0 {
		return errx.New("[taskmill]: PollInterval must be positive")
	}
	if c.VisibilityTimeout <= 0 {
		return errx.New("[taskmill]: VisibilityTimeout must be positive")
	}
	if c.BatchSize < 1 || c.BatchSize > 100 {
		return errx.New("[taskmill]: BatchSize must be between 1 and 100")
	}
	if c.ProcessTimeout <= 0 {
		return errx.New("[taskmill]: ProcessTimeout must be positive")
	}
	if c.VisibilityTimeout < c.ProcessTimeout {
		return errx.New("[taskmill]: VisibilityTimeout must be >= ProcessTimeout to avoid duplicate processing")
	}
	return nil
}

// SchedulerConfig configures a Scheduler instance.
type SchedulerConfig struct {
	// DB is the bun DB instance (required).
	DB *bun.DB

	// Enqueuer is the taskmill Enqueuer instance (required).
	Enqueuer Enqueuer

	// CheckInterval is the interval between schedule checks.
	// Default: 5s.
	CheckInterval time.Duration
}

func (c *SchedulerConfig) setDefaults() {
	if c.CheckInterval == 0 {
		c.CheckInterval = 5 * time.Second
	}
}

func (c *SchedulerConfig) validate() error {
	if c.DB == nil {
		return errx.New("[taskmill]: DB is required")
	}
	if c.Enqueuer == nil {
		return errx.New("[taskmill]: Enqueuer is required")
	}
	if c.CheckInterval <= 0 {
		return errx.New("[taskmill]: CheckInterval must be positive")
	}
	return nil
}
