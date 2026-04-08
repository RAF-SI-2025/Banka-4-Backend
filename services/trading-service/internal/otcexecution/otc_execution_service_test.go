package otcexecution_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	tradingclient "github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/service"
)

type fakeTxManager struct{}

func (m *fakeTxManager) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type fakeContractRepo struct {
	contracts map[uint]*model.OtcContract
}

func (r *fakeContractRepo) Create(_ context.Context, contract *model.OtcContract) error {
	r.contracts[contract.OtcContractID] = contract
	return nil
}
func (r *fakeContractRepo) FindByID(_ context.Context, contractID uint) (*model.OtcContract, error) {
	if contract, ok := r.contracts[contractID]; ok {
		return contract, nil
	}
	return nil, nil
}
func (r *fakeContractRepo) FindByIDForUpdate(ctx context.Context, contractID uint) (*model.OtcContract, error) {
	return r.FindByID(ctx, contractID)
}
func (r *fakeContractRepo) Save(_ context.Context, contract *model.OtcContract) error {
	r.contracts[contract.OtcContractID] = contract
	return nil
}

type fakeShareReservationRepo struct {
	reservations map[uint]*model.OtcShareReservation
}

func (r *fakeShareReservationRepo) Create(_ context.Context, reservation *model.OtcShareReservation) error {
	r.reservations[reservation.ContractID] = reservation
	return nil
}
func (r *fakeShareReservationRepo) FindByContractID(_ context.Context, contractID uint) (*model.OtcShareReservation, error) {
	return r.reservations[contractID], nil
}
func (r *fakeShareReservationRepo) FindByContractIDForUpdate(ctx context.Context, contractID uint) (*model.OtcShareReservation, error) {
	return r.FindByContractID(ctx, contractID)
}
func (r *fakeShareReservationRepo) SumActiveReservedBySellerAsset(_ context.Context, sellerIdentityID uint, sellerOwnerType model.OwnerType, assetID uint, excludeContractID *uint) (float64, error) {
	total := 0.0
	for _, reservation := range r.reservations {
		if reservation.SellerIdentityID != sellerIdentityID || reservation.SellerOwnerType != sellerOwnerType || reservation.AssetID != assetID || reservation.Status != model.OtcShareReservationStatusActive {
			continue
		}
		if excludeContractID != nil && reservation.ContractID == *excludeContractID {
			continue
		}
		total += reservation.ReservedAmount
	}
	return total, nil
}
func (r *fakeShareReservationRepo) Save(_ context.Context, reservation *model.OtcShareReservation) error {
	r.reservations[reservation.ContractID] = reservation
	return nil
}

type fakeExecutionRepo struct {
	byID       map[uint]*model.OtcExecutionSaga
	byContract map[uint]*model.OtcExecutionSaga
	nextID     uint
}

func newFakeExecutionRepo() *fakeExecutionRepo {
	return &fakeExecutionRepo{
		byID:       map[uint]*model.OtcExecutionSaga{},
		byContract: map[uint]*model.OtcExecutionSaga{},
		nextID:     1,
	}
}

