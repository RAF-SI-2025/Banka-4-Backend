package validator

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/require"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/permission"
)

func newPermissionValidator() *validator.Validate {
	v := validator.New()
	_ = v.RegisterValidation("permission", validatePermission)
	return v
}

type permissionField struct {
	Perm string `validate:"permission"`
}

func TestValidatePermission(t *testing.T) {
	t.Parallel()
	v := newPermissionValidator()

	tests := []struct {
		name    string
		input   string
		isValid bool
	}{
		{name: "employee.view", input: string(permission.EmployeeView), isValid: true},
		{name: "employee.create", input: string(permission.EmployeeCreate), isValid: true},
		{name: "employee.update", input: string(permission.EmployeeUpdate), isValid: true},
		{name: "employee.delete", input: string(permission.EmployeeDelete), isValid: true},
		{name: "trading", input: string(permission.Trading), isValid: true},
		{name: "trading.margin", input: string(permission.TradingMargin), isValid: true},
		{name: "invalid permission", input: "admin.super", isValid: false},
		{name: "empty string", input: "", isValid: false},
		{name: "partial match", input: "employee", isValid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := v.Struct(permissionField{Perm: tt.input})
			if tt.isValid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

type uniquePermissionsField struct {
	Perms []permission.Permission `validate:"unique_permissions"`
}

func TestValidateUniquePermissions(t *testing.T) {
	t.Parallel()
	v := validator.New()
	_ = v.RegisterValidation("unique_permissions", validateUniquePermissions)

	tests := []struct {
		name    string
		input   []permission.Permission
		isValid bool
	}{
		{name: "unique permissions", input: []permission.Permission{permission.EmployeeView, permission.EmployeeCreate}, isValid: true},
		{name: "single permission", input: []permission.Permission{permission.EmployeeView}, isValid: true},
		{name: "empty slice", input: []permission.Permission{}, isValid: true},
		{name: "nil slice", input: nil, isValid: true},
		{name: "duplicate permissions", input: []permission.Permission{permission.EmployeeView, permission.EmployeeView}, isValid: false},
		{name: "duplicate among many", input: []permission.Permission{permission.EmployeeView, permission.EmployeeCreate, permission.EmployeeView}, isValid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := v.Struct(uniquePermissionsField{Perms: tt.input})
			if tt.isValid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
