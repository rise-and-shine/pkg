// Package middleware provides HTTP server middleware components.
package middleware

import (
	"runtime"

	"github.com/code19m/errx"
	"github.com/gofiber/fiber/v2"
	"github.com/rise-and-shine/pkg/http/server"
	"github.com/rise-and-shine/pkg/observability/logger"
)

// NewRecoveryMW creates a middleware that recovers from panics in the request
// handling chain and converts them to structured errors.
//
// This middleware serves as a safety net to catch any unexpected panics that may occur
// during request processing. It captures the stack trace and panic message, logs the
// information, and returns a structured error that can be handled by error middleware.
func NewRecoveryMW(logger logger.Logger) server.Middleware {
	return server.Middleware{
		Priority: 1000,
		Handler: func(c *fiber.Ctx) (err error) {
			log := logger.Named("middleware.recovery").WithContext(c.UserContext())

			defer func() {
				if r := recover(); r != nil {
					traceSize := 4096 // 4KB
					stackTrace := make([]byte, traceSize)
					stackTrace = stackTrace[:runtime.Stack(stackTrace, false)]

					log.
						With("stack_trace", string(stackTrace)).
						With("panic_message", r).
						Error("recovered from panic")

					err = errx.New("panic recovered", errx.WithDetails(errx.D{
						"stack_trace":   string(stackTrace),
						"panic_message": r,
					}))
				}
			}()

			return c.Next()
		},
	}
}
