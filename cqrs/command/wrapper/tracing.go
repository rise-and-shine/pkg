package wrapper

import (
	"context"
	"fmt"
	"strings"

	"github.com/rise-and-shine/pkg/cqrs/command"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type TracingCommandWrapper[I command.Input, R command.Result] struct {
	tracer   trace.Tracer
	spanName string
	next     command.Command[I, R]
}

func NewTracingCommandWrapper[I command.Input, R command.Result]() command.WrapFunc[I, R] {
	return func(next command.Command[I, R]) command.Command[I, R] {
		tracer := otel.Tracer("cqrs/command")
		spanName := getSpanNameFromCmd(next)

		return &TracingCommandWrapper[I, R]{
			tracer:   tracer,
			spanName: spanName,
			next:     next,
		}
	}
}

func (t *TracingCommandWrapper[I, R]) Execute(ctx context.Context, input I) (R, error) {
	// Start a new span
	ctx, span := t.tracer.Start(ctx, t.spanName)
	defer span.End()

	// Call the next command in the chain
	result, err := t.next.Execute(ctx, input)

	// End the span
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	return result, err
}

func getSpanNameFromCmd(cmd any) string {
	fullType := fmt.Sprintf("%T", cmd)

	fullType = strings.TrimPrefix(fullType, "*")

	parts := strings.Split(fullType, ".")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}

	return fullType
}
