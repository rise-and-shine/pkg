// Package meta provides functionality and constants for managing request metadata through context.
package meta

import (
	"context"

	"github.com/code19m/errx"
)

// ContextKey is a type for keys used in context values for metadata.
type ContextKey string

const (
	// TraceID represents a unique identifier for tracing requests across services.
	TraceID ContextKey = "trace_id"

	// OperationID represents an identifier for the operation being performed.
	OperationID ContextKey = "operation_id"

	// ActorType indicates the type of the actor triggering the action (user, cronjob, consumer, etc).
	ActorType ContextKey = "actor_type"

	// ActorID represents the unique identifier of the actor.
	ActorID ContextKey = "actor_id"

	// ObjectType indicates the type of the object being interacted with (user, group, order, etc).
	ObjectType ContextKey = "object_type"

	// ObjectID represents the unique identifier of the object.
	ObjectID ContextKey = "object_id"

	// IPAddress indicates the IP address of the client making the request.
	IPAddress ContextKey = "ip_address"

	// UserAgent indicates the user agent of the client making the request.
	UserAgent ContextKey = "user_agent"

	// AcceptLanguage indicates the natural language and locale that the client prefers.
	AcceptLanguage ContextKey = "accept-language"
)

// Get retrieves a metadata value from the context by its key.
// It returns the value as a string if found, or an error if the key does not exist.
func Get(ctx context.Context, key ContextKey) (string, error) {
	if value := ctx.Value(key); value != nil {
		if str, ok := value.(string); ok {
			return str, nil
		}
		return "", errx.New("[meta] type mismatch", errx.WithDetails(errx.D{"key": key, "value": value}))
	}
	return "", errx.New("[meta] key not found", errx.WithDetails(errx.D{"key": key}))
}

// Find retrieves a metadata value from the context by its key.
// It returns the value as a string if found, or an empty string if the key does not exist.
func Find(ctx context.Context, key ContextKey) string {
	if value := ctx.Value(key); value != nil {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}
