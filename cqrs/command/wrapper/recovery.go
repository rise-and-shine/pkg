package wrapper

import (
	"context"
	"fmt"
	"runtime"

	"github.com/code19m/errx"
	"github.com/rise-and-shine/pkg/cqrs/command"
	"github.com/rise-and-shine/pkg/logger"
)

type RecoveryCommandWrapper[I command.Input, R command.Result] struct {
	logger  logger.Logger
	next    command.Command[I, R]
	cmdName string
}

func NewRecoveryCommandWrapper[I command.Input, R command.Result](
	logger logger.Logger,
	cmdName string,
) command.WrapFunc[I, R] {
	return func(next command.Command[I, R]) command.Command[I, R] {
		return &RecoveryCommandWrapper[I, R]{
			logger:  logger.Named("cqrs.command.recovery").With("command_name", cmdName),
			next:    next,
			cmdName: cmdName,
		}
	}
}

func (cmd *RecoveryCommandWrapper[I, R]) Execute(ctx context.Context, input I) (R, error) {
	var result R
	var err error

	defer func() {
		if r := recover(); r != nil {
			stackTrace := make([]byte, 4096) // 4KB
			stackTrace = stackTrace[:runtime.Stack(stackTrace, false)]

			cmd.logger.
				WithContext(ctx).
				With("stack_trace", string(stackTrace)).
				With("panic_values", fmt.Sprintf("%v", r)).
				Error("panic recovered in recovery wrapper")

			err = errx.New("panic recovered in recovery wrapper", errx.WithDetails(errx.D{
				"stack_trace":  string(stackTrace),
				"panic_values": fmt.Sprintf("%v", r),
			}))
		}
	}()

	result, err = cmd.next.Execute(ctx, input)
	return result, err
}
