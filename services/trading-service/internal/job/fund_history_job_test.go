package job

import (
	"context"
	"errors"
	"testing"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/service"
	"github.com/stretchr/testify/require"
)

// ── Fakes ─────────────────────────────────────────────────────────────

type fakeFundRepoForJob struct {
	funds             []model.InvestmentFund
	savedPerformances []*model.FundPerformance
	err               error
}

func (f *fakeFundRepoForJob) GetAllInvestmentFunds(ctx context.Context) ([]model.InvestmentFund, error) {
	return f.funds, f.err
}
func (f *fakeFundRepoForJob) FindByName(ctx context.Context, name string) (*model.InvestmentFund, error) {
	return nil, nil
}
func (f *fakeFundRepoForJob) FindByID(ctx context.Context, id uint) (*model.InvestmentFund, error) {
	return nil, nil
}
func (f *fakeFundRepoForJob) FindByAccountNumber(ctx context.Context, accountNumber string) (*model.InvestmentFund, error) {
	return nil, nil
}
func (f *fakeFundRepoForJob) FindAll(ctx context.Context, name, sortBy, sortDir string, page, pageSize int) ([]model.InvestmentFund, int64, error) {
	return nil, 0, nil
}
func (f *fakeFundRepoForJob) FindByManagerID(ctx context.Context, managerID uint) ([]model.InvestmentFund, error) {
	return nil, nil
}
func (f *fakeFundRepoForJob) Create(ctx context.Context, fund *model.InvestmentFund) error {
	return nil
}
func (f *fakeFundRepoForJob) FindHoldings(ctx context.Context, fundID uint) ([]model.AssetOwnership, error) {
	return nil, nil
}
func (f *fakeFundRepoForJob) GetPerformanceHistory(ctx context.Context, fundID uint, limit int) ([]model.FundPerformance, error) {
	return nil, nil
}
func (f *fakeFundRepoForJob) SavePerformanceSnapshot(ctx context.Context, perf *model.FundPerformance) error {
	f.savedPerformances = append(f.savedPerformances, perf)
	return nil
}
func (f *fakeFundRepoForJob) UpdateManagerID(ctx context.Context, fromManagerID uint, toManagerID uint) (int64, error) {
	return 0, nil
}

func (f *fakeFundRepoForJob) GetAllPerformanceHistories(ctx context.Context, minSnapshots int) (map[uint][]model.FundPerformance, error) {
	return map[uint][]model.FundPerformance{}, nil
}

type dummyBankingClient struct {
	client.BankingClient
}

func (d *dummyBankingClient) GetAccountByNumber(ctx context.Context, accountNumber string) (*pb.GetAccountByNumberResponse, error) {
	return &pb.GetAccountByNumberResponse{AvailableBalance: 100}, nil
}

type dummyOwnershipRepo struct{}

func (d *dummyOwnershipRepo) FindByUserId(ctx context.Context, userId uint, ownerType model.OwnerType) ([]model.AssetOwnership, error) {
	return nil, nil
}
func (d *dummyOwnershipRepo) FindByOwnerType(ctx context.Context, ownerType model.OwnerType) ([]model.AssetOwnership, error) {
	return nil, nil
}
func (d *dummyOwnershipRepo) FindByID(ctx context.Context, id uint) (*model.AssetOwnership, error) {
	return nil, nil
}
func (d *dummyOwnershipRepo) FindByUserAndAsset(ctx context.Context, userId uint, ownerType model.OwnerType, assetID uint) (*model.AssetOwnership, error) {
	return nil, nil
}
func (d *dummyOwnershipRepo) FindByUserAndAssetForUpdate(ctx context.Context, userId uint, ownerType model.OwnerType, assetID uint) (*model.AssetOwnership, error) {
	return nil, nil
}
func (d *dummyOwnershipRepo) Upsert(ctx context.Context, ownership *model.AssetOwnership) error {
	return nil
}
func (d *dummyOwnershipRepo) FindAllPublic(ctx context.Context, page, pageSize int) ([]model.AssetOwnership, int64, error) {
	return nil, 0, nil
}
func (d *dummyOwnershipRepo) UpdateOTCFields(ctx context.Context, ownershipID uint, publicAmount, reservedAmount float64) error {
	return nil
}
func (d *dummyOwnershipRepo) IncreaseReservedAmount(ctx context.Context, identityID uint, ownerType model.OwnerType, assetID uint, delta float64) error {
	return nil
}

// ── Tests ─────────────────────────────────────────────────────────────

func TestFundHistoryJob_Run_Success(t *testing.T) {
	ctx := context.Background()

	fundRepo := &fakeFundRepoForJob{
		funds: []model.InvestmentFund{
			{FundID: 1, AccountNumber: "ACC1"},
		},
	}

	svc := service.NewInvestmentFundService(
		fundRepo,
		nil, nil, nil, nil,
		&dummyOwnershipRepo{},
		nil, nil, nil, nil, nil,
		&dummyBankingClient{},
		nil, nil,
	)

	job := NewFundHistoryJob(svc)

	err := job.Run(ctx)
	require.NoError(t, err)
	require.Len(t, fundRepo.savedPerformances, 1, "expected SavePerformanceSnapshot to be called")
}

func TestFundHistoryJob_Run_ServiceError(t *testing.T) {
	ctx := context.Background()

	fundRepo := &fakeFundRepoForJob{
		err: errors.New("db down"),
	}

	svc := service.NewInvestmentFundService(
		fundRepo,
		nil, nil, nil, nil,
		&dummyOwnershipRepo{},
		nil, nil, nil, nil, nil,
		&dummyBankingClient{},
		nil, nil,
	)

	job := NewFundHistoryJob(svc)

	err := job.Run(ctx)
	require.Error(t, err)
	require.Equal(t, "db down", err.Error())
	require.Len(t, fundRepo.savedPerformances, 0, "SavePerformanceSnapshot should not be called if fetching funds fails")
}
