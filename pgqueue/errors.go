package pgqueue

import "errors"

var (
	// ErrMessageNotFound is returned when a message with the given ID doesn't exist.
	ErrMessageNotFound = errors.New("pgqueue: message not found")

	// ErrMessageInDLQ is returned when attempting to operate on a message in the DLQ.
	ErrMessageInDLQ = errors.New("pgqueue: message is in dead letter queue")

	// ErrQueueNameRequired is returned when queue name is not provided.
	ErrQueueNameRequired = errors.New("pgqueue: queue name is required")

	// ErrPayloadRequired is returned when payload is not provided.
	ErrPayloadRequired = errors.New("pgqueue: payload is required")

	// ErrIdempotencyKeyRequired is returned when idempotency key is not provided.
	ErrIdempotencyKeyRequired = errors.New("pgqueue: idempotency key is required")

	// ErrInvalidPriority is returned when priority is out of valid range.
	ErrInvalidPriority = errors.New("pgqueue: priority must be between -100 and 100")

	// ErrInvalidBatchSize is returned when batch size is invalid.
	ErrInvalidBatchSize = errors.New("pgqueue: batch size must be between 1 and 100")

	// ErrInvalidVisibilityTimeout is returned when visibility timeout is invalid.
	ErrInvalidVisibilityTimeout = errors.New("pgqueue: visibility timeout must be positive")

	// ErrNoMessages is returned when no messages are available.
	ErrNoMessages = errors.New("pgqueue: no messages available")
)
