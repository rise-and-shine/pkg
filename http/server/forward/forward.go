// Package forward provides helper functions for forwarding HTTP requests to use cases.
package forward

import (
	"fmt"
	"reflect"

	"github.com/code19m/errx"
	"github.com/gofiber/fiber/v2"
	"github.com/rise-and-shine/pkg/mask"
	"github.com/rise-and-shine/pkg/observability/logger"
	"github.com/rise-and-shine/pkg/ucdef"
	"github.com/rise-and-shine/pkg/val"
)

const maxLogAllowedSize = 8 << 10 // 8KB

// ToUserAction forwards a request to a use case that returns a response.
// It handles request decoding, validation, and response encoding.
// I is the use case request type.
// O is the use case response type.
func ToUserAction[I, O any](uc ucdef.UserAction[I, O]) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Initialize a new request of type T_Req
		req, err := newRequest[I]()
		if err != nil {
			return errx.Wrap(err)
		}

		// Decode the request based on the HTTP method
		switch c.Method() {
		case fiber.MethodGet:
			err = decodeQuery(c, req)
			if err != nil {
				return errx.Wrap(err)
			}

		case fiber.MethodPost:
			err = decodeBody(c, req)
			if err != nil {
				return errx.Wrap(err)
			}

		default:
			return errx.New(
				"unsupported http method: allowed only GET and POST",
				errx.WithType(errx.T_Validation),
				errx.WithCode(codeInvalidHTTPMethod),
				errx.WithDetails(errx.D{
					"received_http_method": c.Method(),
				}),
			)
		}

		log := logger.
			Named("http.handler").
			WithContext(c.UserContext()).
			With("operation_id", uc.OperationID())

		// Include request body in log if it's size is not too large
		if len(c.Body()) <= maxLogAllowedSize {
			log = log.With("request_body", mask.StructToOrdMap(req))
		} else {
			log = log.With("request_body", fmt.Sprintf("too large for logging: %d bytes", len(c.Body())))
		}

		// Validate the request schema based on validate tags of the struct
		err = val.ValidateSchema(req)
		if err != nil {
			log.Errorx(err)
			return errx.Wrap(err)
		}

		// Execute the use case
		resp, err := uc.Execute(c.UserContext(), req)
		if err != nil {
			log.Errorx(err)
			return errx.Wrap(err)
		}

		// Write the success response
		size, err := writeJSON(c, resp)
		if err != nil {
			log.Errorx(err)
			return errx.Wrap(err)
		}

		// Include response body in log if it's size is not too large
		if size <= maxLogAllowedSize {
			log = log.With("response_body", mask.StructToOrdMap(resp))
		} else {
			log = log.With("response_body", fmt.Sprintf("too large for logging: %d bytes", size))
		}

		log.Debug("")
		return nil
	}
}

// newRequest creates a new request of type I.
// It ensures that I is a pointer to a struct.
func newRequest[I any]() (I, error) {
	var req I

	reqType := reflect.TypeOf((*I)(nil)).Elem()
	if reqType.Kind() != reflect.Pointer || reqType.Elem().Kind() != reflect.Struct {
		return req, errx.New("input type I must be a pointer")
	}

	reqVal := reflect.New(reqType.Elem()).Interface().(I) //nolint:errcheck // safe type assertion
	return reqVal, nil
}

func writeJSON(c *fiber.Ctx, data any) (int, error) {
	raw, err := c.App().Config().JSONEncoder(data)
	if err != nil {
		return 0, errx.Wrap(err)
	}

	c.Response().SetBodyRaw(raw)
	c.Response().Header.SetContentType(fiber.MIMEApplicationJSON)
	return len(raw), nil
}
