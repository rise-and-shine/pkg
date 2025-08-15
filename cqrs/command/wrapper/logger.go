package wrapper

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/code19m/errx"
	"github.com/rise-and-shine/pkg/cqrs/command"
	"github.com/rise-and-shine/pkg/logger"
)

type LoggerCommandWrapper[I command.Input, R command.Result] struct {
	logger  logger.Logger
	next    command.Command[I, R]
	cmdName string
}

func NewLoggerCommandWrapper[I command.Input, R command.Result](
	logger logger.Logger,
	cmdName string,
) command.WrapFunc[I, R] {
	return func(next command.Command[I, R]) command.Command[I, R] {
		return &LoggerCommandWrapper[I, R]{
			logger:  logger.Named("cqrs.command.logger").With("command_name", cmdName),
			next:    next,
			cmdName: cmdName,
		}
	}
}

func (cmd *LoggerCommandWrapper[I, R]) Execute(ctx context.Context, input I) (R, error) {
	start := time.Now()

	result, err := executeWithRecovery(ctx, cmd.next, input)

	duration := time.Since(start)

	logger := cmd.logger.
		WithContext(ctx).
		With("command_name", cmd.cmdName).
		With("execution_time", duration.String()).
		With("input", input)

	if err != nil {
		e := errx.AsErrorX(err)
		logger.With("error", map[string]any{
			"code":    e.Code(),
			"message": e.Error(),
			"type":    e.Type().String(),
			"trace":   e.Trace(),
			"fields":  e.Fields(),
			"details": e.Details(),
		}).Error()
	} else {
		logger.Info()
	}

	return result, err
}

func executeWithRecovery[I command.Input, R command.Result](
	ctx context.Context,
	cmd command.Command[I, R],
	input I,
) (_ R, err error) {
	defer func() {
		if r := recover(); r != nil {
			stackTrace := make([]byte, 4096) // 4KB
			stackTrace = stackTrace[:runtime.Stack(stackTrace, false)]
			err = errx.New("panic recovered in logger command wrapper", errx.WithDetails(errx.D{
				"stack_trace":  string(stackTrace),
				"panic_values": fmt.Sprintf("%v", r),
			}))
		}
	}()

	return cmd.Execute(ctx, input)
}
