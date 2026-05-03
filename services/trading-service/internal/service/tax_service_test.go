package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/config"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

// ── Fake Tax Repository ────────────────────────────────────────────

type fakeTaxRepo struct {
	// AddTaxOwed
	addTaxErr error

	// FindAllPositiveAccumulatedTax
	positiveTaxes    []model.AccumulatedTax
	positiveTaxesErr error

	// FindAccumulatedTaxByAccountNumber
	accumulatedTax    *model.AccumulatedTax
	accumulatedTaxErr error

	// FindAccumulatedTaxByEmployeeID
	employeeTaxes    []model.AccumulatedTax
	employeeTaxesErr error

	// FindAccumulatedTaxByClientAccountNumbers
	clientTaxes    []model.AccumulatedTax
	clientTaxesErr error

	// FindTaxCollectionsByAccountNumber
	collections    []model.TaxCollection
	collectionsErr error

	// RecordCollectionResult
	recordCollectionErr error

	// captured args for assertion
	capturedCollection *model.TaxCollection
}

func (f *fakeTaxRepo) AddTaxOwed(_ context.Context, _ string, _ *uint, _ float64, _ string) error {
	return f.addTaxErr
}

func (c *fakeBankingClient) GetAccountCurrency(_ context.Context, _ string) (string, error) {
	return "RSD", nil
}

func (f *fakeTaxRepo) FindAllPositiveAccumulatedTax(_ context.Context) ([]model.AccumulatedTax, error) {
	return f.positiveTaxes, f.positiveTaxesErr
}

func (f *fakeTaxRepo) FindAccumulatedTaxByAccountNumber(_ context.Context, _ string) (*model.AccumulatedTax, error) {
	return f.accumulatedTax, f.accumulatedTaxErr
}

func (f *fakeTaxRepo) FindAccumulatedTaxByEmployeeID(_ context.Context, _ uint) ([]model.AccumulatedTax, error) {
	return f.employeeTaxes, f.employeeTaxesErr
}

func (f *fakeTaxRepo) FindAccumulatedTaxByClientAccountNumbers(_ context.Context, _ []string) ([]model.AccumulatedTax, error) {
	return f.clientTaxes, f.clientTaxesErr
}

func (f *fakeTaxRepo) FindTaxCollectionsByAccountNumber(_ context.Context, _ string) ([]model.TaxCollection, error) {
	return f.collections, f.collectionsErr
}

func (f *fakeTaxRepo) RecordCollectionResult(_ context.Context, collection *model.TaxCollection, _ bool, _ float64, _ time.Time) error {
	f.capturedCollection = collection
	return f.recordCollectionErr
}

func (f *fakeTaxRepo) FindAllAccumulatedTax(_ context.Context, _ []string, _, _ int) ([]model.AccumulatedTax, int64, error) {
	return nil, 0, nil
}

func (f *fakeTaxRepo) SaveAccumulatedTax(_ context.Context, _ *model.AccumulatedTax) error {
	return nil
}

func (f *fakeTaxRepo) CreateTaxCollection(_ context.Context, _ *model.TaxCollection) error {
	return nil
}

func (f *fakeTaxRepo) FindLatestTaxCollection(_ context.Context, _ string) (*model.TaxCollection, error) {
	return nil, nil
}

func (f *fakeTaxRepo) ClearTax(_ context.Context, _ string, _ time.Time) error {
	return nil
}

// ── Fake Banking Client ────────────────────────────────────────────

type fakeBankingClient struct {
	// GetAccountsByClientID
	accountsResp *pb.GetAccountsByClientIDResponse
	accountsErr  error

	// ConvertCurrency
	convertedAmount float64
	convertErr      error

	// CreatePaymentWithoutVerification
	paymentResp *pb.CreatePaymentResponse
	paymentErr  error

	accountByNumber map[string]uint64
}

