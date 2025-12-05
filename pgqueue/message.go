package pgqueue

import (
	"time"
)

const (
	// TableNameQueueMessages is the name of the main queue messages table.
	TableNameQueueMessages = "queue_messages"
)

// BatchMessage represents a single message for batch enqueueing.
type BatchMessage struct {
	Payload        map[string]any
	IdempotencyKey string
	Options        []EnqueueOption
}

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

	// ExpiresAt indicates when the message expires and should be moved to DLQ.
	// If nil, the message never expires.
	ExpiresAt *time.Time `bun:"expires_at"`

	// DLQAt indicates when the message was moved to the dead letter queue.
	// If nil, the message is not in the DLQ.
	DLQAt *time.Time `bun:"dlq_at"`

	// DLQReason contains the error reason if the message is in the DLQ.
	DLQReason *string `bun:"dlq_reason"`
}

// EnqueueOptions contains options for enqueueing a message.
type EnqueueOptions struct {
	// QueueName identifies the queue (required).
	QueueName string

	// Payload contains the message data (required).
	Payload map[string]any

	// MessageGroupID enables FIFO ordering within a group (optional).
	MessageGroupID *string

	// Priority determines processing order, default 0, range -100 to 100.
	Priority int

	// Delay postpones message availability by the specified duration.
	Delay time.Duration

	// ScheduledAt sets an explicit time when the message becomes available.
	// If set, this takes precedence over Delay.
	ScheduledAt *time.Time

	// MaxAttempts sets the maximum retry attempts, default 3.
	MaxAttempts int

	// ExpiresAt sets an explicit expiration time for the message.
	// If set, the message will be moved to DLQ when dequeued after this time.
	ExpiresAt *time.Time

	// IdempotencyKey is required for idempotency. Messages with the same key are deduplicated.
	IdempotencyKey string
}

// DequeueOptions contains options for dequeuing messages.
type DequeueOptions struct {
	// QueueName identifies the queue to dequeue from (required).
	QueueName string

	// MessageGroupID filters messages to a specific group (optional).
	MessageGroupID *string

	// VisibilityTimeout controls how long the message remains invisible
	// to other consumers. Default is 30 seconds.
	VisibilityTimeout time.Duration

	// BatchSize determines how many messages to dequeue at once.
	// Default is 1, maximum recommended is 100.
	BatchSize int
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
