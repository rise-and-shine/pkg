package server

import (
	"errors"

	"github.com/code19m/errx"
	"github.com/gofiber/fiber/v2"
	"github.com/rise-and-shine/pkg/meta"
)

const (
	// codeRouterError is used when the router encounters an error.
	codeRouterError = "ROUTER_ERROR"
)

// WriteErrorResponse writes a standardized error response to the Fiber context.
func WriteErrorResponse(c *fiber.Ctx, err error, hideDetails bool) error {
	e := mapAnyErrorToErrorX(err)
	traceID := c.UserContext().Value(meta.TraceID)
	lang := c.Get("accept-language")

	c.Status(mapErrorTypeToHTTPStatusCode(e.Type()))
	_ = c.JSON(map[string]any{
		"trace_id": traceID,
		"error":    buildErrorSchema(e, hideDetails, lang),
	})

	return e
}

// customErrorHandler returns a Fiber error handler that ensures consistent error responses.
//
// If the response status code is already set to an error (>= 400), it does not override it.
func customErrorHandler(hideDetails bool) fiber.ErrorHandler {
	return func(ctx *fiber.Ctx, err error) error {
		r := ctx.Response()

		// if error already handled, skip processing by returning nil
		if r != nil && r.StatusCode() >= 400 {
			return nil
		}

		_ = WriteErrorResponse(ctx, err, hideDetails)
		return nil
	}
}

// buildErrorSchema constructs an error response object from an ErrorX instance.
// When hideDetails is false, includes trace and details information.
func buildErrorSchema(e errx.ErrorX, hideDetails bool, lang string) errorSchema {
	errResp := errorSchema{
		Code:    e.Code(),
		Message: meta.Tr(e.Code(), lang),
		Cause:   e.Error(),
		Fields:  e.Fields(),
	}
	if !hideDetails {
		errResp.Trace = e.Trace()
		errResp.Details = e.Details()
	}
	return errResp
}

// errorSchema defines the structure of error responses returned to clients.
type errorSchema struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Cause   string            `json:"cause"`
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
	case errx.T_Throttling:
		return fiber.StatusTooManyRequests
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
		case fiberErr.Code == fiber.StatusTooManyRequests:
			t = errx.T_Throttling
		case fiberErr.Code >= 400 && fiberErr.Code < 500:
			t = errx.T_Validation
		default:
			t = errx.T_Internal
		}

		err = errx.New(
			fiberErr.Message,
			errx.WithCode(codeRouterError),
			errx.WithType(t),
			errx.WithDetails(errx.D{
				"fiber_code": fiberErr.Code,
				"fiber_msg":  fiberErr.Message,
			}),
		)
	}

	return errx.AsErrorX(err)
}
