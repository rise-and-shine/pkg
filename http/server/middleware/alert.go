// Package middleware provides HTTP server middleware components.
package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/code19m/errx"
	"github.com/gofiber/fiber/v2"
	"github.com/rise-and-shine/pkg/http/server"
	"github.com/rise-and-shine/pkg/meta"
	"github.com/rise-and-shine/pkg/observability/alert"
	"github.com/rise-and-shine/pkg/observability/logger"
	"github.com/spf13/cast"
)

const (
	alertSendTimeout = 3 * time.Second
)

// NewAlertingMW creates a middleware that sends alerts for internal server errors.
//
// This middleware captures internal errors, extracts relevant metadata from the request
// context, and sends alerts through the provided alert.Provider. It only processes
// errors of type errx.T_Internal.
func NewAlertingMW() server.Middleware {
	return server.Middleware{
		Priority: 600,
		Handler: func(c *fiber.Ctx) error {
			ctx := c.UserContext()

			log := logger.Named("http.alerting").WithContext(ctx)

			err := c.Next()

			if err == nil {
				return nil
			}

			e := errx.AsErrorX(err)

			// only process internal errors
			if e.Type() != errx.T_Internal {
				return err
			}

			operation := fmt.Sprintf("%s %s", c.Method(), c.Route().Path)

			details := make(map[string]string)
			details["error_trace"] = e.Trace()

			metaCtx := meta.ExtractMetaFromContext(ctx)
			for k, v := range metaCtx {
				details[string(k)] = v
			}

			// get actor info from locals
			details["actor_type"] = cast.ToString(c.Locals(meta.ActorType))
			details["actor_id"] = cast.ToString(c.Locals(meta.ActorID))

			newCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), alertSendTimeout)

			go func() {
				defer cancel() // ensure newCtx is cancelled after sending alert

				sendErr := alert.SendError(newCtx, e.Code(), e.Error(), operation, details)
				if sendErr != nil {
					log.With("alert_send_error", sendErr.Error()).Warn("failed to send alert")
				}
			}()

			return err
		},
	}
}
