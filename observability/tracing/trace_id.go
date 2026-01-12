package tracing

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

// GetStartingTraceID returns the trace ID for a new trace for the given context.
// If no trace ID is available in the context, it generates a new UUID to use as a trace ID.
// For example: if otel tracing is not initialized manual tracing will be used for correlation of logs.
func GetStartingTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	traceID := span.SpanContext().TraceID()

	if traceID.IsValid() {
		return traceID.String()
	}

	return fmt.Sprintf("man-%s", uuid.New().String())
}
