package taskmill

import (
	"time"

	"github.com/code19m/errx"
	"github.com/rise-and-shine/pkg/observability/logger"
	"github.com/rise-and-shine/pkg/pgqueue"
	"github.com/uptrace/bun"
)

// WorkerConfig configures the Worker instance.
type WorkerConfig struct {
	// DB is the bun database connection (required).
	DB *bun.DB

	// Queue is the pgqueue instance (required).
	Queue pgqueue.Queue

	// Logger is the application logger (required).
	Logger logger.Logger

	// ServiceName identifies the service (required).
	ServiceName string

	// ServiceVersion identifies the service version (required).
	ServiceVersion string

	// QueueName specifies the queue to poll and enqueue to.
	// Default: "default".
	QueueName string

	// Concurrency is the number of concurrent task executions.
	// Default: 4.
	Concurrency int

	// PollInterval is the delay between dequeue attempts when queue is empty.
	// Default: 1s.
	PollInterval time.Duration

	// VisibilityTimeout is how long a dequeued task remains invisible.
	// Default: 60s.
	VisibilityTimeout time.Duration

	// BatchSize is how many tasks to dequeue per poll.
	// Default: 10.
	BatchSize int

	// ShutdownTimeout is how long to wait for in-flight tasks during Stop.
	// Default: 30s.
	ShutdownTimeout time.Duration

	// TaskTimeout is the maximum execution time per task.
	// Default: 30s.
	TaskTimeout time.Duration

	// DefaultMaxAttempts is the retry limit when not specified.
	// Default: 3.
	DefaultMaxAttempts int

	// MessageGroupID filters to a specific FIFO group (optional).
	MessageGroupID *string

	// EnableTracing enables OpenTelemetry tracing.
	// Default: true.
	EnableTracing bool

	// EnableAlerting enables error alerting.
	// Default: true.
	EnableAlerting bool
}

// SchedulerConfig configures the Scheduler instance.
type SchedulerConfig struct {
	// Logger is the application logger (required).
	Logger logger.Logger

	// TickInterval is how often the scheduler checks for due tasks.
	// Default: 1s.
	TickInterval time.Duration
}

// applyWorkerDefaults applies default values to WorkerConfig.
func applyWorkerDefaults(cfg *WorkerConfig) {
	if cfg.QueueName == "" {
		cfg.QueueName = "default"
	}
	if cfg.Concurrency == 0 {
		cfg.Concurrency = 4
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = time.Second
	}
	if cfg.VisibilityTimeout == 0 {
		cfg.VisibilityTimeout = 60 * time.Second
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 10
	}
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = 30 * time.Second
	}
	if cfg.TaskTimeout == 0 {
		cfg.TaskTimeout = 30 * time.Second
	}
	if cfg.DefaultMaxAttempts == 0 {
		cfg.DefaultMaxAttempts = 3
	}
}

// applySchedulerDefaults applies default values to SchedulerConfig.
func applySchedulerDefaults(cfg *SchedulerConfig) {
	if cfg.TickInterval == 0 {
		cfg.TickInterval = time.Second
	}
}

// validateWorkerConfig validates the Worker configuration.
func validateWorkerConfig(cfg *WorkerConfig) error {
	if cfg == nil {
		return errx.New("[taskmill.worker]: config is required")
	}
	if cfg.DB == nil {
		return errx.New("[taskmill.worker]: DB is required")
	}
	if cfg.Queue == nil {
		return errx.New("[taskmill.worker]: Queue is required")
	}
	if cfg.Logger == nil {
		return errx.New("[taskmill.worker]: Logger is required")
	}
	if cfg.ServiceName == "" {
		return errx.New("[taskmill.worker]: ServiceName is required")
	}
	if cfg.ServiceVersion == "" {
		return errx.New("[taskmill.worker]: ServiceVersion is required")
	}
	if cfg.Concurrency < 1 || cfg.Concurrency > 1000 {
		return errx.New("[taskmill.worker]: Concurrency must be between 1 and 1000")
	}
	if cfg.BatchSize < 1 || cfg.BatchSize > 100 {
		return errx.New("[taskmill.worker]: BatchSize must be between 1 and 100")
	}
	return nil
}

// validateSchedulerConfig validates the Scheduler configuration.
func validateSchedulerConfig(cfg *SchedulerConfig) error {
	if cfg == nil {
		return errx.New("[taskmill.scheduler]: config is required")
	}
	if cfg.Logger == nil {
		return errx.New("[taskmill.scheduler]: Logger is required")
	}
	if cfg.TickInterval <= 0 {
		return errx.New("[taskmill.scheduler]: TickInterval must be positive")
	}
	return nil
}
