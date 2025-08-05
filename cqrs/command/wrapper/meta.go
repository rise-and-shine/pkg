package wrapper

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rise-and-shine/pkg/cqrs/command"
	"github.com/rise-and-shine/pkg/meta"
	"go.opentelemetry.io/otel/trace"
)

type MetaInjectCommandWrapper[I command.Input, R command.Result] struct {
	serviceName    string
	serviceVersion string
	next           command.Command[I, R]
}

func NewMetaInjectCommandWrapper[I command.Input, R command.Result](
	serviceName, serviceVersion string,
) command.WrapFunc[I, R] {
	return func(next command.Command[I, R]) command.Command[I, R] {
		return &MetaInjectCommandWrapper[I, R]{serviceName: serviceName, serviceVersion: serviceVersion, next: next}
	}
}

func (cmd *MetaInjectCommandWrapper[I, R]) Execute(ctx context.Context, input I) (R, error) {
	metadata := map[meta.ContextKey]string{ //nolint:exhaustive // we are not using all keys
		meta.TraceID:        getTraceID(ctx),
		meta.ServiceName:    cmd.serviceName,
		meta.ServiceVersion: cmd.serviceVersion,
	}

	// add meta to context for downstream chain
	ctx = meta.InjectMetaToContext(ctx, metadata)

	// Call the next command in the chain
	return cmd.next.Execute(ctx, input)
}

// getTraceID extracts the trace ID from the current span in the context.
// If no trace ID is available, it generates a new UUID to use as a trace ID.
func getTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	traceID := span.SpanContext().TraceID()

	if traceID.IsValid() {
		return traceID.String()
	}

	return fmt.Sprintf("man-%s", uuid.New().String())
}
