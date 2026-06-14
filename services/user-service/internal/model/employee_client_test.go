package model

import (
	"reflect"
	"testing"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/permission"
)

func TestEmployeePermissionHelpers(t *testing.T) {
	t.Parallel()

	employee := &Employee{
		Permissions: []EmployeePermission{
			{Permission: permission.EmployeeView},
			{Permission: permission.Trading},
		},
	}

	if !employee.HasPermission(permission.Trading) {
		t.Fatal("expected employee to have trading permission")
	}
	if employee.HasPermission(permission.EmployeeDelete) {
		t.Fatal("did not expect employee delete permission")
	}

	want := []permission.Permission{permission.EmployeeView, permission.Trading}
	if got := employee.RawPermissions(); !reflect.DeepEqual(got, want) {
		t.Fatalf("raw permissions = %#v, want %#v", got, want)
	}

	if got := (*Employee)(nil).RawPermissions(); len(got) != 0 {
		t.Fatalf("nil employee raw permissions = %#v, want empty", got)
	}
}

func TestEmployeeRoles(t *testing.T) {
	t.Parallel()

	admin := &Employee{}
	for _, p := range permission.All {
		admin.Permissions = append(admin.Permissions, EmployeePermission{Permission: p})
	}
	if !admin.IsAdmin() {
		t.Fatal("expected all permissions to mark employee as admin")
	}
	if !admin.IsSupervisor() {
		t.Fatal("expected admin to be supervisor")
	}

	agent := &Employee{ActuaryInfo: &ActuaryInfo{IsAgent: true}}
	if !agent.IsAgent() {
		t.Fatal("expected agent role")
	}
	if agent.IsSupervisor() {
		t.Fatal("agent without supervisor flag should not be supervisor")
	}

	supervisor := &Employee{ActuaryInfo: &ActuaryInfo{IsSupervisor: true}}
	if !supervisor.IsSupervisor() {
		t.Fatal("expected supervisor role")
	}

	if (*Employee)(nil).IsSupervisor() {
		t.Fatal("nil employee should not be supervisor")
	}
}

func TestClientPermissionHelpers(t *testing.T) {
	t.Parallel()

	client := &Client{
		Permissions: []ClientPermission{
			{Permission: permission.ClientView},
			{Permission: permission.Trading},
		},
	}

	if !client.HasPermission(permission.ClientView) {
		t.Fatal("expected client view permission")
	}
	if client.HasPermission(permission.EmployeeView) {
		t.Fatal("did not expect employee view permission")
	}

	want := []permission.Permission{permission.ClientView, permission.Trading}
	if got := client.RawPermissions(); !reflect.DeepEqual(got, want) {
		t.Fatalf("raw permissions = %#v, want %#v", got, want)
	}

	if got := (*Client)(nil).RawPermissions(); len(got) != 0 {
		t.Fatalf("nil client raw permissions = %#v, want empty", got)
	}
}

func TestActuaryInfoHasRole(t *testing.T) {
	t.Parallel()

	if (ActuaryInfo{}).HasRole() {
		t.Fatal("empty actuary info should not have role")
	}
	if !(ActuaryInfo{IsAgent: true}).HasRole() {
		t.Fatal("agent should have role")
	}
	if !(ActuaryInfo{IsSupervisor: true}).HasRole() {
		t.Fatal("supervisor should have role")
	}
}
