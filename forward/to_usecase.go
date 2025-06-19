package forward

import (
	"context"

	"github.com/code19m/errx"
	"github.com/gofiber/fiber/v2"
	"github.com/rise-and-shine/pkg/val"
)

type useCaseMethod[T_Req any, T_Resp any] func(context.Context, T_Req) (T_Resp, error)

func ToUseCase[T_Req any, T_Resp any](uc useCaseMethod[T_Req, T_Resp]) fiber.Handler {
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
		resp, err := uc(c.UserContext(), req)
		if err != nil {
			return errx.Wrap(err)
		}

		// Return the response
		return errx.Wrap(c.JSON(resp))
	}
}
