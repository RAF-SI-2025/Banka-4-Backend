package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

// ── Fake RecurringOrder Repository ───────────────────────────────

type fakeRecurringOrderRepo struct {
	created     *model.RecurringOrder
	createErr   error
	byID        *model.RecurringOrder
	findByIDErr error
	saved       *model.RecurringOrder
	saveErr     error
	deleteErr   error
	byUser      []model.RecurringOrder
	byUserErr   error
	due         []model.RecurringOrder
	dueErr      error
}

func (r *fakeRecurringOrderRepo) Create(_ context.Context, ro *model.RecurringOrder) error {
	r.created = ro
	return r.createErr
}

func (r *fakeRecurringOrderRepo) FindByID(_ context.Context, _ uint) (*model.RecurringOrder, error) {
	return r.byID, r.findByIDErr
}

func (r *fakeRecurringOrderRepo) Save(_ context.Context, ro *model.RecurringOrder) error {
	r.saved = ro
	return r.saveErr
}

func (r *fakeRecurringOrderRepo) Delete(_ context.Context, _ uint) error {
	return r.deleteErr
}

func (r *fakeRecurringOrderRepo) FindByUser(_ context.Context, _ uint, _ model.OwnerType) ([]model.RecurringOrder, error) {
	return r.byUser, r.byUserErr
}

func (r *fakeRecurringOrderRepo) FindDue(_ context.Context, _ time.Time) ([]model.RecurringOrder, error) {
	return r.due, r.dueErr
}

// ── Helpers ───────────────────────────────────────────────────────

func newTestRecurringOrderService(roRepo *fakeRecurringOrderRepo, listingRepo *fakeListingRepo) *RecurringOrderService {
	return NewRecurringOrderService(roRepo, listingRepo)
}

func testListing(id uint) *model.Listing {
	return &model.Listing{
		ListingID: id,
		Ask:       100.0,
		Asset: &model.Asset{
			Ticker:    "AAPL",
			Name:      "Apple Inc",
			AssetType: model.AssetTypeStock,
		},
	}
}

// ── CreateRecurringOrder Tests ────────────────────────────────────

func TestCreateRecurringOrder_Success_Client(t *testing.T) {
	roRepo := &fakeRecurringOrderRepo{}
	svc := newTestRecurringOrderService(roRepo, &fakeListingRepo{listing: testListing(1)})

	ctx := clientAuthCtx()
	req := dto.CreateRecurringOrderRequest{
		ListingID:     1,
		AccountNumber: "444000100000000001",
		Direction:     model.OrderDirectionBuy,
		Mode:          model.RecurringOrderModeByAmount,
		Value:         500.0,
		Cadence:       model.RecurringOrderCadenceWeekly,
	}

	ro, err := svc.CreateRecurringOrder(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, ro)
	require.Equal(t, model.OwnerTypeClient, ro.OwnerType)
	require.Equal(t, model.RecurringOrderCadenceWeekly, ro.Cadence)
	require.True(t, ro.Active)
	require.NotNil(t, roRepo.created)
}

func TestCreateRecurringOrder_Success_Employee(t *testing.T) {
	roRepo := &fakeRecurringOrderRepo{}
	svc := newTestRecurringOrderService(roRepo, &fakeListingRepo{listing: testListing(1)})

	ctx := employeeAuthCtx(20)
	req := dto.CreateRecurringOrderRequest{
		ListingID:     1,
		AccountNumber: "444000100000000001",
		Direction:     model.OrderDirectionSell,
		Mode:          model.RecurringOrderModeByQuantity,
		Value:         10.0,
		Cadence:       model.RecurringOrderCadenceMonthly,
	}

	ro, err := svc.CreateRecurringOrder(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, ro)
	require.Equal(t, model.OwnerTypeBank, ro.OwnerType)
	require.Equal(t, uint(20), ro.UserID)
}

