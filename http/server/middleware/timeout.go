// Package middleware provides HTTP server middleware components.
package middleware

import (
	"context"
	"time"

	"github.com/rise-and-shine/pkg/http/server"
	"github.com/gofiber/fiber/v2"
)

// NewTimeoutMW creates a middleware that applies a timeout to the request context.
//
// This middleware injects a context with a timeout into the Fiber context, ensuring
// that all downstream operations that respect context cancellation will abort after
// the specified duration. This helps prevent long-running requests from consuming
// server resources indefinitely.
func NewTimeoutMW(duration time.Duration) server.Middleware {
	return server.Middleware{
		Priority: 800,
		Handler: func(c *fiber.Ctx) error {
			ctx, cancel := context.WithTimeout(c.UserContext(), duration)
			defer cancel()

			c.SetUserContext(ctx)

			return c.Next()
		},
	}
}
