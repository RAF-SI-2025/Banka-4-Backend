package service

import (
	"context"

	"gorm.io/gorm"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/repository"
)

type CompanyService struct {
	repo          repository.CompanyRepository
	userClient    client.UserClient
	db            *gorm.DB
	workCodeCache WorkCodeCache
}

func NewCompanyService(
	repo repository.CompanyRepository,
	userClient client.UserClient,
	db *gorm.DB,
	workCodeCache WorkCodeCache,
) *CompanyService {
	return &CompanyService{
		repo:          repo,
		userClient:    userClient,
		db:            db,
		workCodeCache: workCodeCache,
	}
}

func (s *CompanyService) Create(ctx context.Context, req dto.CreateCompanyRequest) (*model.Company, error) {
	if _, err := s.userClient.GetClientByID(ctx, req.OwnerID); err != nil {
		return nil, errors.NotFoundErr("owner client not found")
	}

	workCodeExists, err := s.repo.WorkCodeExists(ctx, req.WorkCodeID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if !workCodeExists {
		return nil, errors.NotFoundErr("work code not found")
	}

	regNumExists, err := s.repo.RegistrationNumberExists(ctx, req.RegistrationNumber)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if regNumExists {
		return nil, errors.ConflictErr("registration number already exists")
	}

	taxNumExists, err := s.repo.TaxNumberExists(ctx, req.TaxNumber)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if taxNumExists {
		return nil, errors.ConflictErr("tax number already exists")
	}

	company := &model.Company{
		Name:               req.Name,
		RegistrationNumber: req.RegistrationNumber,
		TaxNumber:          req.TaxNumber,
		WorkCodeID:         req.WorkCodeID,
		Address:            req.Address,
		OwnerID:            req.OwnerID,
	}

	if err := s.repo.Create(ctx, company); err != nil {
		return nil, errors.InternalErr(err)
	}

	return company, nil
}

func (s *CompanyService) GetCompanies(ctx context.Context) ([]model.Company, error) {
	companies, err := s.repo.GetCompanies(ctx)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	return companies, nil
}

func (s *CompanyService) GetWorkCodes(ctx context.Context) ([]model.WorkCode, error) {
	if s.workCodeCache != nil {
		workCodes, found, err := s.workCodeCache.Get(ctx)
		if err == nil && found {
			return workCodes, nil
		}
	}

	workCodes, err := s.repo.GetWorkCodes(ctx)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	if s.workCodeCache != nil {
		_ = s.workCodeCache.Set(ctx, workCodes)
	}

	return workCodes, nil
}