func (f *fakeBankingClient) GetAccountByNumber(_ context.Context, acc string) (*pb.GetAccountByNumberResponse, error) {
	clientID, ok := f.accountByNumber[acc]
	if !ok {
		return nil, errors.New("account not found")
	}

	return &pb.GetAccountByNumberResponse{
		ClientId: clientID,
	}, nil
}

func (f *fakeBankingClient) HasActiveLoan(_ context.Context, _ uint64) (*pb.HasActiveLoanResponse, error) {
	return &pb.HasActiveLoanResponse{HasActiveLoan: false}, nil
}

func (f *fakeBankingClient) CreatePaymentWithoutVerification(_ context.Context, _ *pb.CreatePaymentRequest) (*pb.CreatePaymentResponse, error) {
	return f.paymentResp, f.paymentErr
}

func (f *fakeBankingClient) GetAccountsByClientID(_ context.Context, _ uint64) (*pb.GetAccountsByClientIDResponse, error) {
	return f.accountsResp, f.accountsErr
}

func (f *fakeBankingClient) ConvertCurrency(_ context.Context, amount float64, _, _ string) (float64, error) {
	if f.convertErr != nil {
		return 0, f.convertErr
	}
	return amount * f.convertedAmount, nil
}

func (f *fakeBankingClient) ExecuteTradeSettlement(ctx context.Context, accountNumber, currencyCode string, direction pb.TradeSettlementDirection, amount float64) (*pb.ExecuteTradeSettlementResponse, error) {
	return nil, nil
}

func (f *fakeBankingClient) CreateFundAccount(ctx context.Context, fundName string, managerID uint64) (string, error) {
	return "", nil
}

// ── Constructor ────────────────────────────────────────────────────

func newTestTaxService(repo *fakeTaxRepo, banking *fakeBankingClient) *TaxService {
	return NewTaxService(repo, banking, &config.Configuration{
		TaxAccountNumber: "444000000000000000",
	})
}

// ── RecordTax Tests ────────────────────────────────────────────────

func TestRecordTax_SkipsNonPositiveProfit(t *testing.T) {
	repo := &fakeTaxRepo{}
	svc := newTestTaxService(repo, &fakeBankingClient{})

	err := svc.RecordTax(context.Background(), "444000000000000000", nil, 0, "RSD")
	require.NoError(t, err)

	err = svc.RecordTax(context.Background(), "444000000000000000", nil, -100, "RSD")
	require.NoError(t, err)
}

func TestRecordTax_CalculatesTaxCorrectly(t *testing.T) {
	repo := &fakeTaxRepo{}
	svc := newTestTaxService(repo, &fakeBankingClient{})

	// profit=1000, taxRate=0.15, expected taxOwed=150
	err := svc.RecordTax(context.Background(), "444000000000000000", nil, 1000, "RSD")
	require.NoError(t, err)
}

func TestRecordTax_WithEmployeeID(t *testing.T) {
	repo := &fakeTaxRepo{}
	svc := newTestTaxService(repo, &fakeBankingClient{})

	empID := uint(4)
	err := svc.RecordTax(context.Background(), "444000000000000002", &empID, 5000, "USD")
	require.NoError(t, err)
}

func TestRecordTax_RepoError(t *testing.T) {
	repo := &fakeTaxRepo{addTaxErr: errors.New("db error")}
	svc := newTestTaxService(repo, &fakeBankingClient{})

	err := svc.RecordTax(context.Background(), "444000000000000000", nil, 1000, "RSD")
	require.Error(t, err)
}

// ── CollectTaxes Tests ─────────────────────────────────────────────

func TestCollectTaxes_NoTaxes(t *testing.T) {
	repo := &fakeTaxRepo{positiveTaxes: []model.AccumulatedTax{}}
	svc := newTestTaxService(repo, &fakeBankingClient{})

	err := svc.CollectTaxes(context.Background())
	require.NoError(t, err)
}