func (r *fakeExecutionRepo) Create(_ context.Context, execution *model.OtcExecutionSaga) error {
	execution.OtcExecutionSagaID = r.nextID
	r.nextID++
	r.byID[execution.OtcExecutionSagaID] = execution
	r.byContract[execution.ContractID] = execution
	return nil
}
func (r *fakeExecutionRepo) FindByID(_ context.Context, executionID uint) (*model.OtcExecutionSaga, error) {
	return r.byID[executionID], nil
}
func (r *fakeExecutionRepo) FindByContractID(_ context.Context, contractID uint) (*model.OtcExecutionSaga, error) {
	return r.byContract[contractID], nil
}
func (r *fakeExecutionRepo) FindByContractIDForUpdate(ctx context.Context, contractID uint) (*model.OtcExecutionSaga, error) {
	return r.FindByContractID(ctx, contractID)
}
func (r *fakeExecutionRepo) FindPendingForExecution(_ context.Context, before time.Time, limit int) ([]model.OtcExecutionSaga, error) {
	var out []model.OtcExecutionSaga
	for _, execution := range r.byID {
		if execution.Status == model.OtcExecutionStatusCompleted || execution.Status == model.OtcExecutionStatusFailed {
			continue
		}
		if execution.NextRetryAt == nil || !execution.NextRetryAt.After(before) {
			out = append(out, *execution)
		}
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}
func (r *fakeExecutionRepo) Save(_ context.Context, execution *model.OtcExecutionSaga) error {
	r.byID[execution.OtcExecutionSagaID] = execution
	r.byContract[execution.ContractID] = execution
	return nil
}

type fakeOwnershipRepo struct {
	ownerships map[string]*model.AssetOwnership
	upsertErr  error
}

func ownerKey(identityID uint, ownerType model.OwnerType, assetID uint) string {
	return fmt.Sprintf("%s:%d:%d", ownerType, identityID, assetID)
}

func (r *fakeOwnershipRepo) FindByIdentity(_ context.Context, identityID uint, ownerType model.OwnerType) ([]model.AssetOwnership, error) {
	var out []model.AssetOwnership
	for _, ownership := range r.ownerships {
		if ownership.IdentityID == identityID && ownership.OwnerType == ownerType {
			out = append(out, *ownership)
		}
	}
	return out, nil
}
func (r *fakeOwnershipRepo) FindByOwnerAndAsset(_ context.Context, identityID uint, ownerType model.OwnerType, assetID uint) (*model.AssetOwnership, error) {
	return r.ownerships[ownerKey(identityID, ownerType, assetID)], nil
}
func (r *fakeOwnershipRepo) FindByOwnerAndAssetForUpdate(ctx context.Context, identityID uint, ownerType model.OwnerType, assetID uint) (*model.AssetOwnership, error) {
	return r.FindByOwnerAndAsset(ctx, identityID, ownerType, assetID)
}
func (r *fakeOwnershipRepo) Upsert(_ context.Context, ownership *model.AssetOwnership) error {
	if r.upsertErr != nil {
		return r.upsertErr
	}
	r.ownerships[ownerKey(ownership.IdentityID, ownership.OwnerType, ownership.AssetID)] = ownership
	return nil
}

type fakeBankingClient struct {
	reserveErr error
	releaseErr error
	commitErr  error
	refundErr  error

	reserveCalls int
	releaseCalls int
	commitCalls  int
	refundCalls  int
}

var _ tradingclient.BankingClient = (*fakeBankingClient)(nil)

func (c *fakeBankingClient) GetAccountByNumber(_ context.Context, _ string) (*pb.GetAccountByNumberResponse, error) {
	return nil, nil
}
func (c *fakeBankingClient) CreatePaymentWithoutVerification(_ context.Context, _ *pb.CreatePaymentRequest) (*pb.CreatePaymentResponse, error) {
	return nil, nil
}
func (c *fakeBankingClient) GetAccountsByClientID(_ context.Context, _ uint64) (*pb.GetAccountsByClientIDResponse, error) {
	return nil, nil
}
func (c *fakeBankingClient) ConvertCurrency(_ context.Context, amount float64, _, _ string) (float64, error) {
	return amount, nil
}
func (c *fakeBankingClient) ExecuteTradeSettlement(_ context.Context, _, _ string, _ pb.TradeSettlementDirection, _ float64) (*pb.ExecuteTradeSettlementResponse, error) {
	return nil, nil
}
func (c *fakeBankingClient) ReserveOtcFunds(_ context.Context, req *pb.ReserveOtcFundsRequest) (*pb.OtcFundsReservationResponse, error) {
	c.reserveCalls++
	if c.reserveErr != nil {
		return nil, c.reserveErr
	}
	return &pb.OtcFundsReservationResponse{ExecutionId: req.GetExecutionId(), Status: pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_RESERVED}, nil
}
func (c *fakeBankingClient) ReleaseOtcFunds(_ context.Context, executionID string) (*pb.OtcFundsReservationResponse, error) {
	c.releaseCalls++
	if c.releaseErr != nil {
		return nil, c.releaseErr
	}
	return &pb.OtcFundsReservationResponse{ExecutionId: executionID, Status: pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_RELEASED}, nil
}
func (c *fakeBankingClient) CommitOtcFunds(_ context.Context, executionID string) (*pb.OtcFundsReservationResponse, error) {
	c.commitCalls++
	if c.commitErr != nil {
		return nil, c.commitErr
	}
	return &pb.OtcFundsReservationResponse{ExecutionId: executionID, Status: pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_COMMITTED}, nil
}
func (c *fakeBankingClient) RefundOtcFunds(_ context.Context, executionID string) (*pb.OtcFundsReservationResponse, error) {
	c.refundCalls++
	if c.refundErr != nil {
		return nil, c.refundErr
	}
	return &pb.OtcFundsReservationResponse{ExecutionId: executionID, Status: pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_REFUNDED}, nil
}

func newServiceForTest(now time.Time, banking *fakeBankingClient, otherReservations ...*model.OtcShareReservation) (*service.OtcExecutionService, *fakeContractRepo, *fakeShareReservationRepo, *fakeExecutionRepo, *fakeOwnershipRepo) {
	contractRepo := &fakeContractRepo{
		contracts: map[uint]*model.OtcContract{
			1: {
				OtcContractID:       1,
				BuyerIdentityID:     10,
				BuyerOwnerType:      model.OwnerTypeClient,
				SellerIdentityID:    20,
				SellerOwnerType:     model.OwnerTypeClient,
				SellerAccountNumber: "SELLER-ACC",
				AssetID:             77,
				Quantity:            5,
				StrikePrice:         100,
				Premium:             10,
				TradeCurrencyCode:   "RSD",
				SettlementDate:      now.Add(24 * time.Hour),
				Status:              model.OtcContractStatusActive,
			},
		},
	}
	shareRepo := &fakeShareReservationRepo{
		reservations: map[uint]*model.OtcShareReservation{
			1: {
				ContractID:       1,
				SellerIdentityID: 20,
				SellerOwnerType:  model.OwnerTypeClient,
				AssetID:          77,
				ReservedAmount:   5,
				Status:           model.OtcShareReservationStatusActive,
			},
		},
	}
	for _, reservation := range otherReservations {
		shareRepo.reservations[reservation.ContractID] = reservation
	}
	executionRepo := newFakeExecutionRepo()
	ownershipRepo := &fakeOwnershipRepo{
		ownerships: map[string]*model.AssetOwnership{
			ownerKey(20, model.OwnerTypeClient, 77): {
				IdentityID:  20,
				OwnerType:   model.OwnerTypeClient,
				AssetID:     77,
				Amount:      10,
				AvgBuyPrice: 80,
			},
		},
	}

	svc := service.NewOtcExecutionService(contractRepo, shareRepo, executionRepo, ownershipRepo, &fakeTxManager{}, banking)
	return svc, contractRepo, shareRepo, executionRepo, ownershipRepo
}

func TestExerciseContractHappyPath(t *testing.T) {
	now := time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC)
	banking := &fakeBankingClient{}
	svc, contractRepo, shareRepo, _, ownershipRepo := newServiceForTest(now, banking)

	execution, err := svc.ExerciseContract(context.Background(), 1, "BUYER-ACC")
	require.NoError(t, err)
	require.Equal(t, model.OtcExecutionStatusCompleted, execution.Status)
	require.Equal(t, model.OtcExecutionStepCompleted, execution.CurrentStep)
	require.Equal(t, model.OtcContractStatusExercised, contractRepo.contracts[1].Status)
	require.Equal(t, model.OtcShareReservationStatusConsumed, shareRepo.reservations[1].Status)
	require.Equal(t, 5.0, ownershipRepo.ownerships[ownerKey(10, model.OwnerTypeClient, 77)].Amount)
	require.Equal(t, 5.0, ownershipRepo.ownerships[ownerKey(20, model.OwnerTypeClient, 77)].Amount)
	require.Equal(t, 1, banking.reserveCalls)
	require.Equal(t, 1, banking.commitCalls)
	require.Zero(t, banking.releaseCalls)
	require.Zero(t, banking.refundCalls)
}

