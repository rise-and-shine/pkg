package scheduler

import (
	"time"

	"github.com/rise-and-shine/pkg/taskmill/enqueuer"
)

// Option is a functional option that configures a Scheduler.
type Option func(*options)

type options struct {
	checkInterval time.Duration
}

func defaultOptions() options {
	return options{
		checkInterval: 5 * time.Second,
	}
}

// WithCheckInterval sets the interval between schedule checks.
// Default: 5s.
func WithCheckInterval(interval time.Duration) Option {
	return func(c *options) {
		c.checkInterval = interval
	}
}

// Schedule defines a cron-based schedule.
type Schedule struct {
	// CronPattern is a standard cron expression (e.g., "0 0 * * *" for daily at midnight).
	CronPattern string

	// OperationID is a unique identifier for the task.
	// Should match the OperationID of the ucdef.AsyncTask.
	OperationID string

	// EnqueueOptions are additional options for enqueuing the task.
	EnqueueOptions []enqueuer.Option
}