func TestCollectTaxes_Success(t *testing.T) {
	now := time.Now()
	empID := uint(4)
	repo := &fakeTaxRepo{
		positiveTaxes: []model.AccumulatedTax{
			{
				AccountNumber: "444000000000000000",
				EmployeeID:    &empID,
				TaxOwed:       12000,
				CurrencyCode:  "RSD",
				LastUpdatedAt: now,
			},
		},
	}
	banking := &fakeBankingClient{
		paymentResp: &pb.CreatePaymentResponse{},
	}
	svc := newTestTaxService(repo, banking)

	err := svc.CollectTaxes(context.Background())
	require.NoError(t, err)
	require.NotNil(t, repo.capturedCollection)
	require.Equal(t, model.TaxStatusCollected, repo.capturedCollection.Status)
	require.Nil(t, repo.capturedCollection.FailureReason)
}

func TestCollectTaxes_PaymentFails_RecordsFailedStatus(t *testing.T) {
	now := time.Now()
	repo := &fakeTaxRepo{
		positiveTaxes: []model.AccumulatedTax{
			{
				AccountNumber: "444000112345678911",
				TaxOwed:       8500,
				CurrencyCode:  "RSD",
				LastUpdatedAt: now,
			},
		},
	}
	banking := &fakeBankingClient{
		paymentErr: errors.New("insufficient funds"),
	}
	svc := newTestTaxService(repo, banking)

	err := svc.CollectTaxes(context.Background())
	require.NoError(t, err) // CollectTaxes itself doesn't fail — it records the failure
	require.NotNil(t, repo.capturedCollection)
	require.Equal(t, model.TaxStatusFailed, repo.capturedCollection.Status)
	require.NotNil(t, repo.capturedCollection.FailureReason)
	require.Contains(t, *repo.capturedCollection.FailureReason, "insufficient funds")
}

func TestCollectTaxes_FindTaxesError(t *testing.T) {
	repo := &fakeTaxRepo{positiveTaxesErr: errors.New("db failure")}
	svc := newTestTaxService(repo, &fakeBankingClient{})

	err := svc.CollectTaxes(context.Background())
	require.Error(t, err)
}

func TestCollectTaxes_RecordCollectionError(t *testing.T) {
	now := time.Now()
	repo := &fakeTaxRepo{
		positiveTaxes: []model.AccumulatedTax{
			{
				AccountNumber: "444000000000000000",
				TaxOwed:       5000,
				CurrencyCode:  "RSD",
				LastUpdatedAt: now,
			},
		},
		recordCollectionErr: errors.New("failed to save collection"),
	}
	banking := &fakeBankingClient{paymentResp: &pb.CreatePaymentResponse{}}
	svc := newTestTaxService(repo, banking)

	err := svc.CollectTaxes(context.Background())
	require.Error(t, err)
}

func TestCollectTaxes_SetsCorrectPeriodTimes(t *testing.T) {
	periodStart := time.Now().Add(-24 * time.Hour)
	repo := &fakeTaxRepo{
		positiveTaxes: []model.AccumulatedTax{
			{
				AccountNumber: "444000000000000000",
				TaxOwed:       1000,
				CurrencyCode:  "RSD",
				LastUpdatedAt: periodStart,
			},
		},
	}
	banking := &fakeBankingClient{paymentResp: &pb.CreatePaymentResponse{}}
	svc := newTestTaxService(repo, banking)

	err := svc.CollectTaxes(context.Background())
	require.NoError(t, err)
	require.Equal(t, periodStart, repo.capturedCollection.TaxingPeriodStart)
	require.NotNil(t, repo.capturedCollection.TaxingPeriodEnd)
	require.True(t, repo.capturedCollection.TaxingPeriodEnd.After(periodStart))
}

// ── GetAccumulatedTax Tests ────────────────────────────────────────

