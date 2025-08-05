package wrapper

import (
	"context"

	"github.com/rise-and-shine/pkg/cqrs/command"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type TracingCommandWrapper[I command.Input, R command.Result] struct {
	tracer  trace.Tracer
	next    command.Command[I, R]
	cmdName string
}

func NewTracingCommandWrapper[I command.Input, R command.Result](cmdName string) command.WrapFunc[I, R] {
	return func(next command.Command[I, R]) command.Command[I, R] {
		tracer := otel.Tracer("cqrs/command")

		return &TracingCommandWrapper[I, R]{
			tracer:  tracer,
			cmdName: cmdName,
			next:    next,
		}
	}
}

func (cmd *TracingCommandWrapper[I, R]) Execute(ctx context.Context, input I) (R, error) {
	// Start a new span
	ctx, span := cmd.tracer.Start(ctx, cmd.cmdName)
	defer span.End()

	// Call the next command in the chain
	result, err := cmd.next.Execute(ctx, input)

	// End the span
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	return result, err
}