func TestCreateRecurringOrder_NoAuth(t *testing.T) {
	svc := newTestRecurringOrderService(&fakeRecurringOrderRepo{}, &fakeListingRepo{})

	_, err := svc.CreateRecurringOrder(context.Background(), dto.CreateRecurringOrderRequest{ListingID: 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not authenticated")
}

func TestCreateRecurringOrder_ListingNotFound(t *testing.T) {
	svc := newTestRecurringOrderService(&fakeRecurringOrderRepo{}, &fakeListingRepo{listing: nil})

	ctx := clientAuthCtx()
	_, err := svc.CreateRecurringOrder(ctx, dto.CreateRecurringOrderRequest{ListingID: 99})
	require.Error(t, err)
	require.Contains(t, err.Error(), "listing not found")
}

func TestCreateRecurringOrder_ListingRepoError(t *testing.T) {
	svc := newTestRecurringOrderService(&fakeRecurringOrderRepo{}, &fakeListingRepo{findByIDErr: errors.New("db error")})

	ctx := clientAuthCtx()
	_, err := svc.CreateRecurringOrder(ctx, dto.CreateRecurringOrderRequest{ListingID: 1})
	require.Error(t, err)
}

func TestCreateRecurringOrder_RepoCreateError(t *testing.T) {
	roRepo := &fakeRecurringOrderRepo{createErr: errors.New("insert failed")}
	svc := newTestRecurringOrderService(roRepo, &fakeListingRepo{listing: testListing(1)})

	ctx := clientAuthCtx()
	_, err := svc.CreateRecurringOrder(ctx, dto.CreateRecurringOrderRequest{
		ListingID:     1,
		AccountNumber: "444000100000000001",
		Direction:     model.OrderDirectionBuy,
		Mode:          model.RecurringOrderModeByAmount,
		Value:         100.0,
		Cadence:       model.RecurringOrderCadenceDaily,
	})
	require.Error(t, err)
}

// ── DeleteRecurringOrder Tests ────────────────────────────────────

func TestDeleteRecurringOrder_Success_Client(t *testing.T) {
	clientID := uint(10)
	ro := &model.RecurringOrder{
		RecurringOrderID: 1,
		UserID:           clientID,
		OwnerType:        model.OwnerTypeClient,
	}
	svc := newTestRecurringOrderService(&fakeRecurringOrderRepo{byID: ro}, &fakeListingRepo{})

	ctx := clientAuthCtx()
	err := svc.DeleteRecurringOrder(ctx, 1)
	require.NoError(t, err)
}

func TestDeleteRecurringOrder_NoAuth(t *testing.T) {
	svc := newTestRecurringOrderService(&fakeRecurringOrderRepo{}, &fakeListingRepo{})

	err := svc.DeleteRecurringOrder(context.Background(), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not authenticated")
}

func TestDeleteRecurringOrder_NotFound(t *testing.T) {
	svc := newTestRecurringOrderService(&fakeRecurringOrderRepo{byID: nil}, &fakeListingRepo{})

	ctx := clientAuthCtx()
	err := svc.DeleteRecurringOrder(ctx, 99)
	require.Error(t, err)
	require.Contains(t, err.Error(), "recurring order not found")
}

func TestDeleteRecurringOrder_NotOwner(t *testing.T) {
	ro := &model.RecurringOrder{
		RecurringOrderID: 1,
		UserID:           999,
		OwnerType:        model.OwnerTypeClient,
	}
	svc := newTestRecurringOrderService(&fakeRecurringOrderRepo{byID: ro}, &fakeListingRepo{})

	ctx := clientAuthCtx()
	err := svc.DeleteRecurringOrder(ctx, 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "do not own")
}

func TestDeleteRecurringOrder_RepoDeleteError(t *testing.T) {
	clientID := uint(10)
	ro := &model.RecurringOrder{
		RecurringOrderID: 1,
		UserID:           clientID,
		OwnerType:        model.OwnerTypeClient,
	}
	roRepo := &fakeRecurringOrderRepo{byID: ro, deleteErr: errors.New("delete failed")}
	svc := newTestRecurringOrderService(roRepo, &fakeListingRepo{})

	ctx := clientAuthCtx()
	err := svc.DeleteRecurringOrder(ctx, 1)
	require.Error(t, err)
}

// ── GetMyRecurringOrders Tests ────────────────────────────────────

func TestGetMyRecurringOrders_Success(t *testing.T) {
	orders := []model.RecurringOrder{
		{RecurringOrderID: 1, UserID: 10, OwnerType: model.OwnerTypeClient},
		{RecurringOrderID: 2, UserID: 10, OwnerType: model.OwnerTypeClient},
	}
	svc := newTestRecurringOrderService(&fakeRecurringOrderRepo{byUser: orders}, &fakeListingRepo{})

	ctx := clientAuthCtx()
	result, err := svc.GetMyRecurringOrders(ctx)
	require.NoError(t, err)
	require.Len(t, result, 2)
}

func TestGetMyRecurringOrders_Empty(t *testing.T) {
	svc := newTestRecurringOrderService(&fakeRecurringOrderRepo{byUser: []model.RecurringOrder{}}, &fakeListingRepo{})

	ctx := clientAuthCtx()
	result, err := svc.GetMyRecurringOrders(ctx)
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestGetMyRecurringOrders_NoAuth(t *testing.T) {
	svc := newTestRecurringOrderService(&fakeRecurringOrderRepo{}, &fakeListingRepo{})

	_, err := svc.GetMyRecurringOrders(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "not authenticated")
}

func TestGetMyRecurringOrders_RepoError(t *testing.T) {
	svc := newTestRecurringOrderService(&fakeRecurringOrderRepo{byUserErr: errors.New("db error")}, &fakeListingRepo{})

	ctx := clientAuthCtx()
	_, err := svc.GetMyRecurringOrders(ctx)
	require.Error(t, err)
}

// ── PauseRecurringOrder Tests ─────────────────────────────────────

func TestPauseRecurringOrder_TogglesActiveToFalse(t *testing.T) {
	clientID := uint(10)
	ro := &model.RecurringOrder{
		RecurringOrderID: 1,
		UserID:           clientID,
		OwnerType:        model.OwnerTypeClient,
		Active:           true,
	}
	svc := newTestRecurringOrderService(&fakeRecurringOrderRepo{byID: ro}, &fakeListingRepo{})

	ctx := clientAuthCtx()
	result, err := svc.PauseRecurringOrder(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.Active)
}

func TestPauseRecurringOrder_TogglesActiveToTrue(t *testing.T) {
	clientID := uint(10)
	ro := &model.RecurringOrder{
		RecurringOrderID: 1,
		UserID:           clientID,
		OwnerType:        model.OwnerTypeClient,
		Active:           false,
	}
	svc := newTestRecurringOrderService(&fakeRecurringOrderRepo{byID: ro}, &fakeListingRepo{})

	ctx := clientAuthCtx()
	result, err := svc.PauseRecurringOrder(ctx, 1)
	require.NoError(t, err)
	require.True(t, result.Active)
}

func TestPauseRecurringOrder_NoAuth(t *testing.T) {
	svc := newTestRecurringOrderService(&fakeRecurringOrderRepo{}, &fakeListingRepo{})

	_, err := svc.PauseRecurringOrder(context.Background(), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not authenticated")
}

func TestPauseRecurringOrder_NotFound(t *testing.T) {
	svc := newTestRecurringOrderService(&fakeRecurringOrderRepo{byID: nil}, &fakeListingRepo{})

	ctx := clientAuthCtx()
	_, err := svc.PauseRecurringOrder(ctx, 99)
	require.Error(t, err)
	require.Contains(t, err.Error(), "recurring order not found")
}

func TestPauseRecurringOrder_NotOwner(t *testing.T) {
	ro := &model.RecurringOrder{
		RecurringOrderID: 1,
		UserID:           999,
		OwnerType:        model.OwnerTypeClient,
	}
	svc := newTestRecurringOrderService(&fakeRecurringOrderRepo{byID: ro}, &fakeListingRepo{})

	ctx := clientAuthCtx()
	_, err := svc.PauseRecurringOrder(ctx, 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "do not own")
}

// ── Pure function tests ───────────────────────────────────────────

func TestNextRunTime_Daily(t *testing.T) {
	from := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	require.Equal(t, from.AddDate(0, 0, 1), nextRunTime(model.RecurringOrderCadenceDaily, from))
}

func TestNextRunTime_Weekly(t *testing.T) {
	from := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	require.Equal(t, from.AddDate(0, 0, 7), nextRunTime(model.RecurringOrderCadenceWeekly, from))
}

func TestNextRunTime_Monthly(t *testing.T) {
	from := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	require.Equal(t, from.AddDate(0, 1, 0), nextRunTime(model.RecurringOrderCadenceMonthly, from))
}

func TestNextRunTime_UnknownDefaultsToDaily(t *testing.T) {
	from := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	require.Equal(t, from.AddDate(0, 0, 1), nextRunTime("UNKNOWN", from))
}

func TestOwnsRecurringOrder_Client_Owns(t *testing.T) {
	cid := uint(10)
	authCtx := &auth.AuthContext{IdentityType: auth.IdentityClient, ClientID: &cid}
	ro := &model.RecurringOrder{UserID: 10, OwnerType: model.OwnerTypeClient}
	require.True(t, ownsRecurringOrder(authCtx, ro))
}

func TestOwnsRecurringOrder_Client_WrongUser(t *testing.T) {
	cid := uint(10)
	authCtx := &auth.AuthContext{IdentityType: auth.IdentityClient, ClientID: &cid}
	ro := &model.RecurringOrder{UserID: 99, OwnerType: model.OwnerTypeClient}
	require.False(t, ownsRecurringOrder(authCtx, ro))
}

func TestOwnsRecurringOrder_Employee_Owns(t *testing.T) {
	eid := uint(20)
	authCtx := &auth.AuthContext{IdentityType: auth.IdentityEmployee, EmployeeID: &eid}
	ro := &model.RecurringOrder{UserID: 20, OwnerType: model.OwnerTypeBank}
	require.True(t, ownsRecurringOrder(authCtx, ro))
}

func TestOwnsRecurringOrder_Employee_WrongOwnerType(t *testing.T) {
	eid := uint(20)
	authCtx := &auth.AuthContext{IdentityType: auth.IdentityEmployee, EmployeeID: &eid}
	ro := &model.RecurringOrder{UserID: 20, OwnerType: model.OwnerTypeClient}
	require.False(t, ownsRecurringOrder(authCtx, ro))
}

// ── ResolveQuantity Tests (scheduler) ────────────────────────────

func TestResolveQuantity_ByQuantity(t *testing.T) {
	scheduler := &RecurringOrderScheduler{}
	ro := &model.RecurringOrder{
		Mode:  model.RecurringOrderModeByQuantity,
		Value: 7.8,
	}
	qty, ok := scheduler.resolveQuantity(ro)
	require.True(t, ok)
	require.Equal(t, uint(8), qty) // math.Round(7.8)
}

func TestResolveQuantity_ByAmount_WithPrice(t *testing.T) {
	scheduler := &RecurringOrderScheduler{}
	ro := &model.RecurringOrder{
		Mode:  model.RecurringOrderModeByAmount,
		Value: 500.0,
		Listing: model.Listing{
			ListingID: 1,
			Ask:       100.0,
		},
	}
	qty, ok := scheduler.resolveQuantity(ro)
	require.True(t, ok)
	require.Equal(t, uint(5), qty) // floor(500/100)
}

func TestResolveQuantity_ByAmount_NoListing(t *testing.T) {
	scheduler := &RecurringOrderScheduler{}
	ro := &model.RecurringOrder{
		Mode:  model.RecurringOrderModeByAmount,
		Value: 500.0,
	}
	_, ok := scheduler.resolveQuantity(ro)
	require.False(t, ok)
}

func TestResolveQuantity_ByAmount_ZeroAsk(t *testing.T) {
	scheduler := &RecurringOrderScheduler{}
	ro := &model.RecurringOrder{
		Mode:  model.RecurringOrderModeByAmount,
		Value: 500.0,
		Listing: model.Listing{
			ListingID: 1,
			Ask:       0,
		},
	}
	_, ok := scheduler.resolveQuantity(ro)
	require.False(t, ok)
}

func TestResolveQuantity_ByAmount_InsufficientFunds(t *testing.T) {
	scheduler := &RecurringOrderScheduler{}
	ro := &model.RecurringOrder{
		Mode:  model.RecurringOrderModeByAmount,
		Value: 50.0,
		Listing: model.Listing{
			ListingID: 1,
			Ask:       100.0,
		},
	}
	qty, ok := scheduler.resolveQuantity(ro)
	require.True(t, ok)
	require.Equal(t, uint(0), qty) // floor(50/100) = 0
}
