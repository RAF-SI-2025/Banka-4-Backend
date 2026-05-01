package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/stretchr/testify/require"
)

// ── Fake Fund Repo ────────────────────────────────────────────────

type fakeFundRepo struct {
	findByIDResult   *model.InvestmentFund
	findByIDErr      error
	findByNameResult *model.InvestmentFund
	findByNameErr    error
	createErr        error
	created          *model.InvestmentFund
}

func (f *fakeFundRepo) FindByName(ctx context.Context, name string) (*model.InvestmentFund, error) {
	return f.findByNameResult, f.findByNameErr
}

func (f *fakeFundRepo) FindByID(ctx context.Context, id uint) (*model.InvestmentFund, error) {
	return f.findByIDResult, f.findByIDErr
}

func (f *fakeFundRepo) FindByAccountNumber(ctx context.Context, accountNumber string) (*model.InvestmentFund, error) {
	return nil, nil
}

func (f *fakeFundRepo) Create(ctx context.Context, fund *model.InvestmentFund) error {
	if f.createErr != nil {
		return f.createErr
	}
	fund.FundID = 1
	f.created = fund
	return nil
}

// ── Fake ClientFundPosition Repo ──────────────────────────────────

type fakePositionRepo struct {
	findResult      *model.ClientFundPosition
	findErr         error
	findByClientRes []model.ClientFundPosition
	findByClientErr error
	findByFundRes   []model.ClientFundPosition
	findByFundErr   error
	upsertErr       error
}

func (f *fakePositionRepo) FindByClientAndFund(ctx context.Context, clientID uint, ownerType model.OwnerType, fundID uint) (*model.ClientFundPosition, error) {
	return f.findResult, f.findErr
}

func (f *fakePositionRepo) FindByClient(ctx context.Context, clientID uint, ownerType model.OwnerType) ([]model.ClientFundPosition, error) {
	return f.findByClientRes, f.findByClientErr
}

func (f *fakePositionRepo) FindByFund(ctx context.Context, fundID uint) ([]model.ClientFundPosition, error) {
	return f.findByFundRes, f.findByFundErr
}

func (f *fakePositionRepo) Upsert(ctx context.Context, position *model.ClientFundPosition) error {
	return f.upsertErr
}

// ── Fake ClientFundInvestment Repo ────────────────────────────────

type fakeInvestmentRepo struct {
	createErr error
}

func (f *fakeInvestmentRepo) Create(ctx context.Context, investment *model.ClientFundInvestment) error {
	return f.createErr
}

func (f *fakeInvestmentRepo) FindByClientAndFund(ctx context.Context, clientID uint, ownerType model.OwnerType, fundID uint) ([]model.ClientFundInvestment, error) {
	return nil, nil
}

// ── Fake Fund Banking Client ──────────────────────────────────────

type fakeFundBankingClient struct {
	createdAccountNumber string
	createFundAccountErr error
	getAccountResult     *pb.GetAccountByNumberResponse
	tradeSettlementErr   error
}

func (f *fakeFundBankingClient) GetAccountByNumber(_ context.Context, _ string) (*pb.GetAccountByNumberResponse, error) {
	if f.getAccountResult != nil {
		return f.getAccountResult, nil
	}
	return nil, nil
}
func (f *fakeFundBankingClient) HasActiveLoan(_ context.Context, _ uint64) (*pb.HasActiveLoanResponse, error) {
	return nil, nil
}
func (f *fakeFundBankingClient) CreatePaymentWithoutVerification(_ context.Context, _ *pb.CreatePaymentRequest) (*pb.CreatePaymentResponse, error) {
	return nil, nil
}
func (f *fakeFundBankingClient) GetAccountsByClientID(_ context.Context, _ uint64) (*pb.GetAccountsByClientIDResponse, error) {
	return nil, nil
}
func (f *fakeFundBankingClient) ConvertCurrency(_ context.Context, amount float64, _, _ string) (float64, error) {
	return amount, nil
}
func (f *fakeFundBankingClient) ExecuteTradeSettlement(_ context.Context, _, _ string, _ pb.TradeSettlementDirection, _ float64) (*pb.ExecuteTradeSettlementResponse, error) {
	if f.tradeSettlementErr != nil {
		return nil, f.tradeSettlementErr
	}
	return &pb.ExecuteTradeSettlementResponse{}, nil
}
func (f *fakeFundBankingClient) GetAccountCurrency(_ context.Context, _ string) (string, error) {
	return "RSD", nil
}
func (f *fakeFundBankingClient) CreateFundAccount(_ context.Context, _ string, _ uint64) (string, error) {
	return f.createdAccountNumber, f.createFundAccountErr
}

// ── Helpers ───────────────────────────────────────────────────────

