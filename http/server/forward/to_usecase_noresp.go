// Package forward provides helper functions for forwarding HTTP requests to use cases.
package forward

import (
	"context"

	"github.com/code19m/errx"
	"github.com/gofiber/fiber/v2"
	"github.com/rise-and-shine/pkg/val"
)

// useCaseMethodNoResp is a generic function type for a use case method that takes a request and returns no response.
type useCaseMethodNoResp[T_Req any] func(context.Context, T_Req) error

// ToUseCaseNoResp forwards a request to a use case that returns no response.
// It handles request decoding and validation.
// T_Req is the use case request type.
func ToUseCaseNoResp[T_Req any](uc useCaseMethodNoResp[T_Req]) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Initialize a new request of type T_Req
		req, err := newRequest[T_Req]()
		if err != nil {
			return errx.Wrap(err)
		}

		// Decode request body
		err = decodeBody(c, req)
		if err != nil {
			return errx.Wrap(err)
		}

		// Decode query params
		err = decodeQuery(c, req)
		if err != nil {
			return errx.Wrap(err)
		}

		// Decode path params
		err = decodePath(c, req)
		if err != nil {
			return errx.Wrap(err)
		}

		// Validate the request
		err = val.ValidateSchema(req)
		if err != nil {
			return errx.Wrap(err)
		}

		// Execute the use case
		err = uc(c.UserContext(), req)
		if err != nil {
			return errx.Wrap(err)
		}

		// Return a success response
		return nil
	}
}
