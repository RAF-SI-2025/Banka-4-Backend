package validator

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/require"
)

func newValidateInstance() *validator.Validate {
	v := validator.New()
	_ = v.RegisterValidation("password", validatePassword)
	return v
}

type passwordField struct {
	Password string `validate:"password"`
}

func TestValidatePassword(t *testing.T) {
	t.Parallel()
	v := newValidateInstance()

	tests := []struct {
		name    string
		input   string
		isValid bool
	}{
		{name: "valid password", input: "Abcdef12", isValid: true},
		{name: "valid with extra digits", input: "Hello123World", isValid: true},
		{name: "too short", input: "Ab12", isValid: false},
		{name: "too long", input: "Aaaaaaaabbbbbbbb12ccccccccdddddddd", isValid: false},
		{name: "no uppercase", input: "abcdefg12", isValid: false},
		{name: "no lowercase", input: "ABCDEFG12", isValid: false},
		{name: "no digits", input: "Abcdefghij", isValid: false},
		{name: "only one digit", input: "Abcdefgh1", isValid: false},
		{name: "empty string", input: "", isValid: false},
		{name: "exactly 8 chars valid", input: "aB345678", isValid: true},
		{name: "exactly 32 chars valid", input: "aB12cdefghijklmnopqrstuvwxyz1234", isValid: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := v.Struct(passwordField{Password: tt.input})
			if tt.isValid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
