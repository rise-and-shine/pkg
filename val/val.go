// Package val provides validation functions for various data types and situations.
package val

import (
	"reflect"
	"strings"
	"sync"

	"github.com/code19m/errx"
	"github.com/go-playground/validator/v10"
)

var (
	validate *validator.Validate //nolint: gochecknoglobals // temporarily using a global variable until better solution is found.
	once     sync.Once           //nolint: gochecknoglobals // temporarily using a global variable until better solution is found.
)

// RegisterCustomValidation registers a custom validation function for a specific tag.
func RegisterCustomValidation(tag string, fn validator.Func) error {
	// Ensure the validator instance is initialized
	return errx.Wrap(getValidator().RegisterValidation(tag, fn))
}

// getValidator returns the singleton validator instance with custom configurations.
func getValidator() *validator.Validate {
	once.Do(func() {
		validate = validator.New()
		validate.RegisterTagNameFunc(getTagName)
		registerCommonValidations()
	})
	return validate
}

// registerCommonValidations registers all custom validation functions.
func registerCommonValidations() {
	v := getValidator()

	// Register phone_uz validation
	_ = v.RegisterValidation("phone_uz", validatePhoneUz)

	// Register strong_password validation
	_ = v.RegisterValidation("strong_password", validateStrongPassword)

	// Add more custom validations as needed
	// validate.RegisterValidation("custom_tag", customValidationFunc)
}

// getTagName returns the name of a struct field based on its struct tags.
// It checks 'json', 'query', and 'params' tags in that order, and falls back
// to the field name if none of those tags have a non-empty name component.
func getTagName(fld reflect.StructField) string {
	// Try each tag in order: json, query, params
	for _, tagName := range []string{"json", "query", "params"} {
		name := strings.SplitN(fld.Tag.Get(tagName), ",", 2)[0]
		if name != "" {
			return name
		}
	}

	// Fall back to the actual field name
	return fld.Name
}
