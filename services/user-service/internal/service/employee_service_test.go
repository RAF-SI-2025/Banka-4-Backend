package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/permission"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/model"
)

func newEmployeeService(
	employeeRepo *fakeEmployeeRepo,
	identityRepo *fakeIdentityRepo,
	activationTokenRepo *fakeActivationTokenRepo,
	positionRepo *fakePositionRepo,
	mailer *fakeMailer,
) *EmployeeService {
	return NewEmployeeService(
		employeeRepo,
		identityRepo,
		activationTokenRepo,
		positionRepo,
		mailer,
		testConfig(),
		&fakeTxManager{},
	)
}

func TestRegister(t *testing.T) {
	t.Parallel()

	req := &dto.CreateEmployeeRequest{
		FirstName:  "Jane",
		LastName:   "Doe",
		Email:      "jane@example.com",
		Username:   "janedoe",
		PositionID: 1,
	}

	tests := []struct {
		name         string
		empRepo      *fakeEmployeeRepo
		identityRepo *fakeIdentityRepo
		positionRepo *fakePositionRepo
		mailer       *fakeMailer
		expectErr    bool
		errMsg       string
	}{
		{
			name:         "successful registration",
			empRepo:      &fakeEmployeeRepo{},
			identityRepo: &fakeIdentityRepo{},
			positionRepo: &fakePositionRepo{exists: true},
			mailer:       &fakeMailer{},
		},
		{
			name:         "email already in use",
			empRepo:      &fakeEmployeeRepo{},
			identityRepo: &fakeIdentityRepo{emailExists: true},
			positionRepo: &fakePositionRepo{exists: true},
			mailer:       &fakeMailer{},
			expectErr:    true,
			errMsg:       "email already in use",
		},
		{
			name:         "username already in use",
			empRepo:      &fakeEmployeeRepo{},
			identityRepo: &fakeIdentityRepo{usernameExists: true},
			positionRepo: &fakePositionRepo{exists: true},
			mailer:       &fakeMailer{},
			expectErr:    true,
			errMsg:       "username already in use",
		},
		{
			name:         "repo create fails",
			empRepo:      &fakeEmployeeRepo{createErr: fmt.Errorf("db error")},
			identityRepo: &fakeIdentityRepo{},
			positionRepo: &fakePositionRepo{exists: true},
			mailer:       &fakeMailer{},
			expectErr:    true,
		},
		{
			name:         "email send fails",
			empRepo:      &fakeEmployeeRepo{},
			identityRepo: &fakeIdentityRepo{},
			positionRepo: &fakePositionRepo{exists: true},
			mailer:       &fakeMailer{sendErr: fmt.Errorf("smtp down")},
			expectErr:    true,
		},
		{
			name:         "invalid position",
			empRepo:      &fakeEmployeeRepo{},
			identityRepo: &fakeIdentityRepo{},
			positionRepo: &fakePositionRepo{exists: false},
			mailer:       &fakeMailer{},
			expectErr:    true,
			errMsg:       "invalid position",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			admin := adminEmployee()
			if tt.empRepo.byIDs == nil {
				tt.empRepo.byIDs = map[uint]*model.Employee{}
			}
			tt.empRepo.byIDs[admin.EmployeeID] = admin

			svc := newEmployeeService(tt.empRepo, tt.identityRepo, &fakeActivationTokenRepo{}, tt.positionRepo, tt.mailer)

			emp, err := svc.Register(withAuth(context.Background(), admin.IdentityID, auth.IdentityEmployee), req)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, emp)
				require.Equal(t, "Jane", emp.FirstName)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }

func TestUpdateEmployee(t *testing.T) {
	t.Parallel()

	existing := activeEmployee()
	existing.PositionID = 1

	existingAdmin := adminEmployee()
	existingAdmin.EmployeeID = 4
	existingAdmin.IdentityID = 4
	existingAdmin.ActuaryInfo.EmployeeID = existingAdmin.EmployeeID

	identity := activeIdentity()
	actor := activeEmployee()
	actor.EmployeeID = 9
	actor.IdentityID = 9

	req := &dto.UpdateEmployeeRequest{
		FirstName:  ptr("John"),
		LastName:   ptr("Updated"),
		Email:      ptr("john@example.com"),
		Username:   ptr("johndoe"),
		PositionID: ptr(uint(1)),
	}

	tests := []struct {
		name         string
		empRepo      *fakeEmployeeRepo
		identityRepo *fakeIdentityRepo
		positionRepo *fakePositionRepo
		id           uint
		req          *dto.UpdateEmployeeRequest
		expectErr    bool
		errMsg       string
		useNoAuth    bool
	}{
		{
			name:         "successful update same email/username",
			empRepo:      &fakeEmployeeRepo{byIDs: map[uint]*model.Employee{existing.EmployeeID: existing, actor.EmployeeID: actor}},
			identityRepo: &fakeIdentityRepo{byID: identity},
			positionRepo: &fakePositionRepo{exists: true},
			id:           existing.EmployeeID,
			req:          req,
		},
		{
			name:         "employee not found",
			empRepo:      &fakeEmployeeRepo{byIDs: map[uint]*model.Employee{actor.EmployeeID: actor}},
			identityRepo: &fakeIdentityRepo{},
			positionRepo: &fakePositionRepo{},
			id:           999,
			req:          req,
			expectErr:    true,
			errMsg:       "employee not found",
		},
		{
			// TODO: Test for ability for other admins, or the admin themselves, to update their details?
			name:         "employee is admin",
			empRepo:      &fakeEmployeeRepo{byIDs: map[uint]*model.Employee{existingAdmin.EmployeeID: existingAdmin, actor.EmployeeID: actor}},
			identityRepo: &fakeIdentityRepo{byID: identity},
			positionRepo: &fakePositionRepo{},
			id:           existingAdmin.EmployeeID,
			req:          req,
			expectErr:    true,
			errMsg:       "cannot modify admin",
		},
		{
			name:         "email conflict",
			empRepo:      &fakeEmployeeRepo{byIDs: map[uint]*model.Employee{existing.EmployeeID: existing, actor.EmployeeID: actor}},
			identityRepo: &fakeIdentityRepo{byID: identity, emailExists: true},
			positionRepo: &fakePositionRepo{},
			id:           existing.EmployeeID,
			req: &dto.UpdateEmployeeRequest{
				Email:    ptr("taken@example.com"),
				Username: ptr("johndoe"),
			},
			expectErr: true,
			errMsg:    "email already in use",
		},
		{
			name:         "username conflict",
			empRepo:      &fakeEmployeeRepo{byIDs: map[uint]*model.Employee{existing.EmployeeID: existing, actor.EmployeeID: actor}},
			identityRepo: &fakeIdentityRepo{byID: identity, usernameExists: true},
			positionRepo: &fakePositionRepo{},
			id:           existing.EmployeeID,
			req: &dto.UpdateEmployeeRequest{
				Email:    ptr("john@example.com"),
				Username: ptr("taken"),
			},
			expectErr: true,
			errMsg:    "username already in use",
		},
		{
			name:         "invalid position",
			empRepo:      &fakeEmployeeRepo{byIDs: map[uint]*model.Employee{existing.EmployeeID: existing, actor.EmployeeID: actor}},
			identityRepo: &fakeIdentityRepo{byID: identity},
			positionRepo: &fakePositionRepo{exists: false},
			id:           existing.EmployeeID,
			req: &dto.UpdateEmployeeRequest{
				Email:      ptr("john@example.com"),
				Username:   ptr("johndoe"),
				PositionID: ptr(uint(999)),
			},
			expectErr: true,
			errMsg:    "invalid position_id",
		},
		{
			name:         "identity not found for employee",
			empRepo:      &fakeEmployeeRepo{byIDs: map[uint]*model.Employee{existing.EmployeeID: existing, actor.EmployeeID: actor}},
			identityRepo: &fakeIdentityRepo{byID: nil},
			positionRepo: &fakePositionRepo{exists: true},
			id:           existing.EmployeeID,
			req:          req,
			expectErr:    true,
			errMsg:       "identity not found",
		},
		{
			name: "admin cannot modify other admin permissions",
			empRepo: &fakeEmployeeRepo{byIDs: map[uint]*model.Employee{
				existingAdmin.EmployeeID: existingAdmin,
				actor.EmployeeID:         actor,
			}},
			identityRepo: &fakeIdentityRepo{byID: identity},
			positionRepo: &fakePositionRepo{exists: true},
			id:           existingAdmin.EmployeeID,
			req: &dto.UpdateEmployeeRequest{
				Permissions: &[]permission.Permission{permission.EmployeeView},
			},
			expectErr: true,
			errMsg:    "cannot modify admin",
		},
		{
			name:         "toggle active flag",
			empRepo:      &fakeEmployeeRepo{byIDs: map[uint]*model.Employee{existing.EmployeeID: existing, actor.EmployeeID: actor}},
			identityRepo: &fakeIdentityRepo{byID: identity},
			positionRepo: &fakePositionRepo{exists: true},
			id:           existing.EmployeeID,
			req: &dto.UpdateEmployeeRequest{
				Active: ptr(false),
			},
		},
		{
			name:         "no auth context returns error",
			empRepo:      &fakeEmployeeRepo{byIDs: map[uint]*model.Employee{existing.EmployeeID: existing}},
			identityRepo: &fakeIdentityRepo{byID: identity},
			positionRepo: &fakePositionRepo{exists: true},
			id:           existing.EmployeeID,
			req:          req,
			expectErr:    true,
			errMsg:       "not authenticated",
			useNoAuth:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newEmployeeService(tt.empRepo, tt.identityRepo, &fakeActivationTokenRepo{}, tt.positionRepo, &fakeMailer{})

			ctx := withAuth(context.Background(), actor.IdentityID, auth.IdentityEmployee)
			if tt.useNoAuth {
				ctx = context.Background()
			}

			result, err := svc.UpdateEmployee(ctx, tt.id, tt.req)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
			}
		})
	}
}

