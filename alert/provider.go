package alert

import "context"

// Provider defines the interface for sending error alerts.
// Implementations of this interface can send alerts to various monitoring systems.
type Provider interface {
	// SendError sends an error alert with the given details.
	// ctx is the context for the operation.
	// errCode is a specific code identifying the error.
	// msg is a human-readable error message.
	// operation describes the operation during which the error occurred.
	// details is a map of additional string key-value pairs providing more context about the error.
	// Returns an error if sending the alert fails.
	SendError(ctx context.Context, errCode, msg, operation string, details map[string]string) error
}
