// Package middleware provides HTTP server middleware components.
package middleware

import (
	"runtime"
	"time"

	"github.com/code19m/errx"
	"github.com/code19m/pkg/http/server"
	"github.com/code19m/pkg/logger"
	"github.com/code19m/pkg/meta"
	"github.com/gofiber/fiber/v2"
)

// NewLoggerMW creates a middleware that logs HTTP requests and responses.
//
// This middleware captures comprehensive information about each request including
// method, path, status code, duration, and user context. It also logs errors with
// detailed information when they occur. The logging level is determined by the
// HTTP status code (info for 2xx/3xx, warn for 4xx, error for 5xx).
func NewLoggerMW(log logger.Logger) server.Middleware {
	return server.Middleware{
		Priority: 500,
		Handler: func(c *fiber.Ctx) error {
			ctx := c.UserContext()

			log = log.Named("middleware.logger").WithContext(ctx)

			start := time.Now()

			err := handleWithRecovery(c)

			statusCode := c.Response().StatusCode()

			log = log.
				With("http_status_code", statusCode).
				With("http_schema", c.Protocol()).
				With("http_method", c.Method()).
				With("http_path", c.Path()).
				With("http_route", c.Route().Path).
				With("hostname", c.Hostname()).
				With("duration", time.Since(start)).
				With("query_params", c.Queries()).
				With("request_size", c.Request().Header.ContentLength())

			// Get user data from locals
			log = log.
				With("request_user_id", c.Locals(meta.RequestUserID)).
				With("request_user_type", c.Locals(meta.RequestUserType)).
				With("request_user_role", c.Locals(meta.RequestUserRole))

			if err != nil {
				e := errx.AsErrorX(err)
				log = log.With("error", map[string]any{
					"code":    e.Code(),
					"message": e.Error(),
					"type":    e.Type().String(),
					"trace":   e.Trace(),
					"fields":  e.Fields(),
					"details": e.Details(),
				})
			}

			switch {
			case statusCode >= 500:
				log.Error(err)
			case statusCode >= 400:
				log.Warn(err)
			default:
				log.Info("request processed successfully")
			}

			return err
		},
	}
}

// handleWithRecovery executes the next middleware and recovers from panics.
// It returns any error from the middleware chain or a new error if a panic occurred.
func handleWithRecovery(c *fiber.Ctx) (err error) {
	defer func() {
		if r := recover(); r != nil {
			traceSize := 4096 // 4KB
			stackTrace := make([]byte, traceSize)
			stackTrace = stackTrace[:runtime.Stack(stackTrace, false)]

			err = errx.New(
				"panic recovered at logger middleware",
				errx.WithDetails(errx.D{
					"stack_trace":   string(stackTrace),
					"panic_message": r,
				}),
			)
		}
	}()

	return c.Next()
}
