package pgqueue

import "github.com/code19m/errx"

// validateSingleTask validates a SingleTask.
func validateSingleTask(task TaskParams) error {
	if task.IdempotencyKey == "" {
		return errx.New("[pgqueue]: idempotency key is required")
	}
	if task.Priority < -100 || task.Priority > 100 {
		return errx.New("[pgqueue]: priority must be between -100 and 100")
	}
	if task.MaxAttempts < 1 {
		return errx.New("[pgqueue]: max attempts must be >= 1")
	}
	if task.ExpiresAt != nil && task.ExpiresAt.Before(task.ScheduledAt) {
		return errx.New("[pgqueue]: expires at must be after scheduled at")
	}
	return nil
}

func validateDequeueParams(params DequeueParams) error {
	if params.QueueName == "" {
		return errx.New("[pgqueue]: queue name is required")
	}
	if params.VisibilityTimeout <= 0 {
		return errx.New("[pgqueue]: visibility timeout must be positive")
	}
	if params.BatchSize < 1 || params.BatchSize > 100 {
		return errx.New("[pgqueue]: batch size must be between 1 and 100")
	}
	return nil
}

// calculateLockID generates a lock ID for task group advisory locking.
func calculateLockID(queueName, taskGroupID string) uint64 {
	// Simple hash function using FNV-1a algorithm
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)

	hash := uint64(offset64)
	data := queueName + ":" + taskGroupID

	for i := range len(data) {
		hash ^= uint64(data[i])
		hash *= prime64
	}

	return hash
}

// taskGroupIDToAny converts *string to any for SQL queries.
// This eliminates duplicate null handling code across query functions.
func taskGroupIDToAny(taskGroupID *string) any {
	if taskGroupID != nil {
		return *taskGroupID
	}
	return nil
}
