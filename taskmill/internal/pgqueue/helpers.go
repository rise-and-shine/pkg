package pgqueue

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/code19m/errx"
)

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

// effectiveMaxAttempts is the number of runs a task gets. A row stored with
// max_attempts below 1, which the enqueue refuses, still gets one run, so it
// can never be picked up forever.
func effectiveMaxAttempts(maxAttempts int) int {
	return max(maxAttempts, 1)
}

// encodableReason returns reason if it can be stored as JSON. Otherwise it
// returns a copy without the values that cannot be, and with encode_error
// naming them, so that a task is still parked with what can be kept.
func encodableReason(reason map[string]any) map[string]any {
	_, err := json.Marshal(reason)
	if err == nil {
		return reason
	}

	kept := make(map[string]any, len(reason)+1)
	dropped := make([]string, 0, 1)

	for key, value := range reason {
		_, valueErr := json.Marshal(value)
		if valueErr != nil {
			dropped = append(dropped, key)
			continue
		}
		kept[key] = value
	}

	slices.Sort(dropped)
	kept["encode_error"] = fmt.Sprintf("dropped %s: %v", strings.Join(dropped, ", "), err)

	return kept
}
