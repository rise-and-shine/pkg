package taskmill

import (
	"time"

	"github.com/google/uuid"
	"github.com/rise-and-shine/pkg/taskmill/internal/pgqueue"
)

func getSchemaName() string {
	return "taskmill"
}

func getRetryStrategy() pgqueue.RetryStrategy {
	return pgqueue.NewExponentialBackoffStrategy()
}

func defaultEnqueueOptions() *enqueueOptions {
	return &enqueueOptions{
		priority:       0,
		maxAttempts:    3,
		scheduledAt:    time.Now(),
		expiresAt:      nil,
		messageGroupID: nil,
		idempotencyKey: uuid.NewString(),
	}
}

func defaultSchedulerOptions() schedulerOptions {
	return schedulerOptions{
		checkInterval: 5 * time.Second,
	}
}

func defaultWorkerOptions() workerOptions {
	return workerOptions{
		concurrency:       3,
		pollInterval:      time.Second,
		visibilityTimeout: 1 * time.Minute,
		batchSize:         1,
		processTimeout:    30 * time.Second,
		messageGroupID:    nil,
	}
}
