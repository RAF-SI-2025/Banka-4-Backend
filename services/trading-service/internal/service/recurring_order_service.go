package service

import (
	"context"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
)

type RecurringOrderService struct {
	recurringOrderRepo repository.RecurringOrderRepository
	listingRepo        repository.ListingRepository
}

func NewRecurringOrderService(
	recurringOrderRepo repository.RecurringOrderRepository,
	listingRepo repository.ListingRepository,
) *RecurringOrderService {
	return &RecurringOrderService{
		recurringOrderRepo: recurringOrderRepo,
		listingRepo:        listingRepo,
	}
}

func (s *RecurringOrderService) CreateRecurringOrder(ctx context.Context, req dto.CreateRecurringOrderRequest) (*model.RecurringOrder, error) {
	authCtx := auth.GetAuthFromContext(ctx)
	if authCtx == nil {
		return nil, errors.UnauthorizedErr("not authenticated")
	}

	listing, err := s.listingRepo.FindByID(ctx, req.ListingID, 0)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if listing == nil {
		return nil, errors.NotFoundErr("listing not found")
	}

	ownerType := model.OwnerTypeClient
	userID := authCtx.ClientID
	if authCtx.IdentityType == auth.IdentityEmployee {
		ownerType = model.OwnerTypeActuary
		userID = authCtx.EmployeeID
	}
	if userID == nil {
		return nil, errors.UnauthorizedErr("not authenticated")
	}

	ro := &model.RecurringOrder{
		UserID:        *userID,
		OwnerType:     ownerType,
		ListingID:     req.ListingID,
		Direction:     req.Direction,
		Mode:          req.Mode,
		Value:         req.Value,
		AccountNumber: req.AccountNumber,
		Cadence:       req.Cadence,
		NextRun:       nextRunTime(req.Cadence, time.Now()),
		Active:        true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := s.recurringOrderRepo.Create(ctx, ro); err != nil {
		return nil, errors.InternalErr(err)
	}

	return ro, nil
}

func (s *RecurringOrderService) DeleteRecurringOrder(ctx context.Context, id uint) error {
	authCtx := auth.GetAuthFromContext(ctx)
	if authCtx == nil {
		return errors.UnauthorizedErr("not authenticated")
	}

	ro, err := s.recurringOrderRepo.FindByID(ctx, id)
	if err != nil {
		return errors.InternalErr(err)
	}
	if ro == nil {
		return errors.NotFoundErr("recurring order not found")
	}

	if !ownsRecurringOrder(authCtx, ro) {
		return errors.ForbiddenErr("you do not own this recurring order")
	}

	if err := s.recurringOrderRepo.Delete(ctx, id); err != nil {
		return errors.InternalErr(err)
	}

	return nil
}

func (s *RecurringOrderService) GetMyRecurringOrders(ctx context.Context) ([]model.RecurringOrder, error) {
	authCtx := auth.GetAuthFromContext(ctx)
	if authCtx == nil {
		return nil, errors.UnauthorizedErr("not authenticated")
	}

	ownerType := model.OwnerTypeClient
	userID := authCtx.ClientID
	if authCtx.IdentityType == auth.IdentityEmployee {
		ownerType = model.OwnerTypeActuary
		userID = authCtx.EmployeeID
	}
	if userID == nil {
		return nil, errors.UnauthorizedErr("not authenticated")
	}

	orders, err := s.recurringOrderRepo.FindByUser(ctx, *userID, ownerType)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	return orders, nil
}

func (s *RecurringOrderService) PauseRecurringOrder(ctx context.Context, id uint) (*model.RecurringOrder, error) {
	authCtx := auth.GetAuthFromContext(ctx)
	if authCtx == nil {
		return nil, errors.UnauthorizedErr("not authenticated")
	}

	ro, err := s.recurringOrderRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if ro == nil {
		return nil, errors.NotFoundErr("recurring order not found")
	}

	if !ownsRecurringOrder(authCtx, ro) {
		return nil, errors.ForbiddenErr("you do not own this recurring order")
	}

	ro.Active = !ro.Active
	ro.UpdatedAt = time.Now()

	if err := s.recurringOrderRepo.Save(ctx, ro); err != nil {
		return nil, errors.InternalErr(err)
	}

	return ro, nil
}

func ownsRecurringOrder(authCtx *auth.AuthContext, ro *model.RecurringOrder) bool {
	if authCtx.IdentityType == auth.IdentityClient {
		return authCtx.ClientID != nil && *authCtx.ClientID == ro.UserID && ro.OwnerType == model.OwnerTypeClient
	}
	if authCtx.IdentityType == auth.IdentityEmployee {
		return authCtx.EmployeeID != nil && *authCtx.EmployeeID == ro.UserID && ro.OwnerType == model.OwnerTypeActuary
	}
	return false
}

func nextRunTime(cadence model.RecurringOrderCadence, from time.Time) time.Time {
	switch cadence {
	case model.RecurringOrderCadenceDaily:
		return from.AddDate(0, 0, 1)
	case model.RecurringOrderCadenceWeekly:
		return from.AddDate(0, 0, 7)
	case model.RecurringOrderCadenceMonthly:
		return from.AddDate(0, 1, 0)
	default:
		return from.AddDate(0, 0, 1)
	}
}
