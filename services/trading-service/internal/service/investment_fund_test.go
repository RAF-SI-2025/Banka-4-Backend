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
	findByIDResult      *model.InvestmentFund
	findByIDErr         error
	findByNameResult    *model.InvestmentFund
	findByNameErr       error
	createErr           error
	created             *model.InvestmentFund
	findAllResult       []model.InvestmentFund
	findAllTotal        int64
	findAllErr          error
	findByManagerResult []model.InvestmentFund
	findByManagerErr    error
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

func (f *fakeFundRepo) GetAllInvestmentFunds(ctx context.Context) ([]model.InvestmentFund, error) {
	return f.findAllResult, f.findAllErr
}

func (f *fakeFundRepo) FindAll(ctx context.Context, name, sortBy, sortDir string, page, pageSize int) ([]model.InvestmentFund, int64, error) {
	return f.findAllResult, f.findAllTotal, f.findAllErr
}

func (f *fakeFundRepo) FindByManagerID(ctx context.Context, managerID uint) ([]model.InvestmentFund, error) {
	return f.findByManagerResult, f.findByManagerErr
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

type fakeFundUserClient struct {
	// configurable responses
	getClientByIdResp *pb.GetClientByIdResponse
	getClientByIdErr  error

	getEmployeeByIdResp *pb.GetEmployeeByIdResponse
	getEmployeeByIdErr  error

	getAllClientsResp *pb.GetAllClientsResponse
	getAllClientsErr  error

	getAllActuariesResp *pb.GetAllActuariesResponse
	getAllActuariesErr  error

	getIdentityByUserIdResp *pb.GetIdentityByUserIdResponse
	getIdentityByUserIdErr  error
}

func (f *fakeFundUserClient) GetClientById(_ context.Context, _ uint64) (*pb.GetClientByIdResponse, error) {
	return f.getClientByIdResp, f.getClientByIdErr
}

func (f *fakeFundUserClient) GetClientByIdentityId(_ context.Context, _ uint64) (*pb.GetClientByIdResponse, error) {
	return f.getClientByIdResp, f.getClientByIdErr
}

func (f *fakeFundUserClient) GetEmployeeById(_ context.Context, _ uint64) (*pb.GetEmployeeByIdResponse, error) {
	return f.getEmployeeByIdResp, f.getEmployeeByIdErr
}

func (f *fakeFundUserClient) GetEmployeeByIdentityId(_ context.Context, _ uint64) (*pb.GetEmployeeByIdResponse, error) {
	return f.getEmployeeByIdResp, f.getEmployeeByIdErr
}

func (f *fakeFundUserClient) GetAllClients(_ context.Context, _, _ int32, _, _ string) (*pb.GetAllClientsResponse, error) {
	return f.getAllClientsResp, f.getAllClientsErr
}

func (f *fakeFundUserClient) GetAllActuaries(_ context.Context, _, _ int32, _, _ string) (*pb.GetAllActuariesResponse, error) {
	return f.getAllActuariesResp, f.getAllActuariesErr
}

func (f *fakeFundUserClient) GetIdentityByUserId(_ context.Context, _ uint64, _ string) (*pb.GetIdentityByUserIdResponse, error) {
	return f.getIdentityByUserIdResp, f.getIdentityByUserIdErr
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

func newTestFundService(fundRepo *fakeFundRepo, ownershipRepo *fakeAssetOwnershipRepo, listingRepo *fakeListingRepo, bankingClient *fakeFundBankingClient, userClient *fakeFundUserClient) *InvestmentFundService {
	return NewInvestmentFundService(
		fundRepo,
		&fakePositionRepo{},
		&fakeInvestmentRepo{},
		ownershipRepo,
		listingRepo,
		&fakeStockRepo{},
		&fakeOptionRepo{},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		bankingClient,
		userClient,
	)
}

// ── CreateFund tests ──────────────────────────────────────────────

func TestCreateFund_Success(t *testing.T) {
	fundRepo := &fakeFundRepo{}
	bankingClient := &fakeFundBankingClient{createdAccountNumber: "444000112345678901"}
	svc := newTestFundService(fundRepo, &fakeAssetOwnershipRepo{}, &fakeListingRepo{}, bankingClient, &fakeFundUserClient{})

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
	svc := newTestFundService(&fakeFundRepo{}, &fakeAssetOwnershipRepo{}, &fakeListingRepo{}, &fakeFundBankingClient{}, &fakeFundUserClient{})

	_, err := svc.CreateFund(context.Background(), validFundRequest())

	require.Error(t, err)
	require.Contains(t, err.Error(), "not authenticated")
}

func TestCreateFund_NotEmployee(t *testing.T) {
	svc := newTestFundService(&fakeFundRepo{}, &fakeAssetOwnershipRepo{}, &fakeListingRepo{}, &fakeFundBankingClient{}, &fakeFundUserClient{})

	_, err := svc.CreateFund(fundClientCtx(), validFundRequest())

	require.Error(t, err)
	require.Contains(t, err.Error(), "only employees")
}

func TestCreateFund_DuplicateName(t *testing.T) {
	fundRepo := &fakeFundRepo{
		findByNameResult: &model.InvestmentFund{Name: "Alpha Growth Fund"},
	}
	svc := newTestFundService(fundRepo, &fakeAssetOwnershipRepo{}, &fakeListingRepo{}, &fakeFundBankingClient{}, &fakeFundUserClient{})

	_, err := svc.CreateFund(fundSupervisorCtx(), validFundRequest())

	require.Error(t, err)
	require.Contains(t, err.Error(), "already taken")
}

func TestCreateFund_FindByNameRepoError(t *testing.T) {
	fundRepo := &fakeFundRepo{
		findByNameErr: errors.New("db error"),
	}
	svc := newTestFundService(fundRepo, &fakeAssetOwnershipRepo{}, &fakeListingRepo{}, &fakeFundBankingClient{}, &fakeFundUserClient{})

	_, err := svc.CreateFund(fundSupervisorCtx(), validFundRequest())

	require.Error(t, err)
}

func TestCreateFund_BankingClientError(t *testing.T) {
	fundRepo := &fakeFundRepo{}
	bankingClient := &fakeFundBankingClient{
		createFundAccountErr: fmt.Errorf("banking service unavailable"),
	}
	svc := newTestFundService(fundRepo, &fakeAssetOwnershipRepo{}, &fakeListingRepo{}, bankingClient, &fakeFundUserClient{})

	_, err := svc.CreateFund(fundSupervisorCtx(), validFundRequest())

	require.Error(t, err)
}

func TestCreateFund_RepoCreateError(t *testing.T) {
	fundRepo := &fakeFundRepo{
		createErr: errors.New("db error"),
	}
	bankingClient := &fakeFundBankingClient{createdAccountNumber: "444000112345678901"}
	svc := newTestFundService(fundRepo, &fakeAssetOwnershipRepo{}, &fakeListingRepo{}, bankingClient, &fakeFundUserClient{})

	_, err := svc.CreateFund(fundSupervisorCtx(), validFundRequest())

	require.Error(t, err)
}
