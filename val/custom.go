package val

import "regexp"

// IsPhoneUz checks if the provided phone number is a valid Uzbek phone number.
// Format: 998XXXXXXXXX (12 digits without +).
func IsPhoneUz(phone string) bool {
	matched, _ := regexp.MatchString(`^998[0-9]{9}$`, phone)
	return matched
}

// Implement commonly used custom validation functions here...
