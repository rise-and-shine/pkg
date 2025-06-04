// Package middleware provides HTTP server middleware components.
package middleware

import (
	"errors"

	"github.com/code19m/errx"
	"github.com/gofiber/fiber/v2"
	"github.com/rise-and-shine/pkg/http/server"
	"github.com/rise-and-shine/pkg/meta"
)

// Common error codes used by the error handler middleware.
const (
	// codeRouterError is used when the router encounters an error.
	codeRouterError = "ROUTER_ERROR"
)

// NewErrorHandlerMW creates a middleware that handles errors and converts them to
// standardized JSON responses.
//
// This middleware catches any errors that occur during request processing and
// transforms them into a consistent JSON format. When debug is true, additional
// details like error trace and details are included in the response.
func NewErrorHandlerMW(debug bool) server.Middleware {
	return server.Middleware{
		Priority: 400,
		Handler: func(c *fiber.Ctx) error {
			err := c.Next()
			if err == nil {
				return nil
			}

			e := mapAnyErrorToErrorX(err)
			traceID := c.UserContext().Value(meta.TraceID)

			c.Status(mapErrorTypeToHTTPStatusCode(e.Type()))
			_ = c.JSON(map[string]any{
				"trace_id": traceID,
				"error":    buildErrorSchema(e, debug),
			})

			return err
		},
	}
}

// buildErrorSchema constructs an error response object from an ErrorX instance.
// When debug is true, includes trace and details information.
func buildErrorSchema(e errx.ErrorX, debug bool) errorSchema {
	errResp := errorSchema{
		Code:    e.Code(),
		Message: e.Error(),
		Fields:  e.Fields(),
	}
	if debug {
		errResp.Trace = e.Trace()
		errResp.Details = e.Details()
	}
	return errResp
}

// errorSchema defines the structure of error responses returned to clients.
type errorSchema struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Trace   string            `json:"trace,omitempty"`
	Fields  map[string]string `json:"fields,omitempty"`
	Details map[string]any    `json:"details,omitempty"`
}

// mapErrorTypeToHTTPStatusCode converts an errx.Type to the appropriate HTTP status code.
func mapErrorTypeToHTTPStatusCode(t errx.Type) int {
	switch t {
	case errx.T_Authentication:
		return fiber.StatusUnauthorized
	case errx.T_Forbidden:
		return fiber.StatusForbidden
	case errx.T_NotFound:
		return fiber.StatusNotFound
	case errx.T_Validation:
		return fiber.StatusBadRequest
	case errx.T_Conflict:
		return fiber.StatusConflict
	case errx.T_Internal:
		return fiber.StatusInternalServerError
	default:
		return fiber.StatusInternalServerError
	}
}

// mapAnyErrorToErrorX converts any error to an errx.ErrorX type.
// Special handling is provided for Fiber errors to map them to appropriate error types.
func mapAnyErrorToErrorX(err error) errx.ErrorX {
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		var t errx.Type

		switch {
		case fiberErr.Code == fiber.StatusUnauthorized:
			t = errx.T_Authentication
		case fiberErr.Code == fiber.StatusForbidden:
			t = errx.T_Forbidden
		case fiberErr.Code == fiber.StatusNotFound:
			t = errx.T_NotFound
		case fiberErr.Code == fiber.StatusConflict:
			t = errx.T_Conflict
		case fiberErr.Code >= 400 && fiberErr.Code < 500:
			t = errx.T_Validation
		default:
			t = errx.T_Internal
		}

		err = errx.New(
			fiberErr.Message,
			errx.WithCode(codeRouterError),
			errx.WithType(t),
		)
	}

	return errx.AsErrorX(err)
}
