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
			fields[fieldErr.Field()] = getFieldErrDescription(fieldErr)
		}

		return errx.New(
			"Validation failed with field errors. See fields for details.",
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
	// Bobur aka boshqa taglar uchun ham chunarli description yozberin, playground packagedagi eng kop ishlatiladiganlari uchun...
	default:
		return fmt.Sprintf("Failed validation on tag: %s. Param: %s", fieldErr.Tag(), fieldErr.Param())
	}
}
