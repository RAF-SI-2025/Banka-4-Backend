package service

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/repository"
)

type PayeeService struct {
	repo        repository.PayeeRepository
	accountRepo repository.AccountRepository
}

func NewPayeeService(repo repository.PayeeRepository, accountRepo repository.AccountRepository) *PayeeService {
	return &PayeeService{
		repo:        repo,
		accountRepo: accountRepo,
	}
}

func (s *PayeeService) GetAll(ctx context.Context) ([]model.Payee, error) {
	ac := auth.GetAuthFromContext(ctx)
	if ac == nil || ac.ClientID == nil {
		return nil, errors.ForbiddenErr("not authenticated as client")
	}
	payees, err := s.repo.FindAllByClientID(ctx, *ac.ClientID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	return payees, nil
}

func (s *PayeeService) Create(ctx context.Context, req dto.CreatePayeeRequest) (*model.Payee, error) {
	ac := auth.GetAuthFromContext(ctx)
	if ac == nil || ac.ClientID == nil {
		return nil, errors.ForbiddenErr("not authenticated as client")
	}

	accExists, err := s.accountRepo.AccountNumberExists(ctx, req.AccountNumber)

	if err != nil {
		return nil, errors.InternalErr(err)
	}

	if !accExists {
		return nil, errors.BadRequestErr("invalid account number")
	}

	payee := &model.Payee{
		ClientID:      *ac.ClientID,
		Name:          req.Name,
		AccountNumber: req.AccountNumber,
	}

	if err := s.repo.Create(ctx, payee); err != nil {
		return nil, errors.InternalErr(err)
	}

	return payee, nil
}

func (s *PayeeService) Update(ctx context.Context, id uint, req dto.UpdatePayeeRequest) (*model.Payee, error) {
	ac := auth.GetAuthFromContext(ctx)
	if ac == nil || ac.ClientID == nil {
		return nil, errors.ForbiddenErr("not authenticated as client")
	}
	payee, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	if payee == nil {
		return nil, errors.NotFoundErr("payee not found")
	}

	if payee.ClientID != *ac.ClientID {
		return nil, errors.ForbiddenErr("not your payee")
	}

	if req.Name != "" {
		payee.Name = req.Name
	}

	if req.AccountNumber != "" {
		accExists, err := s.accountRepo.AccountNumberExists(ctx, req.AccountNumber)

		if err != nil {
			return nil, errors.InternalErr(err)
		}

		if !accExists {
			return nil, errors.BadRequestErr("invalid account number")
		}
		payee.AccountNumber = req.AccountNumber
	}

	if err := s.repo.Update(ctx, payee); err != nil {
		return nil, errors.InternalErr(err)
	}

	return payee, nil
}

func (s *PayeeService) Delete(ctx context.Context, id uint) error {
	ac := auth.GetAuthFromContext(ctx)
	if ac == nil || ac.ClientID == nil {
		return errors.ForbiddenErr("not authenticated as client")
	}
	payee, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return errors.InternalErr(err)
	}

	if payee == nil {
		return errors.NotFoundErr("payee not found")
	}

	if payee.ClientID != *ac.ClientID {
		return errors.ForbiddenErr("not your payee")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return errors.InternalErr(err)
	}

	return nil
}
