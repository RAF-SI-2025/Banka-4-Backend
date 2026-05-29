package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/model"
)

type fakeTradingClient struct {
	transferErr    error
	transferCalled bool
	fromManagerID  uint
	toManagerID    uint
}

func (f *fakeTradingClient) TransferFunds(_ context.Context, fromManagerID uint, toManagerID uint) (uint64, error) {
	f.transferCalled = true
	f.fromManagerID = fromManagerID
	f.toManagerID = toManagerID
	return 0, f.transferErr
}

func TestUpdateActuarySettings(t *testing.T) {
	t.Parallel()

	agent := activeAgent()
	supervisor := activeSupervisor()
	admin := adminEmployee()

	tests := []struct {
		name                 string
		empRepo              *fakeEmployeeRepo
		actuaryRepo          *fakeActuaryRepo
		tradingClient        *fakeTradingClient
		employeeID           uint
		callerID             uint
		req                  *dto.UpdateActuarySettingsRequest
		expectErr            bool
		errMsg               string
		expectTransferCalled bool
	}{
		{
			name: "successful limit update",
			empRepo: &fakeEmployeeRepo{
				byIDs: map[uint]*model.Employee{agent.EmployeeID: agent},
			},
			actuaryRepo: &fakeActuaryRepo{
				byEmployeeID: map[uint]*model.ActuaryInfo{agent.EmployeeID: agent.ActuaryInfo},
			},
			tradingClient: &fakeTradingClient{},
			employeeID:    agent.EmployeeID,
			callerID:      admin.EmployeeID,
			req: &dto.UpdateActuarySettingsRequest{
				Limit:        ptr(200000.0),
				NeedApproval: ptr(false),
			},
			expectTransferCalled: false,
		},
		{
			name:          "employee not found",
			empRepo:       &fakeEmployeeRepo{byIDs: map[uint]*model.Employee{}},
			actuaryRepo:   &fakeActuaryRepo{},
			tradingClient: &fakeTradingClient{},
			employeeID:    999,
			callerID:      admin.EmployeeID,
			req:           &dto.UpdateActuarySettingsRequest{Limit: ptr(1000.0)},
			expectErr:     true,
			errMsg:        "employee not found",
		},
		{
			name: "employee is not an agent - limit update fails",
			empRepo: &fakeEmployeeRepo{
				byIDs: map[uint]*model.Employee{activeEmployee().EmployeeID: activeEmployee()},
			},
			actuaryRepo:   &fakeActuaryRepo{},
			tradingClient: &fakeTradingClient{},
			employeeID:    activeEmployee().EmployeeID,
			callerID:      admin.EmployeeID,
			req:           &dto.UpdateActuarySettingsRequest{Limit: ptr(1000.0)},
			expectErr:     true,
			errMsg:        "only agents have configurable limits",
		},
		{
			name: "repo save error",
			empRepo: &fakeEmployeeRepo{
				byIDs: map[uint]*model.Employee{agent.EmployeeID: agent},
			},
			actuaryRepo: &fakeActuaryRepo{
				byEmployeeID: map[uint]*model.ActuaryInfo{agent.EmployeeID: agent.ActuaryInfo},
				saveErr:      fmt.Errorf("db error"),
			},
			tradingClient: &fakeTradingClient{},
			employeeID:    agent.EmployeeID,
			callerID:      admin.EmployeeID,
			req:           &dto.UpdateActuarySettingsRequest{Limit: ptr(200000.0)},
			expectErr:     true,
		},
		{
			name: "remove isSupervisor - transfer funds called",
			empRepo: &fakeEmployeeRepo{
				byIDs: map[uint]*model.Employee{supervisor.EmployeeID: supervisor},
			},
			actuaryRepo: &fakeActuaryRepo{
				byEmployeeID: map[uint]*model.ActuaryInfo{supervisor.EmployeeID: supervisor.ActuaryInfo},
			},
			tradingClient: &fakeTradingClient{},
			employeeID:    supervisor.EmployeeID,
			callerID:      admin.EmployeeID,
			req: &dto.UpdateActuarySettingsRequest{
				IsSupervisor: ptr(false),
			},
			expectTransferCalled: true,
		},
		{
			name: "keep isSupervisor true - transfer funds not called",
			empRepo: &fakeEmployeeRepo{
				byIDs: map[uint]*model.Employee{supervisor.EmployeeID: supervisor},
			},
			actuaryRepo: &fakeActuaryRepo{
				byEmployeeID: map[uint]*model.ActuaryInfo{supervisor.EmployeeID: supervisor.ActuaryInfo},
			},
			tradingClient: &fakeTradingClient{},
			employeeID:    supervisor.EmployeeID,
			callerID:      admin.EmployeeID,
			req: &dto.UpdateActuarySettingsRequest{
				IsSupervisor: ptr(true),
			},
			expectTransferCalled: false,
		},
		{
			name: "remove isSupervisor - transfer funds fails",
			empRepo: &fakeEmployeeRepo{
				byIDs: map[uint]*model.Employee{supervisor.EmployeeID: supervisor},
			},
			actuaryRepo: &fakeActuaryRepo{
				byEmployeeID: map[uint]*model.ActuaryInfo{supervisor.EmployeeID: supervisor.ActuaryInfo},
			},
			tradingClient: &fakeTradingClient{
				transferErr: fmt.Errorf("trading service unavailable"),
			},
			employeeID: supervisor.EmployeeID,
			callerID:   admin.EmployeeID,
			req: &dto.UpdateActuarySettingsRequest{
				IsSupervisor: ptr(false),
			},
			expectErr:            true,
			expectTransferCalled: true,
		},
		{
			name: "set isAgent true",
			empRepo: &fakeEmployeeRepo{
				byIDs: map[uint]*model.Employee{supervisor.EmployeeID: supervisor},
			},
			actuaryRepo: &fakeActuaryRepo{
				byEmployeeID: map[uint]*model.ActuaryInfo{supervisor.EmployeeID: supervisor.ActuaryInfo},
			},
			tradingClient: &fakeTradingClient{},
			employeeID:    supervisor.EmployeeID,
			callerID:      admin.EmployeeID,
			req: &dto.UpdateActuarySettingsRequest{
				IsAgent: ptr(true),
			},
			expectTransferCalled: false,
		},
		{
			name: "no actuary info - fails when changing isAgent",
			empRepo: &fakeEmployeeRepo{
				byIDs: map[uint]*model.Employee{activeEmployee().EmployeeID: activeEmployee()},
			},
			actuaryRepo:   &fakeActuaryRepo{},
			tradingClient: &fakeTradingClient{},
			employeeID:    activeEmployee().EmployeeID,
			callerID:      admin.EmployeeID,
			req: &dto.UpdateActuarySettingsRequest{
				IsAgent: ptr(true),
			},
			expectErr: true,
			errMsg:    "employee has no actuary info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewActuaryService(tt.actuaryRepo, tt.empRepo, tt.tradingClient, fakeAuditService(nil))

			response, err := service.UpdateActuarySettings(context.Background(), tt.employeeID, tt.callerID, tt.req)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
				require.Nil(t, response)
			} else {
				require.NoError(t, err)
				require.NotNil(t, response)
			}

			require.Equal(t, tt.expectTransferCalled, tt.tradingClient.transferCalled)
		})
	}
}

