// Package meta provides functionality for managing request metadata through context.
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

	// ActorType indicates the type of the actor triggering the action (user, cronjob, consumer, etc).
	ActorType ContextKey = "actor_type"

	// ActorID represents the unique identifier of the actor.
	ActorID ContextKey = "actor_id"

	// ServiceName identifies the name of current running service.
	ServiceName ContextKey = "service_name"

	// ServiceVersion indicates the version of the service.
	ServiceVersion ContextKey = "service_version"

	// AcceptLanguage indicates the natural language and locale that the client prefers.
	AcceptLanguage ContextKey = "accept-language"

	// XTzOffset contains the timezone offset from the client.
	XTzOffset ContextKey = "x-tz-offset"
)

// ExtractMetaFromContext extracts all metadata from the provided context.
// It retrieves values for all predefined context keys and returns them in a map.
// Only non-empty string values are included in the returned map.
func ExtractMetaFromContext(ctx context.Context) map[ContextKey]string {
	data := make(map[ContextKey]string)
	for _, k := range []ContextKey{
		TraceID,
		ActorType,
		ActorID,
		ServiceName,
		ServiceVersion,
		AcceptLanguage,
		XTzOffset,
	} {
		if v, ok := ctx.Value(k).(string); ok && v != "" {
			data[k] = v
		}
	}
	return data
}

// ShouldGetMeta retrieves a metadata value from the context by its key.
// It returns the value as a string if found, or an error if the key does not exist.
func ShouldGetMeta(ctx context.Context, key ContextKey) (string, error) {
	if value := ctx.Value(key); value != nil {
		if str, ok := value.(string); ok {
			return str, nil
		}
		return "", errx.New("[meta] type mismatch", errx.WithDetails(errx.D{"key": key, "value": value}))
	}
	return "", errx.New("[meta] key not found", errx.WithDetails(errx.D{"key": key}))
}
