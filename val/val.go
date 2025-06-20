// Package val provides validation functions for various data types and situations.
package val

import (
	"reflect"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
)

var (
	validate *validator.Validate //nolint: gochecknoglobals // singleton validator instance
	once     sync.Once           //nolint: gochecknoglobals // ensures validator is initialized once
)

// RegisterCustomValidation registers a custom validation function for a specific tag.
func RegisterCustomValidation(tag string, fn validator.Func) error {
	return getValidator().RegisterValidation(tag, fn)
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
	// Register phone_uz validation
	_ = validate.RegisterValidation("phone_uz", validatePhoneUz)

	// Register strong_password validation
	_ = validate.RegisterValidation("strong_password", validateStrongPassword)
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
