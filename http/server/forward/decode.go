// Package forward provides helper functions for forwarding HTTP requests to use cases.
package forward

import (
	"mime"

	"github.com/code19m/errx"
	"github.com/gofiber/fiber/v2"
)

// decodeBody decodes the request body into the given request struct.
// It only decodes if the content type is application/json.
func decodeBody[I any](c *fiber.Ctx, req I) error {
	if len(c.Body()) == 0 {
		return nil // No body to decode
	}

	mediaType, _, err := mime.ParseMediaType(c.Get(fiber.HeaderContentType))
	if err != nil || mediaType != fiber.MIMEApplicationJSON {
		return errx.New(
			"content type must be application/json for this request",
			errx.WithType(errx.T_Validation),
			errx.WithCode(codeInvalidContentType),
		)
	}

	err = c.BodyParser(req)
	if err != nil {
		return errx.Wrap(
			err,
			errx.WithType(errx.T_Validation),
			errx.WithCode(codeInvalidJSONBody),
		)
	}

	return nil
}

// decodeQuery decodes the query params into the given request struct.
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
