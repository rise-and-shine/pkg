package config

import "github.com/rise-and-shine/pkg/taskmill/internal/pgqueue"

// SchemaName returns the database schema name for taskmill.
func SchemaName() string {
	return "taskmill"
}

// RetryStrategy returns the default retry strategy.
func RetryStrategy() pgqueue.RetryStrategy {
	return pgqueue.NewExponentialBackoffStrategy()
}