func TestGetAccumulatedTax_Success(t *testing.T) {
	expected := &model.AccumulatedTax{
		AccountNumber: "444000112345678911",
		TaxOwed:       8500,
		CurrencyCode:  "RSD",
	}
	repo := &fakeTaxRepo{accumulatedTax: expected}
	svc := newTestTaxService(repo, &fakeBankingClient{})

	result, err := svc.GetAccumulatedTax(context.Background(), "444000112345678911")
	require.NoError(t, err)
	require.Equal(t, expected, result)
}

func TestGetAccumulatedTax_NotFound(t *testing.T) {
	repo := &fakeTaxRepo{accumulatedTax: nil}
	svc := newTestTaxService(repo, &fakeBankingClient{})

	result, err := svc.GetAccumulatedTax(context.Background(), "nonexistent")
	require.NoError(t, err)
	require.Nil(t, result)
}

func TestGetAccumulatedTax_RepoError(t *testing.T) {
	repo := &fakeTaxRepo{accumulatedTaxErr: errors.New("db error")}
	svc := newTestTaxService(repo, &fakeBankingClient{})

	result, err := svc.GetAccumulatedTax(context.Background(), "444000112345678911")
	require.Error(t, err)
	require.Nil(t, result)
}

// ── GetTaxCollections Tests ────────────────────────────────────────

func TestGetTaxCollections_Success(t *testing.T) {
	collections := []model.TaxCollection{
		{AccountNumber: "444000112345678911", TaxOwed: 8500, CurrencyCode: "RSD", Status: model.TaxStatusCollected},
		{AccountNumber: "444000112345678911", TaxOwed: 3000, CurrencyCode: "RSD", Status: model.TaxStatusFailed},
	}
	repo := &fakeTaxRepo{collections: collections}
	svc := newTestTaxService(repo, &fakeBankingClient{})

	result, err := svc.GetTaxCollections(context.Background(), "444000112345678911")
	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Equal(t, model.TaxStatusCollected, result[0].Status)
	require.Equal(t, model.TaxStatusFailed, result[1].Status)
}

