// Package middleware provides HTTP server middleware components.
package middleware

import (
	"runtime"
	"time"

	"github.com/code19m/errx"
	"github.com/gofiber/fiber/v2"
	"github.com/rise-and-shine/pkg/http/server"
	"github.com/rise-and-shine/pkg/logger"
	"github.com/rise-and-shine/pkg/mask"
	"github.com/rise-and-shine/pkg/meta"
)

// NewLoggerMW creates a middleware that logs HTTP requests and responses.
//
// This middleware captures comprehensive information about each request including
// method, path, status code, duration, and user context. It also logs errors with
// detailed information when they occur. The logging level is determined by the
// HTTP status code (info for 2xx/3xx, warn for 4xx, error for 5xx).
func NewLoggerMW(debug bool) server.Middleware {
	return server.Middleware{
		Priority: 500,
		Handler: func(c *fiber.Ctx) error {
			ctx := c.UserContext()
			start := time.Now()

			// handle request
			err := handleWithRecovery(c, debug)

			statusCode := c.Response().StatusCode()

			logger := logger.Named("http.access_logger").WithContext(ctx)

			logger = withSafeHeaders(c, logger)

			logger = logger.With(
				"hostname", c.Hostname(),
				"ip_addr", c.IP(),
				"remote_addr", c.Context().RemoteAddr().String(),
				"http_status_code", statusCode,
				"http_protocol", c.Protocol(),
				"http_method", c.Method(),
				"http_path", c.Path(),
				"http_route", c.Route().Path,
				"request_size", c.Request().Header.ContentLength(),
				"duration", time.Since(start),
				"query_params", c.Queries(),
			)

			// get user data from locals which should be set by authentication middleware
			logger = logger.With(
				"actor_type", c.Locals(meta.ActorType),
				"actor_id", c.Locals(meta.ActorID),
			)

			logger = withSafeRequestResponse(c, logger, debug)

			switch {
			case statusCode >= 500:
				logger.Errorx(err)
			case statusCode >= 400:
				logger.Warnx(err)
			default:
				logger.Info("request processed successfully")
			}

			return err
		},
	}
}

// handleWithRecovery executes the next middleware and recovers from panics.
// It returns any error from the middleware chain or a new error if a panic occurred.
func handleWithRecovery(c *fiber.Ctx, debug bool) (err error) {
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

			// if error already handled, skip processing.
			if c.Response() != nil && c.Response().StatusCode() >= 400 {
				return
			}

			err = server.WriteErrorResponse(c, err, debug)
		}
	}()

	return c.Next()
}

// withSafeHeaders returns a logger with safe to log headers from the request.
func withSafeHeaders(c *fiber.Ctx, logger logger.Logger) logger.Logger {
	safeHeaders := []string{
		"referer",
		"user-agent",
		"content-type",
		"x-client-app-name",
		"x-client-app-os",
		"x-client-app-version",
	}

	for _, h := range safeHeaders {
		v := c.Get(h)
		if v != "" {
			logger = logger.With(h, v)
		}
	}

	return logger
}

// withSafeRequestResponse returns a logger with safe to log request and response body.
func withSafeRequestResponse(c *fiber.Ctx, logger logger.Logger, debug bool) logger.Logger {
	if !debug {
		return logger
	}

	if c.Method() != fiber.MethodPost {
		return logger
	}

	// get request and response body from locals
	// which should be set by handlers
	req := c.Locals("request_body")
	resp := c.Locals("response_body")

	if req != nil {
		logger = logger.With("request_body", mask.StructToOrdMap(req)) // TODO: watch for log entry format
	}

	if resp != nil {
		logger = logger.With("response_body", mask.StructToOrdMap(resp)) // TODO: watch for log entry format
	}

	return logger
}
