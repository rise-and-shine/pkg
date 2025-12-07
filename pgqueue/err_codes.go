package pgqueue

const (
	// CodeNoMessages is returned when no messages are found.
	CodeNoMessages = "NO_MESSAGES"

	// CodeDuplicateMessage is returned when a message with the same idempotency key already exists.
	CodeDuplicateMessage = "DUPLICATE_MESSAGE"
)