func TestIncrementUsedLimit(t *testing.T) {
	t.Parallel()

	agent := activeAgent()

	tests := []struct {
		name        string
		empRepo     *fakeEmployeeRepo
		actuaryRepo *fakeActuaryRepo
		employeeID  uint
		amount      float64
		expectErr   bool
		errMsg      string
		expectUsed  float64
	}{
		{
			name: "successful increment",
			empRepo: &fakeEmployeeRepo{
				byIDs: map[uint]*model.Employee{agent.EmployeeID: agent},
			},
			actuaryRepo: &fakeActuaryRepo{
				byEmployeeID: map[uint]*model.ActuaryInfo{agent.EmployeeID: agent.ActuaryInfo},
			},
			employeeID: agent.EmployeeID,
			amount:     500.0,
			expectUsed: agent.ActuaryInfo.UsedLimit + 500.0,
		},
		{
			name:        "amount zero",
			empRepo:     &fakeEmployeeRepo{},
			actuaryRepo: &fakeActuaryRepo{},
			employeeID:  agent.EmployeeID,
			amount:      0,
			expectErr:   true,
			errMsg:      "amount must be positive",
		},
		{
			name:        "amount negative",
			empRepo:     &fakeEmployeeRepo{},
			actuaryRepo: &fakeActuaryRepo{},
			employeeID:  agent.EmployeeID,
			amount:      -10.0,
			expectErr:   true,
			errMsg:      "amount must be positive",
		},
		{
			name:        "employee not found",
			empRepo:     &fakeEmployeeRepo{byIDs: map[uint]*model.Employee{}},
			actuaryRepo: &fakeActuaryRepo{},
			employeeID:  999,
			amount:      100.0,
			expectErr:   true,
			errMsg:      "employee not found",
		},
		{
			name: "employee find error",
			empRepo: &fakeEmployeeRepo{
				findErr: fmt.Errorf("db error"),
			},
			actuaryRepo: &fakeActuaryRepo{},
			employeeID:  agent.EmployeeID,
			amount:      100.0,
			expectErr:   true,
		},
		{
			name: "employee is not an agent",
			empRepo: &fakeEmployeeRepo{
				byIDs: map[uint]*model.Employee{activeEmployee().EmployeeID: activeEmployee()},
			},
			actuaryRepo: &fakeActuaryRepo{},
			employeeID:  activeEmployee().EmployeeID,
			amount:      100.0,
			expectErr:   true,
			errMsg:      "only agents have used limits",
		},
		{
			name: "actuary info not found - nil result",
			empRepo: &fakeEmployeeRepo{
				byIDs: map[uint]*model.Employee{agent.EmployeeID: agent},
			},
			actuaryRepo: &fakeActuaryRepo{},
			employeeID:  agent.EmployeeID,
			amount:      100.0,
			expectErr:   true,
			errMsg:      "actuary info not found",
		},
		{
			name: "actuary repo record not found error",
			empRepo: &fakeEmployeeRepo{
				byIDs: map[uint]*model.Employee{agent.EmployeeID: agent},
			},
			actuaryRepo: &fakeActuaryRepo{
				incrementErr: gorm.ErrRecordNotFound,
			},
			employeeID: agent.EmployeeID,
			amount:     100.0,
			expectErr:  true,
			errMsg:     "actuary info not found",
		},
		{
			name: "actuary repo internal error",
			empRepo: &fakeEmployeeRepo{
				byIDs: map[uint]*model.Employee{agent.EmployeeID: agent},
			},
			actuaryRepo: &fakeActuaryRepo{
				incrementErr: fmt.Errorf("db error"),
			},
			employeeID: agent.EmployeeID,
			amount:     100.0,
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewActuaryService(tt.actuaryRepo, tt.empRepo, &fakeTradingClient{}, fakeAuditService(nil))

			used, err := service.IncrementUsedLimit(context.Background(), tt.employeeID, tt.amount)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
				require.Zero(t, used)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectUsed, used)
			}
		})
	}
}

