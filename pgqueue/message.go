package pgqueue

import (
	"time"
)

// Message represents a queue message with all its metadata.
type Message struct {
	// ID is the unique identifier for the message.
	ID int64 `bun:"id,pk"`

	// QueueName identifies which queue this message belongs to.
	QueueName string `bun:"queue_name"`

	// MessageGroupID enables FIFO ordering within a group.
	// Messages with the same group ID are processed sequentially.
	MessageGroupID *string `bun:"message_group_id"`

	// Payload contains the actual message data as JSONB.
	Payload map[string]any `bun:"payload,type:jsonb"`

	// ScheduledAt indicates when the message becomes available for processing.
	ScheduledAt time.Time `bun:"scheduled_at"`

	// VisibleAt controls visibility timeout. Messages are invisible to other
	// consumers until this time passes.
	VisibleAt time.Time `bun:"visible_at"`

	// ExpiresAt indicates when the message expires and should be moved to DLQ.
	// If nil, the message never expires.
	ExpiresAt *time.Time `bun:"expires_at"`

	// Priority determines processing order (higher values processed first).
	// Default is 0, valid range is -100 to 100.
	Priority int `bun:"priority"`

	// Attempts tracks how many times this message has been dequeued.
	Attempts int `bun:"attempts"`

	// MaxAttempts defines the maximum retry attempts before moving to DLQ.
	MaxAttempts int `bun:"max_attempts"`

	// IdempotencyKey is used for idempotency. Messages with the same idempotency key
	// within a queue are considered duplicates.
	IdempotencyKey string `bun:"idempotency_key"`

	// CreatedAt stores when the message was originally enqueued.
	CreatedAt time.Time `bun:"created_at"`

	// UpdatedAt tracks the last modification time.
	UpdatedAt time.Time `bun:"updated_at"`

	// DLQAt indicates when the message was moved to the dead letter queue.
	// If nil, the message is not in the DLQ.
	DLQAt *time.Time `bun:"dlq_at"`

	// DLQReason contains structured error information if the message is in the DLQ.
	DLQReason map[string]any `bun:"dlq_reason,type:jsonb"`
}

// SingleMessage represents a single message for batch enqueueing.
type SingleMessage struct {
	// Payload contains the message data (required, can be empty map).
	Payload map[string]any

	// IdempotencyKey prevents duplicate messages (required, non-empty).
	IdempotencyKey string

	// MessageGroupID enables FIFO ordering within a group (optional).
	// Messages with same group ID are processed sequentially.
	// nil = standard (non-FIFO) queue.
	MessageGroupID *string

	// Priority determines processing order (required).
	// Range: -100 (lowest) to 100 (highest), 0 = normal.
	Priority int

	// ScheduledAt is when the message becomes available (required).
	// Use time.Now() for immediate availability.
	ScheduledAt time.Time

	// MaxAttempts is the retry limit before moving to DLQ (required).
	// Must be >= 1.
	MaxAttempts int

	// ExpiresAt is the expiration time (optional).
	// If set, message moves to DLQ after this time.
	// nil = never expires.
	ExpiresAt *time.Time
}

// QueueStats contains statistics about a queue.
type QueueStats struct {
	// QueueName identifies the queue.
	QueueName string `bun:"queue_name"`

	// Total is the total number of messages (excluding DLQ).
	Total int64 `bun:"total"`

	// Available is the number of messages ready for processing.
	Available int64 `bun:"available"`

	// InFlight is the number of messages currently being processed.
	InFlight int64 `bun:"in_flight"`

	// Scheduled is the number of messages scheduled for future processing.
	Scheduled int64 `bun:"scheduled"`

	// InDLQ is the number of messages in the dead letter queue.
	InDLQ int64 `bun:"in_dlq"`

	// OldestMessage is the creation time of the oldest message.
	OldestMessage *time.Time `bun:"oldest_message"`

	// AvgAttempts is the average number of attempts across all messages.
	AvgAttempts float64 `bun:"avg_attempts"`

	// P95Attempts is the 95th percentile of attempts.
	P95Attempts float64 `bun:"p95_attempts"`
}
