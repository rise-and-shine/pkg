package pgqueue

import "github.com/code19m/errx"

// validateQueueConfig validates a QueueConfig.
func validateQueueConfig(config *QueueConfig) error {
	if config.Schema == "" {
		return errx.New("[pgqueue]: schema is required")
	}
	if config.RetryStrategy == nil {
		return errx.New("[pgqueue]: retry strategy is required")
	}
	return nil
}

// validateSingleMsg validates a SingleMessage.
func validateSingleMsg(msg SingleMessage) error {
	if msg.IdempotencyKey == "" {
		return errx.New("[pgqueue]: idempotency key is required")
	}
	if msg.Priority < -100 || msg.Priority > 100 {
		return errx.New("[pgqueue]: priority must be between -100 and 100")
	}
	if msg.MaxAttempts < 1 {
		return errx.New("[pgqueue]: max attempts must be >= 1")
	}
	if msg.ExpiresAt != nil && msg.ExpiresAt.Before(msg.ScheduledAt) {
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

// calculateLockID generates a lock ID for message group advisory locking.
func calculateLockID(queueName, messageGroupID string) uint64 {
	// Simple hash function using FNV-1a algorithm
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)

	hash := uint64(offset64)
	data := queueName + ":" + messageGroupID

	for i := range len(data) {
		hash ^= uint64(data[i])
		hash *= prime64
	}

	return hash
}

// messageGroupIDToAny converts *string to any for SQL queries.
// This eliminates duplicate null handling code across query functions.
func messageGroupIDToAny(messageGroupID *string) any {
	if messageGroupID != nil {
		return *messageGroupID
	}
	return nil
}