func TestGetAllEmployees(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		repo      *fakeEmployeeRepo
		expectErr bool
	}{
		{
			name: "success",
			repo: &fakeEmployeeRepo{
				allEmps:  []model.Employee{*activeEmployee()},
				allTotal: 1,
			},
		},
		{
			name:      "repo error",
			repo:      &fakeEmployeeRepo{getAllErr: fmt.Errorf("db error")},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newEmployeeService(tt.repo, &fakeIdentityRepo{}, &fakeActivationTokenRepo{}, &fakePositionRepo{}, &fakeMailer{})

			res, err := svc.GetAllEmployees(context.Background(), &dto.ListEmployeesQuery{Page: 1, PageSize: 10})

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, int64(1), res.Total)
				require.Len(t, res.Data, 1)
			}
		})
	}
}

func TestDeactivateEmployee(t *testing.T) {
	t.Parallel()

	active := activeEmployee()
	adminEmp := activeEmployee()
	adminEmp.Permissions = mapPermissions(adminEmp.EmployeeID, permission.All)

	identity := activeIdentity()

	tests := []struct {
		name         string
		empRepo      *fakeEmployeeRepo
		identityRepo *fakeIdentityRepo
		id           uint
		expectErr    bool
		errMsg       string
	}{
		{
			name:         "successful deactivation",
			empRepo:      &fakeEmployeeRepo{byID: active},
			identityRepo: &fakeIdentityRepo{byID: identity},
			id:           1,
		},
		{
			name:         "employee not found",
			empRepo:      &fakeEmployeeRepo{byID: nil},
			identityRepo: &fakeIdentityRepo{},
			id:           999,
			expectErr:    true,
			errMsg:       "employee not found",
		},
		{
			name:         "cannot deactivate admin",
			empRepo:      &fakeEmployeeRepo{byID: adminEmp},
			identityRepo: &fakeIdentityRepo{byID: identity},
			id:           1,
			expectErr:    true,
			errMsg:       "cannot deactivate admin",
		},
		{
			name:         "identity not found for employee",
			empRepo:      &fakeEmployeeRepo{byID: active},
			identityRepo: &fakeIdentityRepo{byID: nil},
			id:           1,
			expectErr:    true,
			errMsg:       "identity not found",
		},
		{
			name:         "identity update error",
			empRepo:      &fakeEmployeeRepo{byID: active},
			identityRepo: &fakeIdentityRepo{byID: identity, updateErr: fmt.Errorf("db error")},
			id:           1,
			expectErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newEmployeeService(tt.empRepo, tt.identityRepo, &fakeActivationTokenRepo{}, &fakePositionRepo{}, &fakeMailer{})

			err := svc.DeactivateEmployee(context.Background(), tt.id)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, tt.identityRepo.updatedIdentity)
				require.False(t, tt.identityRepo.updatedIdentity.Active)
			}
		})
	}
}

