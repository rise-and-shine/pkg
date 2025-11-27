package val

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/code19m/errx"
	"github.com/go-playground/validator/v10"
)

const (
	CodeValidationFailed = "VALIDATION_FAILED"
)

// ValidateSchema validates a given schema using the go-playground/validator package.
func ValidateSchema(schema any) error {
	err := getValidator().Struct(schema)

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
			errx.WithCode(CodeValidationFailed),
			errx.WithType(errx.T_Validation),
			errx.WithFields(fields),
		)
	}
	return errx.New(
		fmt.Sprintf("Unknown validation error: %s", err.Error()),
		errx.WithCode(CodeValidationFailed),
		errx.WithType(errx.T_Validation),
	)
}

func getFieldErrDescription(fieldErr validator.FieldError) string {
	param := fieldErr.Param()
	tag := fieldErr.Tag()

	if desc := getBasicValidationDesc(tag, param, fieldErr); desc != "" {
		return desc
	}

	if desc := getFormatValidationDesc(tag, param); desc != "" {
		return desc
	}

	if desc := getNetworkValidationDesc(tag); desc != "" {
		return desc
	}

	if desc := getColorValidationDesc(tag); desc != "" {
		return desc
	}

	if desc := getDocumentValidationDesc(tag); desc != "" {
		return desc
	}

	if desc := getCustomValidationDesc(tag); desc != "" {
		return desc
	}

	return fmt.Sprintf("Failed validation: %s", tag)
}

func getBasicValidationDesc(tag, param string, fieldErr validator.FieldError) string {
	if desc := getCoreValidationDesc(tag, param, fieldErr); desc != "" {
		return desc
	}

	if desc := getStringValidationDesc(tag, param); desc != "" {
		return desc
	}

	if desc := getFieldComparisonDesc(tag, param); desc != "" {
		return desc
	}

	return ""
}

func getCoreValidationDesc(tag, param string, fieldErr validator.FieldError) string {
	switch tag {
	case "required":
		return "This field is required"
	case "email":
		return "Invalid email format"
	case "min":
		if fieldErr.Kind() == reflect.String {
			return fmt.Sprintf("Must be at least %s characters", param)
		}
		return fmt.Sprintf("Must be at least %s", param)
	case "max":
		if fieldErr.Kind() == reflect.String {
			return fmt.Sprintf("Must be at most %s characters", param)
		}
		return fmt.Sprintf("Must be at most %s", param)
	case "gte":
		return fmt.Sprintf("Must be greater than or equal to %s", param)
	case "lte":
		return fmt.Sprintf("Must be less than or equal to %s", param)
	case "gt":
		return fmt.Sprintf("Must be greater than %s", param)
	case "lt":
		return fmt.Sprintf("Must be less than %s", param)
	case "len":
		if fieldErr.Kind() == reflect.String {
			return fmt.Sprintf("Must be exactly %s characters", param)
		}
		return fmt.Sprintf("Must have exactly %s items", param)
	case "alpha":
		return "Must contain only alphabetic characters"
	case "alphanum":
		return "Must contain only alphanumeric characters"
	case "numeric":
		return "Must be a valid number"
	}
	return ""
}

func getStringValidationDesc(tag, param string) string {
	switch tag {
	case "oneof":
		options := strings.ReplaceAll(param, " ", ", ")
		return fmt.Sprintf("Must be one of: %s", options)
	case "containsany":
		return fmt.Sprintf("Must contain at least one of: %s", param)
	case "excludes":
		return fmt.Sprintf("Must not contain: %s", param)
	case "excludesall":
		return fmt.Sprintf("Must not contain any of: %s", param)
	case "startswith":
		return fmt.Sprintf("Must start with: %s", param)
	case "endswith":
		return fmt.Sprintf("Must end with: %s", param)
	case "datetime":
		return fmt.Sprintf("Must be a valid datetime in format: %s", param)
	}
	return ""
}

func getFieldComparisonDesc(tag, param string) string {
	switch tag {
	case "eqfield":
		return fmt.Sprintf("Must be equal to %s", param)
	case "nefield":
		return fmt.Sprintf("Must not be equal to %s", param)
	case "gtfield":
		return fmt.Sprintf("Must be greater than %s", param)
	case "ltfield":
		return fmt.Sprintf("Must be less than %s", param)
	}
	return ""
}

func getFormatValidationDesc(tag, _ string) string {
	switch tag {
	case "url":
		return "Must be a valid URL"
	case "uri":
		return "Must be a valid URI"
	case "uuid":
		return "Must be a valid UUID"
	case "uuid4":
		return "Must be a valid UUID v4"
	case "uuid5":
		return "Must be a valid UUID v5"
	case "json":
		return "Must be valid JSON"
	case "base64":
		return "Must be valid base64"
	case "jwt":
		return "Must be a valid JWT token"
	}
	return ""
}

func getNetworkValidationDesc(tag string) string {
	switch tag {
	case "hostname":
		return "Must be a valid hostname"
	case "fqdn":
		return "Must be a valid fully qualified domain name"
	case "ipv4":
		return "Must be a valid IPv4 address"
	case "ipv6":
		return "Must be a valid IPv6 address"
	case "ip":
		return "Must be a valid IP address"
	case "mac":
		return "Must be a valid MAC address"
	case "latitude":
		return "Must be a valid latitude"
	case "longitude":
		return "Must be a valid longitude"
	}
	return ""
}

func getColorValidationDesc(tag string) string {
	switch tag {
	case "rgb":
		return "Must be a valid RGB color"
	case "rgba":
		return "Must be a valid RGBA color"
	case "hsl":
		return "Must be a valid HSL color"
	case "hsla":
		return "Must be a valid HSLA color"
	case "hexcolor":
		return "Must be a valid hex color"
	}
	return ""
}

func getDocumentValidationDesc(tag string) string {
	switch tag {
	case "isbn":
		return "Must be a valid ISBN"
	case "isbn10":
		return "Must be a valid ISBN-10"
	case "isbn13":
		return "Must be a valid ISBN-13"
	case "credit_card":
		return "Must be a valid credit card number"
	case "ssn":
		return "Must be a valid social security number"
	}
	return ""
}

func getCustomValidationDesc(tag string) string {
	switch tag {
	case "phone_uz":
		return "Must be a valid Uzbek phone number (format: 998XXXXXXXXX)"
	case "strong_password":
		return "Must be a strong password (at least 8 characters with uppercase, lowercase, number, and special character)"
	}
	return ""
}
