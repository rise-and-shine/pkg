package wrapper

import (
	"context"
	"time"

	"github.com/rise-and-shine/pkg/cqrs/command"
)

type TimeoutCommandWrapper[I command.Input, R command.Result] struct {
	timeout time.Duration
	next    command.Command[I, R]
}

func NewTimeoutCommandWrapper[I command.Input, R command.Result](timeout time.Duration) command.WrapFunc[I, R] {
	return func(next command.Command[I, R]) command.Command[I, R] {
		return &TimeoutCommandWrapper[I, R]{timeout: timeout, next: next}
	}
}

func (cmd *TimeoutCommandWrapper[I, R]) Execute(ctx context.Context, input I) (R, error) {
	ctx, cancel := context.WithTimeout(ctx, cmd.timeout)
	defer cancel()

	return cmd.next.Execute(ctx, input)
}