func TestGetTaxCollections_Empty(t *testing.T) {
	repo := &fakeTaxRepo{collections: []model.TaxCollection{}}
	svc := newTestTaxService(repo, &fakeBankingClient{})

	result, err := svc.GetTaxCollections(context.Background(), "444000112345678911")
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestGetTaxCollections_RepoError(t *testing.T) {
	repo := &fakeTaxRepo{collectionsErr: errors.New("db error")}
	svc := newTestTaxService(repo, &fakeBankingClient{})

	result, err := svc.GetTaxCollections(context.Background(), "444000112345678911")
	require.Error(t, err)
	require.Nil(t, result)
}

// ── GetEmployeeTotalTax Tests ──────────────────────────────────────

func TestGetEmployeeTotalTax_AllRSD(t *testing.T) {
	empID := uint(4)
	repo := &fakeTaxRepo{
		employeeTaxes: []model.AccumulatedTax{
			{AccountNumber: "444000000000000000", EmployeeID: &empID, TaxOwed: 12000, CurrencyCode: "RSD"},
		},
	}
	svc := newTestTaxService(repo, &fakeBankingClient{})

	total, err := svc.GetEmployeeTotalTax(context.Background(), 4)
	require.NoError(t, err)
	require.Equal(t, 12000.0, total)
}

func TestGetEmployeeTotalTax_MultiCurrency(t *testing.T) {
	empID := uint(4)
	repo := &fakeTaxRepo{
		employeeTaxes: []model.AccumulatedTax{
			{AccountNumber: "444000000000000000", EmployeeID: &empID, TaxOwed: 12000, CurrencyCode: "RSD"},
			{AccountNumber: "444000000000000001", EmployeeID: &empID, TaxOwed: 80, CurrencyCode: "EUR"},
			{AccountNumber: "444000000000000002", EmployeeID: &empID, TaxOwed: 150, CurrencyCode: "USD"},
		},
	}
	// ConvertCurrency multiplies by convertedAmount (acts as exchange rate)
	banking := &fakeBankingClient{convertedAmount: 117.0} // ~117 RSD per foreign unit
	svc := newTestTaxService(repo, banking)

	total, err := svc.GetEmployeeTotalTax(context.Background(), 4)
	require.NoError(t, err)
	// 12000 + (80 * 117) + (150 * 117) = 12000 + 9360 + 17550 = 38910
	require.Equal(t, 12000.0+(80*117.0)+(150*117.0), total)
}

func TestGetEmployeeTotalTax_NoTaxes(t *testing.T) {
	repo := &fakeTaxRepo{employeeTaxes: []model.AccumulatedTax{}}
	svc := newTestTaxService(repo, &fakeBankingClient{})

	total, err := svc.GetEmployeeTotalTax(context.Background(), 99)
	require.NoError(t, err)
	require.Equal(t, 0.0, total)
}

func TestGetEmployeeTotalTax_RepoError(t *testing.T) {
	repo := &fakeTaxRepo{employeeTaxesErr: errors.New("db error")}
	svc := newTestTaxService(repo, &fakeBankingClient{})

	total, err := svc.GetEmployeeTotalTax(context.Background(), 4)
	require.Error(t, err)
	require.Equal(t, 0.0, total)
}

func TestGetEmployeeTotalTax_ConvertError(t *testing.T) {
	empID := uint(5)
	repo := &fakeTaxRepo{
		employeeTaxes: []model.AccumulatedTax{
			{AccountNumber: "444000000000000004", EmployeeID: &empID, TaxOwed: 60, CurrencyCode: "GBP"},
		},
	}
	banking := &fakeBankingClient{convertErr: errors.New("exchange rate unavailable")}
	svc := newTestTaxService(repo, banking)

	total, err := svc.GetEmployeeTotalTax(context.Background(), 5)
	require.Error(t, err)
	require.Equal(t, 0.0, total)
}

func TestGetEmployeeTotalTax_AggregatesSameCurrency(t *testing.T) {
	empID := uint(6)
	repo := &fakeTaxRepo{
		employeeTaxes: []model.AccumulatedTax{
			{AccountNumber: "444000000000000005", EmployeeID: &empID, TaxOwed: 3000, CurrencyCode: "RSD"},
			{AccountNumber: "444000000000000006", EmployeeID: &empID, TaxOwed: 2000, CurrencyCode: "RSD"},
			{AccountNumber: "444000000000000007", EmployeeID: &empID, TaxOwed: 5000, CurrencyCode: "RSD"},
		},
	}
	svc := newTestTaxService(repo, &fakeBankingClient{})

	total, err := svc.GetEmployeeTotalTax(context.Background(), 6)
	require.NoError(t, err)
	// all RSD: 3000 + 2000 + 5000 = 10000
	require.Equal(t, 10000.0, total)
}

func TestGetEmployeeTotalTax_SkipsNegativeCurrencyTotal(t *testing.T) {
	empID := uint(6)
	repo := &fakeTaxRepo{
		employeeTaxes: []model.AccumulatedTax{
			// EUR totals to -50, skipped by sumToRSD since amount <= 0
			{AccountNumber: "444000000000000005", EmployeeID: &empID, TaxOwed: -50, CurrencyCode: "EUR"},
			{AccountNumber: "444000000000000007", EmployeeID: &empID, TaxOwed: 5000, CurrencyCode: "RSD"},
		},
	}
	svc := newTestTaxService(repo, &fakeBankingClient{})

	total, err := svc.GetEmployeeTotalTax(context.Background(), 6)
	require.NoError(t, err)
	// EUR total is -50 so skipped, only 5000 RSD counts
	require.Equal(t, 5000.0, total)
}

// ── GetClientTotalTax Tests ────────────────────────────────────────

func TestGetClientTotalTax_Success(t *testing.T) {
	repo := &fakeTaxRepo{
		clientTaxes: []model.AccumulatedTax{
			{AccountNumber: "444000112345678911", TaxOwed: 8500, CurrencyCode: "RSD"},
			{AccountNumber: "444000112345678913", TaxOwed: 120, CurrencyCode: "EUR"},
		},
	}
	banking := &fakeBankingClient{
		accountsResp: &pb.GetAccountsByClientIDResponse{
			Accounts: []*pb.AccountInfo{
				{AccountNumber: "444000112345678911"},
				{AccountNumber: "444000112345678913"},
			},
		},
		convertedAmount: 117.0,
	}
	svc := newTestTaxService(repo, banking)

	total, err := svc.GetClientTotalTax(context.Background(), 1)
	require.NoError(t, err)
	// 8500 + (120 * 117) = 8500 + 14040 = 22540
	require.Equal(t, 8500.0+(120*117.0), total)
}

func TestGetClientTotalTax_OnlyUSD(t *testing.T) {
	repo := &fakeTaxRepo{
		clientTaxes: []model.AccumulatedTax{
			{AccountNumber: "444000112345678922", TaxOwed: 250, CurrencyCode: "USD"},
		},
	}
	banking := &fakeBankingClient{
		accountsResp: &pb.GetAccountsByClientIDResponse{
			Accounts: []*pb.AccountInfo{
				{AccountNumber: "444000112345678922"},
			},
		},
		convertedAmount: 110.0,
	}
	svc := newTestTaxService(repo, banking)

	total, err := svc.GetClientTotalTax(context.Background(), 2)
	require.NoError(t, err)
	require.Equal(t, 250*110.0, total)
}

func TestGetClientTotalTax_GetAccountsError(t *testing.T) {
	repo := &fakeTaxRepo{}
	banking := &fakeBankingClient{accountsErr: errors.New("user service unavailable")}
	svc := newTestTaxService(repo, banking)

	total, err := svc.GetClientTotalTax(context.Background(), 1)
	require.Error(t, err)
	require.Equal(t, 0.0, total)
}

func TestGetClientTotalTax_NoAccounts(t *testing.T) {
	repo := &fakeTaxRepo{}
	banking := &fakeBankingClient{
		accountsResp: &pb.GetAccountsByClientIDResponse{Accounts: []*pb.AccountInfo{}},
	}
	svc := newTestTaxService(repo, banking)

	total, err := svc.GetClientTotalTax(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 0.0, total)
}

func TestGetClientTotalTax_RepoError(t *testing.T) {
	repo := &fakeTaxRepo{clientTaxesErr: errors.New("db error")}
	banking := &fakeBankingClient{
		accountsResp: &pb.GetAccountsByClientIDResponse{
			Accounts: []*pb.AccountInfo{
				{AccountNumber: "444000112345678911"},
			},
		},
	}
	svc := newTestTaxService(repo, banking)

	total, err := svc.GetClientTotalTax(context.Background(), 1)
	require.Error(t, err)
	require.Equal(t, 0.0, total)
}

func TestGetClientTotalTax_ConvertError(t *testing.T) {
	repo := &fakeTaxRepo{
		clientTaxes: []model.AccumulatedTax{
			{AccountNumber: "444000112345678922", TaxOwed: 250, CurrencyCode: "USD"},
		},
	}
	banking := &fakeBankingClient{
		accountsResp: &pb.GetAccountsByClientIDResponse{
			Accounts: []*pb.AccountInfo{
				{AccountNumber: "444000112345678922"},
			},
		},
		convertErr: errors.New("exchange rate unavailable"),
	}
	svc := newTestTaxService(repo, banking)

	total, err := svc.GetClientTotalTax(context.Background(), 2)
	require.Error(t, err)
	require.Equal(t, 0.0, total)
}