func TestExerciseContractMarksExpiredContract(t *testing.T) {
	now := time.Now().UTC()
	svc, contractRepo, _, _, _ := newServiceForTest(now, &fakeBankingClient{})
	contractRepo.contracts[1].SettlementDate = now.Add(-time.Hour)

	_, err := svc.ExerciseContract(context.Background(), 1, "BUYER-ACC")
	require.Error(t, err)
	require.Contains(t, err.Error(), "expired")
	require.Equal(t, model.OtcContractStatusExpired, contractRepo.contracts[1].Status)
}

func TestExerciseContractFailsWithoutActiveReservation(t *testing.T) {
	now := time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC)
	svc, _, shareRepo, _, _ := newServiceForTest(now, &fakeBankingClient{})
	delete(shareRepo.reservations, 1)

	_, err := svc.ExerciseContract(context.Background(), 1, "BUYER-ACC")
	require.Error(t, err)
	require.Contains(t, err.Error(), "active OTC share reservation")
}

func TestExerciseContractReleasesFundsWhenSellerCapacityIsGone(t *testing.T) {
	now := time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC)
	other := &model.OtcShareReservation{
		ContractID:       2,
		SellerIdentityID: 20,
		SellerOwnerType:  model.OwnerTypeClient,
		AssetID:          77,
		ReservedAmount:   8,
		Status:           model.OtcShareReservationStatusActive,
	}
	banking := &fakeBankingClient{}
	svc, _, _, executionRepo, _ := newServiceForTest(now, banking, other)

	execution, err := svc.ExerciseContract(context.Background(), 1, "BUYER-ACC")
	require.NoError(t, err)
	require.Equal(t, model.OtcExecutionStatusFailed, execution.Status)
	require.Equal(t, 1, banking.releaseCalls)
	require.NotNil(t, executionRepo.byContract[1])
}