func TestResetUsedLimit(t *testing.T) {
	t.Parallel()

	agent := activeAgent()

	tests := []struct {
		name        string
		empRepo     *fakeEmployeeRepo
		actuaryRepo *fakeActuaryRepo
		employeeID  uint
		expectErr   bool
		errMsg      string
	}{
		{
			name: "successful reset",
			empRepo: &fakeEmployeeRepo{
				byIDs: map[uint]*model.Employee{agent.EmployeeID: agent},
			},
			actuaryRepo: &fakeActuaryRepo{
				byEmployeeID: map[uint]*model.ActuaryInfo{agent.EmployeeID: agent.ActuaryInfo},
			},
			employeeID: agent.EmployeeID,
		},
		{
			name:        "employee not found",
			empRepo:     &fakeEmployeeRepo{byIDs: map[uint]*model.Employee{}},
			actuaryRepo: &fakeActuaryRepo{},
			employeeID:  999,
			expectErr:   true,
			errMsg:      "employee not found",
		},
		{
			name: "employee is not an agent",
			empRepo: &fakeEmployeeRepo{
				byIDs: map[uint]*model.Employee{activeEmployee().EmployeeID: activeEmployee()},
			},
			actuaryRepo: &fakeActuaryRepo{},
			employeeID:  activeEmployee().EmployeeID,
			expectErr:   true,
			errMsg:      "only agents have used limits",
		},
		{
			name: "repo reset error",
			empRepo: &fakeEmployeeRepo{
				byIDs: map[uint]*model.Employee{agent.EmployeeID: agent},
			},
			actuaryRepo: &fakeActuaryRepo{
				byEmployeeID: map[uint]*model.ActuaryInfo{agent.EmployeeID: agent.ActuaryInfo},
				resetErr:     fmt.Errorf("db error"),
			},
			employeeID: agent.EmployeeID,
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewActuaryService(tt.actuaryRepo, tt.empRepo, &fakeTradingClient{}, fakeAuditService(nil))

			response, err := service.ResetUsedLimit(context.Background(), tt.employeeID)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
				require.Nil(t, response)
			} else {
				require.NoError(t, err)
				require.Zero(t, response.UsedLimit)
			}
		})
	}
}