func fundSupervisorCtx() context.Context {
	employeeID := uint(25)
	return auth.SetAuthOnContext(context.Background(), &auth.AuthContext{
		IdentityID:   200,
		IdentityType: auth.IdentityEmployee,
		EmployeeID:   &employeeID,
	})
}

func fundClientCtx() context.Context {
	clientID := uint(99)
	return auth.SetAuthOnContext(context.Background(), &auth.AuthContext{
		IdentityID:   1,
		IdentityType: auth.IdentityClient,
		ClientID:     &clientID,
	})
}

func validFundRequest() dto.CreateFundRequest {
	return dto.CreateFundRequest{
		Name:                "Alpha Growth Fund",
		Description:         "Fund focused on the IT sector.",
		MinimumContribution: 1000.00,
	}
}

func newTestFundService(fundRepo *fakeFundRepo, bankingClient *fakeFundBankingClient) *InvestmentFundService {
	return NewInvestmentFundService(
		fundRepo,
		&fakePositionRepo{},
		&fakeInvestmentRepo{},
		&fakeAssetOwnershipRepo{},
		&fakeStockRepo{},
		&fakeOptionRepo{},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		bankingClient,
	)
}

func newTestFundServiceFull(
	fundRepo *fakeFundRepo,
	positionRepo *fakePositionRepo,
	ownershipRepo *fakeAssetOwnershipRepo,
	stockRepo *fakeStockRepo,
	bankingClient *fakeFundBankingClient,
) *InvestmentFundService {
	return NewInvestmentFundService(
		fundRepo,
		positionRepo,
		&fakeInvestmentRepo{},
		ownershipRepo,
		stockRepo,
		&fakeOptionRepo{},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		bankingClient,
	)
}

// ── Tests: CreateFund ─────────────────────────────────────────────

func TestCreateFund_Success(t *testing.T) {
	fundRepo := &fakeFundRepo{}
	bankingClient := &fakeFundBankingClient{createdAccountNumber: "444000112345678901"}
	svc := newTestFundService(fundRepo, bankingClient)

	resp, err := svc.CreateFund(fundSupervisorCtx(), validFundRequest())

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "Alpha Growth Fund", resp.Name)
	require.Equal(t, "444000112345678901", resp.AccountNumber)
	require.Equal(t, uint(25), resp.ManagerID)
	require.Equal(t, 1000.00, resp.MinimumContribution)
	require.WithinDuration(t, time.Now(), resp.CreatedAt, 5*time.Second)
}

func TestCreateFund_Unauthenticated(t *testing.T) {
	svc := newTestFundService(&fakeFundRepo{}, &fakeFundBankingClient{})

	_, err := svc.CreateFund(context.Background(), validFundRequest())

	require.Error(t, err)
	require.Contains(t, err.Error(), "not authenticated")
}

func TestCreateFund_NotEmployee(t *testing.T) {
	svc := newTestFundService(&fakeFundRepo{}, &fakeFundBankingClient{})

	_, err := svc.CreateFund(fundClientCtx(), validFundRequest())

	require.Error(t, err)
	require.Contains(t, err.Error(), "only employees")
}

func TestCreateFund_DuplicateName(t *testing.T) {
	fundRepo := &fakeFundRepo{
		findByNameResult: &model.InvestmentFund{Name: "Alpha Growth Fund"},
	}
	svc := newTestFundService(fundRepo, &fakeFundBankingClient{})

	_, err := svc.CreateFund(fundSupervisorCtx(), validFundRequest())

	require.Error(t, err)
	require.Contains(t, err.Error(), "already taken")
}

func TestCreateFund_FindByNameRepoError(t *testing.T) {
	fundRepo := &fakeFundRepo{
		findByNameErr: errors.New("db error"),
	}
	svc := newTestFundService(fundRepo, &fakeFundBankingClient{})

	_, err := svc.CreateFund(fundSupervisorCtx(), validFundRequest())

	require.Error(t, err)
}

func TestCreateFund_BankingClientError(t *testing.T) {
	fundRepo := &fakeFundRepo{}
	bankingClient := &fakeFundBankingClient{
		createFundAccountErr: fmt.Errorf("banking service unavailable"),
	}
	svc := newTestFundService(fundRepo, bankingClient)

	_, err := svc.CreateFund(fundSupervisorCtx(), validFundRequest())

	require.Error(t, err)
}

func TestCreateFund_RepoCreateError(t *testing.T) {
	fundRepo := &fakeFundRepo{
		createErr: errors.New("db error"),
	}
	bankingClient := &fakeFundBankingClient{createdAccountNumber: "444000112345678901"}
	svc := newTestFundService(fundRepo, bankingClient)

	_, err := svc.CreateFund(fundSupervisorCtx(), validFundRequest())

	require.Error(t, err)
}

