package validator

import (
	"unicode"

	"github.com/go-playground/validator/v10"
)

func validatePassword(fl validator.FieldLevel) bool {
	password := fl.Field().String()
	if len(password) < 8 || len(password) > 32 {
		return false
	}

	var upper, lower, digits int
	for _, c := range password {
		switch {
		case unicode.IsUpper(c):
			upper++
		case unicode.IsLower(c):
			lower++
		case unicode.IsDigit(c):
			digits++
		}
	}

	return upper >= 1 && lower >= 1 && digits >= 2
}