func TestExerciseContractRefundsWhenOwnershipTransferFails(t *testing.T) {
	now := time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC)
	banking := &fakeBankingClient{}
	svc, _, _, _, ownershipRepo := newServiceForTest(now, banking)
	ownershipRepo.upsertErr = errors.New("write failed")

	execution, err := svc.ExerciseContract(context.Background(), 1, "BUYER-ACC")
	require.NoError(t, err)
	require.Equal(t, model.OtcExecutionStatusFailed, execution.Status)
	require.Equal(t, 1, banking.refundCalls)
}

func TestProcessPendingExecutionsResumesAfterRetryableReserveFailure(t *testing.T) {
	now := time.Now().UTC()
	banking := &fakeBankingClient{
		reserveErr: status.Error(codes.Unavailable, "temporary outage"),
	}
	svc, _, _, executionRepo, _ := newServiceForTest(now, banking)

	execution, err := svc.ExerciseContract(context.Background(), 1, "BUYER-ACC")
	require.NoError(t, err)
	require.Equal(t, model.OtcExecutionStatusInProgress, execution.Status)
	require.NotNil(t, execution.NextRetryAt)

	banking.reserveErr = nil
	executionRepo.byID[execution.OtcExecutionSagaID].NextRetryAt = nil

	err = svc.ProcessPendingExecutions(context.Background())
	require.NoError(t, err)

	execution, err = svc.GetExecutionStatus(context.Background(), execution.OtcExecutionSagaID)
	require.NoError(t, err)
	require.Equal(t, model.OtcExecutionStatusCompleted, execution.Status)
}
