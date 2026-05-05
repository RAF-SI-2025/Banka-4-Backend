package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/model"
)

func TestUpdateActuarySettings(t *testing.T) {
	t.Parallel()

	agent := activeAgent()

	tests := []struct {
		name        string
		empRepo     *fakeEmployeeRepo
		actuaryRepo *fakeActuaryRepo
		employeeID  uint
		req         *dto.UpdateActuarySettingsRequest
		expectErr   bool
		errMsg      string
	}{
		{
			name: "successful update",
			empRepo: &fakeEmployeeRepo{
				byIDs: map[uint]*model.Employee{agent.EmployeeID: agent},
			},
			actuaryRepo: &fakeActuaryRepo{
				byEmployeeID: map[uint]*model.ActuaryInfo{agent.EmployeeID: agent.ActuaryInfo},
			},
			employeeID: agent.EmployeeID,
			req: &dto.UpdateActuarySettingsRequest{
				Limit:        ptr(200000.0),
				NeedApproval: ptr(false),
			},
		},
		{
			name:        "employee not found",
			empRepo:     &fakeEmployeeRepo{byIDs: map[uint]*model.Employee{}},
			actuaryRepo: &fakeActuaryRepo{},
			employeeID:  999,
			req:         &dto.UpdateActuarySettingsRequest{Limit: ptr(1000.0)},
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
			req:         &dto.UpdateActuarySettingsRequest{Limit: ptr(1000.0)},
			expectErr:   true,
			errMsg:      "only agents have configurable limits",
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
			employeeID: agent.EmployeeID,
			req:        &dto.UpdateActuarySettingsRequest{Limit: ptr(200000.0)},
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewActuaryService(tt.actuaryRepo, tt.empRepo)

			response, err := service.UpdateActuarySettings(context.Background(), tt.employeeID, tt.req)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
				require.Nil(t, response)
			} else {
				require.NoError(t, err)
				require.NotNil(t, response)
				require.Equal(t, *tt.req.Limit, response.Limit)
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
			service := NewActuaryService(tt.actuaryRepo, tt.empRepo)

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

func TestIncrementUsedLimit(t *testing.T) {
	t.Parallel()

	agent := activeAgent()

	tests := []struct {
		name        string
		empRepo     *fakeEmployeeRepo
		actuaryRepo *fakeActuaryRepo
		employeeID  uint
		amount      float64
		expectUsed  float64
		expectErr   bool
		errMsg      string
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
			amount:     2500,
			expectUsed: agent.ActuaryInfo.UsedLimit + 2500,
		},
		{
			name:        "invalid amount",
			empRepo:     &fakeEmployeeRepo{},
			actuaryRepo: &fakeActuaryRepo{},
			employeeID:  agent.EmployeeID,
			amount:      0,
			expectErr:   true,
			errMsg:      "amount must be positive",
		},
		{
			name:        "employee not found",
			empRepo:     &fakeEmployeeRepo{byIDs: map[uint]*model.Employee{}},
			actuaryRepo: &fakeActuaryRepo{},
			employeeID:  999,
			amount:      1000,
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
			amount:      1000,
			expectErr:   true,
			errMsg:      "only agents have used limits",
		},
		{
			name: "repo increment error",
			empRepo: &fakeEmployeeRepo{
				byIDs: map[uint]*model.Employee{agent.EmployeeID: agent},
			},
			actuaryRepo: &fakeActuaryRepo{
				byEmployeeID: map[uint]*model.ActuaryInfo{agent.EmployeeID: agent.ActuaryInfo},
				incrementErr: fmt.Errorf("db error"),
			},
			employeeID: agent.EmployeeID,
			amount:     1000,
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewActuaryService(tt.actuaryRepo, tt.empRepo)

			usedLimit, err := service.IncrementUsedLimit(context.Background(), tt.employeeID, tt.amount)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
				require.Zero(t, usedLimit)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectUsed, usedLimit)
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
			service := NewActuaryService(tt.repo, &fakeEmployeeRepo{})

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
			service := NewActuaryService(tt.repo, &fakeEmployeeRepo{})

			err := service.ResetAllUsedLimits(context.Background())

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
