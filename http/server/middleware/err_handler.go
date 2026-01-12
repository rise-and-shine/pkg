// Package middleware provides HTTP server middleware components.
package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rise-and-shine/pkg/http/server"
)

// NewErrorHandlerMW creates a middleware that handles errors and converts them to
// standardized JSON responses.
//
// This middleware catches any errors that occur during request processing and
// transforms them into a consistent JSON format. When hideDetails is false, additional
// details like error trace and details are included in the response.
func NewErrorHandlerMW(hideDetails bool) server.Middleware {
	return server.Middleware{
		Priority: 400,
		Handler: func(c *fiber.Ctx) error {
			err := c.Next()
			if err == nil {
				return nil
			}

			// if error already handled, skip processing.
			if c.Response() != nil && c.Response().StatusCode() >= 400 {
				return err
			}

			return server.WriteErrorResponse(c, err, hideDetails)
		},
	}
}
