// Package val provides validation functions for various data types and situations.
package val

import (
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate //nolint: gochecknoglobals // temporarily using a global variable until better solution is found

func init() { //nolint: gochecknoinits // temporarily using init function until better solution is found
	validate = validator.New()
	validate.RegisterTagNameFunc(getTagName)
}

// TODO: Implement RegisterCustomValidations function

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
