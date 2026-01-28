package val

import (
	"regexp"

	"github.com/go-playground/validator/v10"
)

// Phone number regex patterns.
var (
	phoneUzRegex = regexp.MustCompile(`^998[0-9]{9}$`)
)

// validatePhoneUz is a validator function for phone_uz tag.
func validatePhoneUz(fl validator.FieldLevel) bool {
	phone := fl.Field().String()
	return IsPhoneUz(phone)
}

func IsPhoneUz(phone string) bool {
	return phoneUzRegex.MatchString(phone)
}

// validateStrongPassword is a validator function for strong_password tag.
func validateStrongPassword(fl validator.FieldLevel) bool {
	password := fl.Field().String()
	return IsStrongPassword(password)
}

// IsStrongPassword checks if the password meets strong password criteria.
// At least 8 characters, uppercase, lowercase, number, and special character.
func IsStrongPassword(password string) bool {
	// A strong password must:
	if len(password) < 8 {
		return false
	}

	var (
		hasUpper   = false
		hasLower   = false
		hasNumber  = false
		hasSpecial = false
	)

	for _, char := range password {
		switch {
		case char >= 'A' && char <= 'Z':
			hasUpper = true
		case char >= 'a' && char <= 'z':
			hasLower = true
		case char >= '0' && char <= '9':
			hasNumber = true
		case char < 'A' || (char > 'Z' && char < 'a') || (char > 'z' && char < '0') || char > '9':
			hasSpecial = true
		}
	}

	return hasUpper && hasLower && hasNumber && hasSpecial
}

// Additional custom validation functions can be added here...
