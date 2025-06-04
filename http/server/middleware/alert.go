// Package middleware provides HTTP server middleware components.
package middleware

import (
	"fmt"

	"github.com/code19m/errx"
	"github.com/gofiber/fiber/v2"
	"github.com/rise-and-shine/pkg/alert"
	"github.com/rise-and-shine/pkg/http/server"
	"github.com/rise-and-shine/pkg/logger"
	"github.com/rise-and-shine/pkg/meta"
	"github.com/spf13/cast"
)

// NewAlertingMW creates a middleware that sends alerts for internal server errors.
//
// This middleware captures internal errors, extracts relevant metadata from the request
// context, and sends alerts through the provided alert.Provider. It only processes
// errors of type errx.T_Internal.
func NewAlertingMW(log logger.Logger, provider alert.Provider) server.Middleware {
	return server.Middleware{
		Priority: 600,
		Handler: func(c *fiber.Ctx) error {
			ctx := c.UserContext()

			log = log.Named("middleware.alerting").WithContext(ctx)

			err := c.Next()

			if err == nil {
				return nil
			}

			e := errx.AsErrorX(err)

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

			// get user data from locals
			details["request_user_id"] = cast.ToString(c.Locals(meta.RequestUserID))
			details["request_user_type"] = cast.ToString(c.Locals(meta.RequestUserType))
			details["request_user_role"] = cast.ToString(c.Locals(meta.RequestUserRole))

			sendErr := provider.SendError(ctx, e.Code(), e.Error(), operation, details)
			if sendErr != nil {
				log.With("alert_send_error", sendErr).Warn("failed to send alert")
			}

			return err
		},
	}
}
