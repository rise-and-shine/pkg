// Package wrapper provides middleware wrappers for CQRS query handlers.
//
// This package includes wrappers for cross-cutting concerns such as tracing.
// The TracingQueryWrapper enables OpenTelemetry tracing for query execution,
// automatically creating spans and recording errors for observability.
package wrapper

import (
	"context"
	"fmt"
	"strings"

	"github.com/code19m/pkg/cqrs/query"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// TracingQueryWrapper wraps a query handler with OpenTelemetry tracing.
//
// It starts a new span for each query execution, records errors, and sets span status.
// The span name is derived from the query type for better trace readability.
type TracingQueryWrapper[I query.Input, R query.Result] struct {
	tracer   trace.Tracer
	spanName string
	next     query.Query[I, R]
}

// NewTracingqueryWrapper returns a query.WrapFunc that wraps a query handler with tracing.
//
// The returned wrapper starts a new OpenTelemetry span for each query execution.
// Example:
//
//	wrapped := wrapper.NewTracingqueryWrapper[MyInput, MyResult]()
//	handler := wrapped(myQueryHandler)
func NewTracingqueryWrapper[I query.Input, R query.Result]() query.WrapFunc[I, R] {
	return func(next query.Query[I, R]) query.Query[I, R] {
		tracer := otel.Tracer("cqrs/query")
		spanName := getSpanNameFromCmd(next)

		return &TracingQueryWrapper[I, R]{
			tracer:   tracer,
			spanName: spanName,
			next:     next,
		}
	}
}

// Execute runs the wrapped query handler with tracing.
//
// It starts a new span, calls the next handler, and records any errors on the span.
// The context is propagated to the next handler.
//
// Parameters:
//   - ctx: The context for span propagation and cancellation.
//   - input: The query input.
//
// Returns the query result and error, if any. Errors are recorded on the span.
func (t *TracingQueryWrapper[I, R]) Execute(ctx context.Context, input I) (R, error) {
	// Start a new span
	ctx, span := t.tracer.Start(ctx, t.spanName)
	defer span.End()

	// Call the next query in the chain
	result, err := t.next.Execute(ctx, input)

	// End the span
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	return result, err
}

// getSpanNameFromCmd returns a span name based on the command type.
//
// It extracts the type name from the command for use as the span name in traces.
func getSpanNameFromCmd(cmd any) string {
	fullType := fmt.Sprintf("%T", cmd)

	fullType = strings.TrimPrefix(fullType, "*")

	parts := strings.Split(fullType, ".")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}

	return fullType
}
