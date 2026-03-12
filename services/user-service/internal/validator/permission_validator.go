package validator

import (
	"common/pkg/permission"
	"reflect"

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

func validateUniquePermissions(fl validator.FieldLevel) bool {
	field := fl.Field()
	if field.Kind() != reflect.Slice {
		return false
	}

	seen := make(map[permission.Permission]struct{}, field.Len())

	for i := 0; i < field.Len(); i++ {
		p, ok := field.Index(i).Interface().(permission.Permission)
		if !ok {
			return false
		}

		if _, exists := seen[p]; exists {
			return false
		}
		seen[p] = struct{}{}
	}

	return true
}
