package taskmill

import "time"

// SchedulerOption is a functional option that configures a Scheduler.
type SchedulerOption func(*schedulerOptions)

type schedulerOptions struct {
	checkInterval time.Duration
}

// WithCheckInterval sets the interval between schedule checks.
// Default: 5s.
func WithCheckInterval(interval time.Duration) SchedulerOption {
	return func(c *schedulerOptions) {
		c.checkInterval = interval
	}
}