func TestGetAllActuaries(t *testing.T) {
	t.Parallel()

	agent := activeAgent()

	tests := []struct {
		name        string
		repo        *fakeActuaryRepo
		query       *dto.ListActuariesQuery
		expectErr   bool
		expectTotal int64
		expectLen   int
	}{
		{
			name: "success with results",
			repo: &fakeActuaryRepo{
				allEmployees: []model.Employee{*agent},
				allTotal:     1,
			},
			query:       &dto.ListActuariesQuery{Page: 1, PageSize: 10},
			expectTotal: 1,
			expectLen:   1,
		},
		{
			name: "empty results",
			repo: &fakeActuaryRepo{
				allEmployees: []model.Employee{},
				allTotal:     0,
			},
			query:       &dto.ListActuariesQuery{Page: 1, PageSize: 10},
			expectTotal: 0,
			expectLen:   0,
		},
		{
			name: "repo error",
			repo: &fakeActuaryRepo{
				getAllErr: fmt.Errorf("db down"),
			},
			query:     &dto.ListActuariesQuery{Page: 1, PageSize: 10},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewActuaryService(tt.repo, &fakeEmployeeRepo{}, &fakeTradingClient{}, fakeAuditService(nil))

			response, err := service.GetAllActuaries(context.Background(), tt.query)

			if tt.expectErr {
				require.Error(t, err)
				require.Nil(t, response)
			} else {
				require.NoError(t, err)
				require.NotNil(t, response)
				require.Equal(t, tt.expectTotal, response.Total)
				require.Len(t, response.Data, tt.expectLen)
				require.Equal(t, tt.query.Page, response.Page)
				require.Equal(t, tt.query.PageSize, response.PageSize)
			}
		})
	}
}

func TestResetAllUsedLimits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		repo      *fakeActuaryRepo
		expectErr bool
	}{
		{
			name: "success",
			repo: &fakeActuaryRepo{},
		},
		{
			name: "repo error",
			repo: &fakeActuaryRepo{
				resetAllErr: fmt.Errorf("db down"),
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewActuaryService(tt.repo, &fakeEmployeeRepo{}, &fakeTradingClient{}, fakeAuditService(nil))

			err := service.ResetAllUsedLimits(context.Background())

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
