package model

import "testing"

func TestPermissionTableNames(t *testing.T) {
	t.Parallel()

	if got := (EmployeePermission{}).TableName(); got != "employee_permissions" {
		t.Fatalf("employee permission table = %q", got)
	}
	if got := (ClientPermission{}).TableName(); got != "client_permissions" {
		t.Fatalf("client permission table = %q", got)
	}
}
