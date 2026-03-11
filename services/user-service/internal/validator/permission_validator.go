package validator

import (
	"common/pkg/permission"

	"github.com/go-playground/validator/v10"
)

func validatePermission(fl validator.FieldLevel) bool {
	value := permission.Permission(fl.Field().String())
	for _, p := range permission.All {
		if p == value {
			return true
		}
	}
	return false
}
