package val

import (
	"errors"
	"fmt"

	"github.com/code19m/errx"
	"github.com/go-playground/validator/v10"
)

// ValidateSchema validates a given schema using the go-playground/validator package.
func ValidateSchema(schema any) error {
	err := validate.Struct(schema)

	if err == nil {
		return nil
	}

	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		fields := make(errx.M)

		for _, fieldErr := range validationErrors {
			field := fieldErr.Field()
			fields[field] = getFieldErrDescription(fieldErr)
		}

		return errx.New(
			"Validation failed. See fields for details.",
			errx.WithCode(CodeInvalidInput),
			errx.WithType(errx.T_Validation),
			errx.WithFields(fields),
		)
	}
	return errx.New(
		fmt.Sprintf("Unknown validation error: %s", err.Error()),
		errx.WithCode(CodeInvalidInput),
		errx.WithType(errx.T_Validation),
	)
}

func getFieldErrDescription(fieldErr validator.FieldError) string {
	switch fieldErr.Tag() {
	case "required":
		return "This field is required"
	case "email":
		return "Invalid email format"
	case "min":
		return fmt.Sprintf("Must be at least %s characters", fieldErr.Param())
	case "max":
		return fmt.Sprintf("Must be at most %s characters", fieldErr.Param())
	case "gte":
		return fmt.Sprintf("Must be at least %s", fieldErr.Param())
	case "lte":
		return fmt.Sprintf("Must be at most %s", fieldErr.Param())
	case "gt":
		return fmt.Sprintf("Must be greater than %s", fieldErr.Param())
	case "lt":
		return fmt.Sprintf("Must be less than %s", fieldErr.Param())
	case "len":
		return fmt.Sprintf("Must be exactly %s characters", fieldErr.Param())
	case "alpha":
		return "Must contain only alphabetic characters"
	case "alphanum":
		return "Must contain only alphanumeric characters"
	case "numeric":
		return "Must be a valid number"
	case "url":
		return "Must be a valid URL"
	case "uri":
		return "Must be a valid URI"
	case "uuid":
		return "Must be a valid UUID"
	case "oneof":
		return fmt.Sprintf("Must be one of: %s", fieldErr.Param())
	default:
		return fmt.Sprintf("Failed validation: %s", fieldErr.Tag())
	}
}
