package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
)

type fakeCompanyRepo struct {
	createdCompany        *model.Company
	createErr             error
	companies             []model.Company
	companiesErr          error
	workCodes             []model.WorkCode
	workCodesErr          error
	workCodeExists        bool
	workCodeErr           error
	registrationNumExists bool
	registrationNumErr    error
	taxNumExists          bool
	taxNumErr             error
	getWorkCodesCalls     int
}

func (f *fakeCompanyRepo) Create(_ context.Context, company *model.Company) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.createdCompany = company
	return nil
}

func (f *fakeCompanyRepo) GetCompanies(_ context.Context) ([]model.Company, error) {
	if f.companiesErr != nil {
		return nil, f.companiesErr
	}
	return f.companies, nil
}

func (f *fakeCompanyRepo) GetWorkCodes(_ context.Context) ([]model.WorkCode, error) {
	f.getWorkCodesCalls++
	if f.workCodesErr != nil {
		return nil, f.workCodesErr
	}
	return f.workCodes, nil
}

type fakeWorkCodeCache struct {
	cached           []model.WorkCode
	found            bool
	getErr           error
	setErr           error
	getCalls         int
	setCalls         int
	lastSetWorkCodes []model.WorkCode
}

func (f *fakeWorkCodeCache) Get(_ context.Context) ([]model.WorkCode, bool, error) {
	f.getCalls++
	if f.getErr != nil {
		return nil, false, f.getErr
	}

	if !f.found {
		return nil, false, nil
	}

	return f.cached, true, nil
}

func (f *fakeWorkCodeCache) Set(_ context.Context, workCodes []model.WorkCode) error {
	f.setCalls++
	f.lastSetWorkCodes = workCodes
	return f.setErr
}

func (f *fakeCompanyRepo) WorkCodeExists(_ context.Context, _ uint) (bool, error) {
	if f.workCodeErr != nil {
		return false, f.workCodeErr
	}
	return f.workCodeExists, nil
}

func (f *fakeCompanyRepo) RegistrationNumberExists(_ context.Context, _ string) (bool, error) {
	if f.registrationNumErr != nil {
		return false, f.registrationNumErr
	}
	return f.registrationNumExists, nil
}

func (f *fakeCompanyRepo) TaxNumberExists(_ context.Context, _ string) (bool, error) {
	if f.taxNumErr != nil {
		return false, f.taxNumErr
	}
	return f.taxNumExists, nil
}

func TestCreateCompany(t *testing.T) {
	t.Parallel()

	req := dto.CreateCompanyRequest{
		Name:               "Acme Ltd",
		RegistrationNumber: "12345678",
		TaxNumber:          "123456789",
		WorkCodeID:         1,
		Address:            "123 Main St",
		OwnerID:            1,
	}

	tests := []struct {
		name       string
		repo       *fakeCompanyRepo
		userClient *fakeUserClient
		req        dto.CreateCompanyRequest
		expectErr  bool
		errMsg     string
	}{
		{
			name:       "success",
			repo:       &fakeCompanyRepo{workCodeExists: true},
			userClient: &fakeUserClient{},
			req:        req,
		},
		{
			name:       "owner client not found",
			repo:       &fakeCompanyRepo{},
			userClient: &fakeUserClient{clientErr: fmt.Errorf("not found")},
			req:        req,
			expectErr:  true,
			errMsg:     "owner client not found",
		},
		{
			name:       "work code not found",
			repo:       &fakeCompanyRepo{workCodeExists: false},
			userClient: &fakeUserClient{},
			req:        req,
			expectErr:  true,
			errMsg:     "work code not found",
		},
		{
			name:       "work code repo error",
			repo:       &fakeCompanyRepo{workCodeErr: fmt.Errorf("db error")},
			userClient: &fakeUserClient{},
			req:        req,
			expectErr:  true,
		},
		{
			name:       "registration number already exists",
			repo:       &fakeCompanyRepo{workCodeExists: true, registrationNumExists: true},
			userClient: &fakeUserClient{},
			req:        req,
			expectErr:  true,
			errMsg:     "registration number already exists",
		},
		{
			name:       "registration number repo error",
			repo:       &fakeCompanyRepo{workCodeExists: true, registrationNumErr: fmt.Errorf("db error")},
			userClient: &fakeUserClient{},
			req:        req,
			expectErr:  true,
		},
		{
			name:       "tax number already exists",
			repo:       &fakeCompanyRepo{workCodeExists: true, taxNumExists: true},
			userClient: &fakeUserClient{},
			req:        req,
			expectErr:  true,
			errMsg:     "tax number already exists",
		},
		{
			name:       "tax number repo error",
			repo:       &fakeCompanyRepo{workCodeExists: true, taxNumErr: fmt.Errorf("db error")},
			userClient: &fakeUserClient{},
			req:        req,
			expectErr:  true,
		},
		{
			name:       "repo create fails",
			repo:       &fakeCompanyRepo{workCodeExists: true, createErr: fmt.Errorf("db error")},
			userClient: &fakeUserClient{},
			req:        req,
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewCompanyService(tt.repo, tt.userClient, nil, nil)

			company, err := svc.Create(context.Background(), tt.req)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, company)
				require.Equal(t, tt.req.Name, company.Name)
				require.Equal(t, tt.req.RegistrationNumber, company.RegistrationNumber)
				require.Equal(t, tt.req.TaxNumber, company.TaxNumber)
				require.Equal(t, tt.req.WorkCodeID, company.WorkCodeID)
				require.Equal(t, tt.req.OwnerID, company.OwnerID)
			}
		})
	}
}