// ── Tests: GetClientFundPositions ─────────────────────────────────

func TestGetClientFundPositions_Empty(t *testing.T) {
	svc := newTestFundServiceFull(
		&fakeFundRepo{},
		&fakePositionRepo{findByClientRes: nil},
		&fakeAssetOwnershipRepo{},
		&fakeStockRepo{},
		&fakeFundBankingClient{},
	)

	result, err := svc.GetClientFundPositions(context.Background(), 99)

	require.NoError(t, err)
	require.Empty(t, result)
}

func TestGetClientFundPositions_PositionRepoError(t *testing.T) {
	svc := newTestFundServiceFull(
		&fakeFundRepo{},
		&fakePositionRepo{findByClientErr: errors.New("db error")},
		&fakeAssetOwnershipRepo{},
		&fakeStockRepo{},
		&fakeFundBankingClient{},
	)

	_, err := svc.GetClientFundPositions(context.Background(), 1)

	require.Error(t, err)
}

func TestGetClientFundPositions_ZeroTotalInvested(t *testing.T) {
	// Fund has one other position with 0 total invested (edge: avoid divide-by-zero)
	fund := &model.InvestmentFund{FundID: 1, Name: "Test Fund", Description: "desc"}
	pos := model.ClientFundPosition{
		ClientID:            1,
		OwnerType:           model.OwnerTypeClient,
		FundID:              1,
		Fund:                fund,
		TotalInvestedAmount: 1000,
	}
	// FindByFund returns only this position, so fundTotalInvested = 1000
	svc := newTestFundServiceFull(
		&fakeFundRepo{},
		&fakePositionRepo{
			findByClientRes: []model.ClientFundPosition{pos},
			findByFundRes:   []model.ClientFundPosition{pos},
		},
		&fakeAssetOwnershipRepo{},
		&fakeStockRepo{},
		&fakeFundBankingClient{},
	)

	result, err := svc.GetClientFundPositions(context.Background(), 1)

	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, uint(1), result[0].FundID)
	require.InDelta(t, 1.0, result[0].ClientsSharePercent, 0.001)  // sole investor = 100%
	require.InDelta(t, 0.0, result[0].ClientsShareValueRSD, 0.001) // fund holds no securities
	require.InDelta(t, -1000.0, result[0].TotalProfit, 0.001)
}

func TestGetClientFundPositions_ShareCalculation(t *testing.T) {
	// Client invested 500 out of 2000 total → 25% share
	// Fund holds no securities → shares value = 0, profit = -500
	fund := &model.InvestmentFund{FundID: 2, Name: "Growth Fund", Description: "desc"}
	clientPos := model.ClientFundPosition{
		ClientID: 5, OwnerType: model.OwnerTypeClient, FundID: 2, Fund: fund, TotalInvestedAmount: 500,
	}
	otherPos := model.ClientFundPosition{
		ClientID: 6, OwnerType: model.OwnerTypeClient, FundID: 2, Fund: fund, TotalInvestedAmount: 1500,
	}

	svc := newTestFundServiceFull(
		&fakeFundRepo{},
		&fakePositionRepo{
			findByClientRes: []model.ClientFundPosition{clientPos},
			findByFundRes:   []model.ClientFundPosition{clientPos, otherPos},
		},
		&fakeAssetOwnershipRepo{},
		&fakeStockRepo{},
		&fakeFundBankingClient{},
	)

	result, err := svc.GetClientFundPositions(context.Background(), 5)

	require.NoError(t, err)
	require.Len(t, result, 1)
	require.InDelta(t, 0.25, result[0].ClientsSharePercent, 0.001) // 500 / 2000 = 0.25
	require.InDelta(t, 0.0, result[0].ClientsShareValueRSD, 0.001)
	require.InDelta(t, -500.0, result[0].TotalProfit, 0.001)
}

func TestGetClientFundPositions_FindByFundError(t *testing.T) {
	fund := &model.InvestmentFund{FundID: 1, Name: "Fund", Description: "desc"}
	pos := model.ClientFundPosition{
		ClientID: 1, OwnerType: model.OwnerTypeClient, FundID: 1, Fund: fund, TotalInvestedAmount: 1000,
	}

	svc := newTestFundServiceFull(
		&fakeFundRepo{},
		&fakePositionRepo{
			findByClientRes: []model.ClientFundPosition{pos},
			findByFundErr:   errors.New("db error"),
		},
		&fakeAssetOwnershipRepo{},
		&fakeStockRepo{},
		&fakeFundBankingClient{},
	)

	_, err := svc.GetClientFundPositions(context.Background(), 1)

	require.Error(t, err)
}