func TestGetEmployeeByID(t *testing.T) {
	t.Parallel()

	emp := activeEmployee()

	tests := []struct {
		name      string
		empRepo   *fakeEmployeeRepo
		id        uint
		expectErr bool
		errMsg    string
	}{
		{
			name:    "success",
			empRepo: &fakeEmployeeRepo{byID: emp},
			id:      emp.EmployeeID,
		},
		{
			name:      "not found",
			empRepo:   &fakeEmployeeRepo{byID: nil},
			id:        999,
			expectErr: true,
			errMsg:    "employee not found",
		},
		{
			name:      "repo error",
			empRepo:   &fakeEmployeeRepo{findErr: fmt.Errorf("db error")},
			id:        1,
			expectErr: true,
			errMsg:    "db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newEmployeeService(tt.empRepo, &fakeIdentityRepo{}, &fakeActivationTokenRepo{}, &fakePositionRepo{}, &fakeMailer{})

			res, err := svc.GetEmployeeByID(context.Background(), tt.id)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, res)
				require.Equal(t, emp.EmployeeID, res.Id)
			}
		})
	}
}

func TestBuildActuaryInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		existing     *model.ActuaryInfo
		isAdmin      bool
		isAgent      bool
		isSupervisor bool
		limit        float64
		needApproval bool
		expectErr    bool
		errMsg       string
		expectNil    bool
		checkAgent   bool
		checkSuper   bool
	}{
		{
			name:         "admin forces supervisor",
			isAdmin:      true,
			isAgent:      true,
			isSupervisor: false,
			checkSuper:   true,
		},
		{
			name:         "agent and supervisor conflict",
			isAgent:      true,
			isSupervisor: true,
			expectErr:    true,
			errMsg:       "cannot be both agent and supervisor",
		},
		{
			name:      "neither agent nor supervisor returns nil",
			expectNil: true,
		},
		{
			name:         "agent with limit",
			isAgent:      true,
			limit:        50000,
			needApproval: true,
			checkAgent:   true,
		},
		{
			name:         "supervisor zeroes limit",
			isSupervisor: true,
			limit:        50000,
			needApproval: true,
			checkSuper:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := buildActuaryInfo(tt.existing, tt.isAdmin, tt.isAgent, tt.isSupervisor, tt.limit, tt.needApproval)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}

			require.NoError(t, err)

			if tt.expectNil {
				require.Nil(t, result)
				return
			}

			require.NotNil(t, result)

			if tt.checkSuper {
				require.True(t, result.IsSupervisor)
				require.False(t, result.IsAgent)
				require.Equal(t, float64(0), result.Limit)
				require.False(t, result.NeedApproval)
			}

			if tt.checkAgent {
				require.True(t, result.IsAgent)
				require.False(t, result.IsSupervisor)
				require.Equal(t, tt.limit, result.Limit)
				require.Equal(t, tt.needApproval, result.NeedApproval)
			}
		})
	}
}