func TestGetWorkCodes(t *testing.T) {
	t.Parallel()

	expected := []model.WorkCode{
		{WorkCodeID: 1, Code: "62.0", Description: "Software development"},
		{WorkCodeID: 2, Code: "64.1", Description: "Banking"},
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		svc := NewCompanyService(&fakeCompanyRepo{workCodes: expected}, &fakeUserClient{}, nil, nil)

		workCodes, err := svc.GetWorkCodes(context.Background())

		require.NoError(t, err)
		require.Equal(t, expected, workCodes)
	})

	t.Run("repo error", func(t *testing.T) {
		t.Parallel()

		svc := NewCompanyService(&fakeCompanyRepo{workCodesErr: fmt.Errorf("db error")}, &fakeUserClient{}, nil, nil)

		workCodes, err := svc.GetWorkCodes(context.Background())

		require.Error(t, err)
		require.Nil(t, workCodes)
		require.Contains(t, err.Error(), "db error")
	})

	t.Run("cache hit", func(t *testing.T) {
		t.Parallel()

		repo := &fakeCompanyRepo{workCodes: []model.WorkCode{{WorkCodeID: 99, Code: "x", Description: "repo"}}}
		cache := &fakeWorkCodeCache{cached: expected, found: true}
		svc := NewCompanyService(repo, &fakeUserClient{}, nil, cache)

		workCodes, err := svc.GetWorkCodes(context.Background())

		require.NoError(t, err)
		require.Equal(t, expected, workCodes)
		require.Equal(t, 0, repo.getWorkCodesCalls)
		require.Equal(t, 1, cache.getCalls)
		require.Equal(t, 0, cache.setCalls)
	})

	t.Run("cache miss writes cache", func(t *testing.T) {
		t.Parallel()

		repo := &fakeCompanyRepo{workCodes: expected}
		cache := &fakeWorkCodeCache{found: false}
		svc := NewCompanyService(repo, &fakeUserClient{}, nil, cache)

		workCodes, err := svc.GetWorkCodes(context.Background())

		require.NoError(t, err)
		require.Equal(t, expected, workCodes)
		require.Equal(t, 1, repo.getWorkCodesCalls)
		require.Equal(t, 1, cache.getCalls)
		require.Equal(t, 1, cache.setCalls)
		require.Equal(t, expected, cache.lastSetWorkCodes)
	})

	t.Run("cache read error falls back to repo", func(t *testing.T) {
		t.Parallel()

		repo := &fakeCompanyRepo{workCodes: expected}
		cache := &fakeWorkCodeCache{getErr: fmt.Errorf("redis down")}
		svc := NewCompanyService(repo, &fakeUserClient{}, nil, cache)

		workCodes, err := svc.GetWorkCodes(context.Background())

		require.NoError(t, err)
		require.Equal(t, expected, workCodes)
		require.Equal(t, 1, repo.getWorkCodesCalls)
		require.Equal(t, 1, cache.getCalls)
		require.Equal(t, 1, cache.setCalls)
	})
}

func TestGetCompanies(t *testing.T) {
	t.Parallel()

	expected := []model.Company{
		{CompanyID: 1, Name: "Acme", RegistrationNumber: "12345678", TaxNumber: "123456789", WorkCodeID: 1, Address: "A", OwnerID: 10},
		{CompanyID: 2, Name: "Globex", RegistrationNumber: "87654321", TaxNumber: "987654321", WorkCodeID: 2, Address: "B", OwnerID: 11},
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		svc := NewCompanyService(&fakeCompanyRepo{companies: expected}, &fakeUserClient{}, nil, nil)

		companies, err := svc.GetCompanies(context.Background())

		require.NoError(t, err)
		require.Equal(t, expected, companies)
	})

	t.Run("repo error", func(t *testing.T) {
		t.Parallel()

		svc := NewCompanyService(&fakeCompanyRepo{companiesErr: fmt.Errorf("db error")}, &fakeUserClient{}, nil, nil)

		companies, err := svc.GetCompanies(context.Background())

		require.Error(t, err)
		require.Nil(t, companies)
		require.Contains(t, err.Error(), "db error")
	})
}
