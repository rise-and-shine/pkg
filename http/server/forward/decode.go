package forward

import (
	"github.com/code19m/errx"
	"github.com/gofiber/fiber/v2"
)

func decodeBody[T_Req any](c *fiber.Ctx, req T_Req) error {
	if !isJSONMethod(c.Method()) {
		return nil // No body to decode for non-JSON methods
	}

	if len(c.Body()) == 0 {
		return nil // No body to decode
	}

	if c.Get(fiber.HeaderContentType) != fiber.MIMEApplicationJSON {
		return errx.New(
			"only application/json content type is supported for POST, PUT, PATCH methods when using ToUseCase forwarder",
			errx.WithType(errx.T_Validation),
			errx.WithCode(codeInvalidContentType),
		)
	}

	if err := c.BodyParser(req); err != nil {
		return errx.Wrap(
			err,
			errx.WithType(errx.T_Validation),
			errx.WithCode(codeInvalidJSONBody),
		)
	}

	return nil
}

func decodeQuery[T_Req any](c *fiber.Ctx, req T_Req) error {
	if len(c.Queries()) == 0 {
		return nil // No query params to decode
	}

	if err := c.QueryParser(req); err != nil {
		return errx.Wrap(
			err,
			errx.WithType(errx.T_Validation),
			errx.WithCode(codeInvalidQueryParams),
		)
	}

	return nil
}

func decodePath[T_Req any](c *fiber.Ctx, req T_Req) error {
	if len(c.Route().Params) == 0 {
		return nil // No path params to decode
	}

	if err := c.ParamsParser(req); err != nil {
		return errx.Wrap(
			err,
			errx.WithType(errx.T_Validation),
			errx.WithCode(codeInvalidPathParams),
		)
	}

	return nil
}
