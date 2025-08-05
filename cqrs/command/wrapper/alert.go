package wrapper

import (
	"context"
	"fmt"
	"time"

	"github.com/code19m/errx"
	"github.com/rise-and-shine/pkg/alert"
	"github.com/rise-and-shine/pkg/cqrs/command"
	"github.com/rise-and-shine/pkg/logger"
	"github.com/rise-and-shine/pkg/meta"
)

const (
	alertTimeout = 3 * time.Second
)

type AlertCommandWrapper[I command.Input, R command.Result] struct {
	logger        logger.Logger
	alertProvider alert.Provider
	next          command.Command[I, R]
	cmdName       string
}

func NewAlertCommandWrapper[I command.Input, R command.Result](
	logger logger.Logger,
	alertProvider alert.Provider,
	cmdName string,
) command.WrapFunc[I, R] {
	return func(next command.Command[I, R]) command.Command[I, R] {
		return &AlertCommandWrapper[I, R]{
			logger:        logger.Named("cqrs.command.alerting"),
			alertProvider: alertProvider,
			next:          next,
			cmdName:       cmdName,
		}
	}
}

func (cmd *AlertCommandWrapper[I, R]) Execute(ctx context.Context, input I) (R, error) {
	// Call the next command in the chain
	result, err := cmd.next.Execute(ctx, input)
	if err == nil {
		return result, nil
	}

	e := errx.AsErrorX(err)

	operation := fmt.Sprintf("command: %s", cmd.cmdName)
	details := make(map[string]string)
	metaCtx := meta.ExtractMetaFromContext(ctx)
	for k, v := range metaCtx {
		details[string(k)] = v
	}

	newCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), alertTimeout)

	go func() {
		defer cancel() // ensure newCtx is cancelled after sending alert

		sendErr := cmd.alertProvider.SendError(newCtx, e.Code(), err.Error(), operation, details)
		if sendErr != nil {
			cmd.logger.With("alert_send_error", sendErr).Warn("failed to send error alert")
		}
	}()

	return result, nil
}
