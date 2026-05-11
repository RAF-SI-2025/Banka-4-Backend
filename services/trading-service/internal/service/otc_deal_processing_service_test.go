package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type processingOfferRepo struct {
	offers map[uint]*model.OtcOffer
	nextID uint
}

func newProcessingOfferRepo() *processingOfferRepo {
	return &processingOfferRepo{offers: map[uint]*model.OtcOffer{}, nextID: 1}
}

func (r *processingOfferRepo) Create(_ context.Context, offer *model.OtcOffer) error {
	if offer.OtcOfferID == 0 {
		offer.OtcOfferID = r.nextID
		r.nextID++
	}

	r.offers[offer.OtcOfferID] = new(*offer)
	return nil
}

func (r *processingOfferRepo) Save(_ context.Context, offer *model.OtcOffer) error {
	r.offers[offer.OtcOfferID] = new(*offer)
	return nil
}

func (r *processingOfferRepo) FindByID(_ context.Context, id uint) (*model.OtcOffer, error) {
	offer, ok := r.offers[id]
	if !ok {
		return nil, nil
	}
	return new(*offer), nil
}

func (r *processingOfferRepo) FindByIDForUpdate(ctx context.Context, id uint) (*model.OtcOffer, error) {
	return r.FindByID(ctx, id)
}

func (r *processingOfferRepo) FindActiveForUser(_ context.Context, userID uint) ([]model.OtcOffer, error) {
	out := make([]model.OtcOffer, 0)
	for _, offer := range r.offers {
		if offer.Status == model.OtcOfferStatusActive && (offer.BuyerID == userID || offer.SellerID == userID) {
			out = append(out, *offer)
		}
	}
	return out, nil
}

func (r *processingOfferRepo) FindActiveBySellerAndStock(_ context.Context, sellerID, stockID uint, excludeID *uint) ([]model.OtcOffer, error) {
	out := make([]model.OtcOffer, 0)
	for _, offer := range r.offers {
		if offer.Status != model.OtcOfferStatusActive || offer.SellerID != sellerID || offer.StockAssetID != stockID {
			continue
		}
		if excludeID != nil && offer.OtcOfferID == *excludeID {
			continue
		}
		out = append(out, *offer)
	}
	return out, nil
}

type processingContractRepo struct {
	contracts map[uint]*model.OtcOptionContract
	nextID    uint
	createErr error
	saveErr   error
}

func newProcessingContractRepo() *processingContractRepo {
	return &processingContractRepo{contracts: map[uint]*model.OtcOptionContract{}, nextID: 1}
}

func (r *processingContractRepo) Create(_ context.Context, contract *model.OtcOptionContract) error {
	if r.createErr != nil {
		return r.createErr
	}

	if contract.OtcOptionContractID == 0 {
		contract.OtcOptionContractID = r.nextID
		r.nextID++
	}

	r.contracts[contract.OtcOptionContractID] = new(*contract)
	return nil
}

func (r *processingContractRepo) Save(_ context.Context, contract *model.OtcOptionContract) error {
	if r.saveErr != nil {
		return r.saveErr
	}

	r.contracts[contract.OtcOptionContractID] = new(*contract)
	return nil
}

func (r *processingContractRepo) FindByID(_ context.Context, id uint) (*model.OtcOptionContract, error) {
	contract, ok := r.contracts[id]
	if !ok {
		return nil, nil
	}
	return new(*contract), nil
}

func (r *processingContractRepo) FindByIDForUpdate(ctx context.Context, id uint) (*model.OtcOptionContract, error) {
	return r.FindByID(ctx, id)
}

func (r *processingContractRepo) FindByOfferID(_ context.Context, offerID uint) (*model.OtcOptionContract, error) {
	for _, contract := range r.contracts {
		if contract.OtcOfferID == offerID {
			return new(*contract), nil
		}
	}
	return nil, nil
}

func (r *processingContractRepo) FindForUser(_ context.Context, userID uint) ([]model.OtcOptionContract, error) {
	out := make([]model.OtcOptionContract, 0)
	for _, contract := range r.contracts {
		if contract.BuyerID == userID || contract.SellerID == userID {
			out = append(out, *contract)
		}
	}
	return out, nil
}

func (r *processingContractRepo) FindActiveBySellerAndStock(_ context.Context, sellerID, stockID uint, now time.Time) ([]model.OtcOptionContract, error) {
	out := make([]model.OtcOptionContract, 0)
	for _, contract := range r.contracts {
		if contract.SellerID == sellerID && contract.StockAssetID == stockID && contract.Status == model.OtcOptionContractStatusActive && contract.SettlementDate.After(now) {
			out = append(out, *contract)
		}
	}
	return out, nil
}

func (r *processingContractRepo) FindExpiredActive(_ context.Context, before time.Time, limit int) ([]model.OtcOptionContract, error) {
	out := make([]model.OtcOptionContract, 0)
	for _, contract := range r.contracts {
		if contract.Status == model.OtcOptionContractStatusActive && !contract.SettlementDate.After(before) {
			out = append(out, *contract)
		}
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

type processingShareReservationRepo struct {
	byContract map[uint]*model.OtcShareReservation
	nextID     uint
}

func newProcessingShareReservationRepo() *processingShareReservationRepo {
	return &processingShareReservationRepo{byContract: map[uint]*model.OtcShareReservation{}, nextID: 1}
}

func (r *processingShareReservationRepo) Create(_ context.Context, reservation *model.OtcShareReservation) error {
	if reservation.OtcShareReservationID == 0 {
		reservation.OtcShareReservationID = r.nextID
		r.nextID++
	}
	r.byContract[reservation.ContractID] = new(*reservation)
	return nil
}

func (r *processingShareReservationRepo) FindByContractID(_ context.Context, contractID uint) (*model.OtcShareReservation, error) {
	reservation, ok := r.byContract[contractID]
	if !ok {
		return nil, nil
	}

	return new(*reservation), nil
}

func (r *processingShareReservationRepo) FindByContractIDForUpdate(ctx context.Context, contractID uint) (*model.OtcShareReservation, error) {
	return r.FindByContractID(ctx, contractID)
}

func (r *processingShareReservationRepo) SumActiveReservedBySellerAsset(_ context.Context, sellerID uint, ownerType model.OwnerType, stockAssetID uint, excludeContractID *uint) (float64, error) {
	total := 0.0
	for contractID, reservation := range r.byContract {
		if reservation.SellerID != sellerID || reservation.OwnerType != ownerType || reservation.StockAssetID != stockAssetID || reservation.Status != model.OtcShareReservationStatusActive {
			continue
		}
		if excludeContractID != nil && contractID == *excludeContractID {
			continue
		}
		total += reservation.ReservedAmount
	}
	return total, nil
}

func (r *processingShareReservationRepo) Save(_ context.Context, reservation *model.OtcShareReservation) error {
	r.byContract[reservation.ContractID] = new(*reservation)
	return nil
}

type processingExecutionRepo struct {
	byID       map[uint]*model.OtcExecutionSaga
	byContract map[uint]uint
	nextID     uint
}

func newProcessingExecutionRepo() *processingExecutionRepo {
	return &processingExecutionRepo{
		byID:       map[uint]*model.OtcExecutionSaga{},
		byContract: map[uint]uint{},
		nextID:     1,
	}
}

func (r *processingExecutionRepo) Create(_ context.Context, saga *model.OtcExecutionSaga) error {
	if saga.OtcExecutionSagaID == 0 {
		saga.OtcExecutionSagaID = r.nextID
		r.nextID++
	}

	r.byID[saga.OtcExecutionSagaID] = new(*saga)
	r.byContract[saga.ContractID] = saga.OtcExecutionSagaID
	return nil
}

func (r *processingExecutionRepo) FindByID(_ context.Context, sagaID uint) (*model.OtcExecutionSaga, error) {
	saga, ok := r.byID[sagaID]
	if !ok {
		return nil, nil
	}
	return new(*saga), nil
}

func (r *processingExecutionRepo) FindByContractID(_ context.Context, contractID uint) (*model.OtcExecutionSaga, error) {
	id, ok := r.byContract[contractID]
	if !ok {
		return nil, nil
	}
	return r.FindByID(context.Background(), id)
}

func (r *processingExecutionRepo) FindByContractIDForUpdate(ctx context.Context, contractID uint) (*model.OtcExecutionSaga, error) {
	return r.FindByContractID(ctx, contractID)
}

func (r *processingExecutionRepo) FindPendingForExecution(_ context.Context, before time.Time, limit int) ([]model.OtcExecutionSaga, error) {
	out := make([]model.OtcExecutionSaga, 0)
	for _, saga := range r.byID {
		if saga.Status != model.OtcExecutionStatusInProgress && saga.Status != model.OtcExecutionStatusCompensating {
			continue
		}
		if saga.NextRetryAt != nil && saga.NextRetryAt.After(before) {
			continue
		}
		out = append(out, *saga)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (r *processingExecutionRepo) Save(_ context.Context, saga *model.OtcExecutionSaga) error {
	r.byID[saga.OtcExecutionSagaID] = new(*saga)
	r.byContract[saga.ContractID] = saga.OtcExecutionSagaID
	return nil
}

type processingOwnershipRepo struct {
	ownerships map[string]*model.AssetOwnership
	upsertErr  error
}

func newProcessingOwnershipRepo() *processingOwnershipRepo {
	return &processingOwnershipRepo{ownerships: map[string]*model.AssetOwnership{}}
}

func processingOwnershipKey(userID uint, ownerType model.OwnerType, assetID uint) string {
	return fmt.Sprintf("%d:%s:%d", userID, ownerType, assetID)
}

func (r *processingOwnershipRepo) seed(ownership model.AssetOwnership) {
	r.ownerships[processingOwnershipKey(ownership.UserId, ownership.OwnerType, ownership.AssetID)] = new(ownership)
}

func (r *processingOwnershipRepo) FindByUserId(_ context.Context, userID uint, ownerType model.OwnerType) ([]model.AssetOwnership, error) {
	out := make([]model.AssetOwnership, 0)
	for _, ownership := range r.ownerships {
		if ownership.UserId == userID && ownership.OwnerType == ownerType {
			out = append(out, *ownership)
		}
	}
	return out, nil
}

func (r *processingOwnershipRepo) FindByOwnerType(_ context.Context, ownerType model.OwnerType) ([]model.AssetOwnership, error) {
	out := make([]model.AssetOwnership, 0)
	for _, ownership := range r.ownerships {
		if ownership.OwnerType == ownerType {
			out = append(out, *ownership)
		}
	}
	return out, nil
}

func (r *processingOwnershipRepo) FindByID(_ context.Context, id uint) (*model.AssetOwnership, error) {
	for _, ownership := range r.ownerships {
		if ownership.AssetOwnershipID == id || ownership.AssetID == id {
			copy := *ownership
			return &copy, nil
		}
	}
	return nil, nil
}

func (r *processingOwnershipRepo) FindByUserAndAsset(_ context.Context, userID uint, ownerType model.OwnerType, assetID uint) (*model.AssetOwnership, error) {
	ownership, ok := r.ownerships[processingOwnershipKey(userID, ownerType, assetID)]
	if !ok {
		return nil, nil
	}
	return new(*ownership), nil
}

func (r *processingOwnershipRepo) FindByUserAndAssetForUpdate(ctx context.Context, userID uint, ownerType model.OwnerType, assetID uint) (*model.AssetOwnership, error) {
	return r.FindByUserAndAsset(ctx, userID, ownerType, assetID)
}

func (r *processingOwnershipRepo) Upsert(_ context.Context, ownership *model.AssetOwnership) error {
	if r.upsertErr != nil {
		return r.upsertErr
	}
	r.ownerships[processingOwnershipKey(ownership.UserId, ownership.OwnerType, ownership.AssetID)] = new(*ownership)
	return nil
}

func (r *processingOwnershipRepo) IncreaseReservedAmount(_ context.Context, userID uint, ownerType model.OwnerType, assetID uint, delta float64) error {
	key := processingOwnershipKey(userID, ownerType, assetID)
	ownership, ok := r.ownerships[key]
	if !ok {
		return nil
	}
	ownership.ReservedAmount += delta
	return nil
}

func (r *processingOwnershipRepo) FindAllPublic(_ context.Context, _, _ int) ([]model.AssetOwnership, int64, error) {
	return nil, 0, nil
}

func (r *processingOwnershipRepo) UpdateOTCFields(_ context.Context, _ uint, _, _ float64) error {
	return nil
}

type processingTxManager struct{}

func (m *processingTxManager) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type processingBankingClient struct {
	payments         []*pb.CreatePaymentRequest
	reserveCalls     []string
	commitCalls      []string
	releaseCalls     []string
	refundCalls      []string
	paymentErr       error
	reserveErr       error
	commitErr        error
	releaseErr       error
	refundErr        error
	accountByNumber  map[string]uint64
	accountsByClient map[uint64]*pb.GetAccountsByClientIDResponse
}

func (c *processingBankingClient) GetAccountByNumber(_ context.Context, accountNumber string) (*pb.GetAccountByNumberResponse, error) {
	clientID := c.accountByNumber[accountNumber]
	return &pb.GetAccountByNumberResponse{AccountNumber: accountNumber, ClientId: clientID, CurrencyCode: "RSD"}, nil
}

func (c *processingBankingClient) HasActiveLoan(_ context.Context, _ uint64) (*pb.HasActiveLoanResponse, error) {
	return &pb.HasActiveLoanResponse{HasActiveLoan: false}, nil
}

func (c *processingBankingClient) CreatePaymentWithoutVerification(_ context.Context, req *pb.CreatePaymentRequest) (*pb.CreatePaymentResponse, error) {
	c.payments = append(c.payments, req)
	if c.paymentErr != nil {
		return nil, c.paymentErr
	}
	return &pb.CreatePaymentResponse{PaymentId: uint64(len(c.payments))}, nil
}

func (c *processingBankingClient) GetAccountsByClientID(_ context.Context, clientID uint64) (*pb.GetAccountsByClientIDResponse, error) {
	if resp, ok := c.accountsByClient[clientID]; ok {
		return resp, nil
	}
	return &pb.GetAccountsByClientIDResponse{}, nil
}

func (c *processingBankingClient) ConvertCurrency(_ context.Context, amount float64, _, _ string) (float64, error) {
	return amount, nil
}

func (c *processingBankingClient) ExecuteTradeSettlement(_ context.Context, _, _ string, _ pb.TradeSettlementDirection, _ float64) (*pb.ExecuteTradeSettlementResponse, error) {
	return &pb.ExecuteTradeSettlementResponse{TransactionId: 1}, nil
}

func (c *processingBankingClient) GetAccountCurrency(_ context.Context, _ string) (string, error) {
	return "RSD", nil
}

func (c *processingBankingClient) ReserveOtcFunds(_ context.Context, req *pb.ReserveOtcFundsRequest) (*pb.OtcFundsReservationResponse, error) {
	c.reserveCalls = append(c.reserveCalls, req.ExecutionId)
	if c.reserveErr != nil {
		return nil, c.reserveErr
	}
	return &pb.OtcFundsReservationResponse{
		ExecutionId:         req.ExecutionId,
		Status:              pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_RESERVED,
		TradeAmount:         req.Amount,
		TradeCurrencyCode:   req.CurrencyCode,
		BuyerAccountNumber:  req.BuyerAccountNumber,
		SellerAccountNumber: req.SellerAccountNumber,
	}, nil
}

func (c *processingBankingClient) ReleaseOtcFunds(_ context.Context, executionID string) (*pb.OtcFundsReservationResponse, error) {
	c.releaseCalls = append(c.releaseCalls, executionID)
	if c.releaseErr != nil {
		return nil, c.releaseErr
	}
	return &pb.OtcFundsReservationResponse{ExecutionId: executionID, Status: pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_RELEASED}, nil
}

func (c *processingBankingClient) CommitOtcFunds(_ context.Context, executionID string) (*pb.OtcFundsReservationResponse, error) {
	c.commitCalls = append(c.commitCalls, executionID)
	if c.commitErr != nil {
		return nil, c.commitErr
	}
	return &pb.OtcFundsReservationResponse{ExecutionId: executionID, Status: pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_COMMITTED}, nil
}

func (c *processingBankingClient) RefundOtcFunds(_ context.Context, executionID string) (*pb.OtcFundsReservationResponse, error) {
	c.refundCalls = append(c.refundCalls, executionID)
	if c.refundErr != nil {
		return nil, c.refundErr
	}
	return &pb.OtcFundsReservationResponse{ExecutionId: executionID, Status: pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_REFUNDED}, nil
}

func (c *processingBankingClient) CreateFundAccount(_ context.Context, _ string, _ uint64) (string, error) {
	return "", nil
}

func newProcessingServiceForTest(now time.Time) (*OtcDealProcessingService, *processingOfferRepo, *processingContractRepo, *processingShareReservationRepo, *processingExecutionRepo, *processingOwnershipRepo, *processingBankingClient) {
	offerRepo := newProcessingOfferRepo()
	contractRepo := newProcessingContractRepo()
	reservationRepo := newProcessingShareReservationRepo()
	executionRepo := newProcessingExecutionRepo()
	ownershipRepo := newProcessingOwnershipRepo()
	bankingClient := &processingBankingClient{accountByNumber: map[string]uint64{}}

	svc := NewOtcDealProcessingService(
		offerRepo,
		contractRepo,
		reservationRepo,
		executionRepo,
		ownershipRepo,
		&processingTxManager{},
		bankingClient,
	)
	svc.now = func() time.Time { return now }
	return svc, offerRepo, contractRepo, reservationRepo, executionRepo, ownershipRepo, bankingClient
}

func TestOtcDealProcessingServiceFinalizeAgreementActivatesContract(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	svc, offerRepo, contractRepo, reservationRepo, _, ownershipRepo, bankingClient := newProcessingServiceForTest(now)

	sellerAccount := "seller-rsd"
	offer := &model.OtcOffer{
		BuyerID:             10,
		SellerID:            20,
		StockAssetID:        1,
		Amount:              10,
		PricePerStockRSD:    50,
		PremiumRSD:          5,
		SettlementDate:      now.Add(48 * time.Hour),
		BuyerAccountNumber:  "buyer-rsd",
		SellerAccountNumber: &sellerAccount,
		Status:              model.OtcOfferStatusActive,
		LastModified:        now,
		ModifiedBy:          10,
	}
	require.NoError(t, offerRepo.Create(context.Background(), offer))

	ownershipRepo.seed(model.AssetOwnership{
		AssetOwnershipID: 1,
		UserId:           20,
		OwnerType:        model.OwnerTypeClient,
		AssetID:          1,
		Amount:           100,
		PublicAmount:     100,
		ReservedAmount:   0,
	})

	contract, err := svc.FinalizeAgreement(context.Background(), offer.OtcOfferID, 20)
	require.NoError(t, err)
	require.NotNil(t, contract)
	require.Equal(t, model.OtcOptionContractStatusActive, contract.Status)
	require.Equal(t, "buyer-rsd", contract.BuyerAccountNumber)
	require.Equal(t, "seller-rsd", contract.SellerAccountNumber)
	require.Len(t, contractRepo.contracts, 1)
	require.Len(t, bankingClient.payments, 1)

	storedOffer := offerRepo.offers[offer.OtcOfferID]
	require.Equal(t, model.OtcOfferStatusAccepted, storedOffer.Status)
	require.NotNil(t, storedOffer.OptionContractID)

	storedReservation := reservationRepo.byContract[contract.OtcOptionContractID]
	require.Equal(t, model.OtcShareReservationStatusActive, storedReservation.Status)
	require.Equal(t, 10.0, storedReservation.ReservedAmount)

	sellerOwnership := ownershipRepo.ownerships[processingOwnershipKey(20, model.OwnerTypeClient, 1)]
	require.Equal(t, 10.0, sellerOwnership.ReservedAmount)
}

func TestOtcDealProcessingServiceFinalizeAgreementFailsWhenPremiumTransferFails(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	svc, offerRepo, contractRepo, reservationRepo, _, ownershipRepo, bankingClient := newProcessingServiceForTest(now)

	sellerAccount := "seller-rsd"
	offer := &model.OtcOffer{
		BuyerID:             10,
		SellerID:            20,
		StockAssetID:        1,
		Amount:              10,
		PricePerStockRSD:    50,
		PremiumRSD:          5,
		SettlementDate:      now.Add(48 * time.Hour),
		BuyerAccountNumber:  "buyer-rsd",
		SellerAccountNumber: &sellerAccount,
		Status:              model.OtcOfferStatusActive,
		LastModified:        now,
		ModifiedBy:          10,
	}
	require.NoError(t, offerRepo.Create(context.Background(), offer))

	ownershipRepo.seed(model.AssetOwnership{
		AssetOwnershipID: 1,
		UserId:           20,
		OwnerType:        model.OwnerTypeClient,
		AssetID:          1,
		Amount:           100,
		PublicAmount:     100,
		ReservedAmount:   0,
	})

	bankingClient.paymentErr = errors.New("payment down")

	contract, err := svc.FinalizeAgreement(context.Background(), offer.OtcOfferID, 20)
	require.Nil(t, contract)
	require.Error(t, err)
	require.Contains(t, err.Error(), "premium transfer failed")
	require.Len(t, bankingClient.payments, 1)
	require.Empty(t, contractRepo.contracts)
	require.Empty(t, reservationRepo.byContract)

	storedOffer := offerRepo.offers[offer.OtcOfferID]
	require.Equal(t, model.OtcOfferStatusActive, storedOffer.Status)
	require.Nil(t, storedOffer.OptionContractID)

	sellerOwnership := ownershipRepo.ownerships[processingOwnershipKey(20, model.OwnerTypeClient, 1)]
	require.Equal(t, 0.0, sellerOwnership.ReservedAmount)
}

func TestOtcDealProcessingServiceFinalizeAgreementCompensatesPremiumWhenActivationFails(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	svc, offerRepo, contractRepo, reservationRepo, _, ownershipRepo, bankingClient := newProcessingServiceForTest(now)

	sellerAccount := "seller-rsd"
	offer := &model.OtcOffer{
		BuyerID:             10,
		SellerID:            20,
		StockAssetID:        1,
		Amount:              10,
		PricePerStockRSD:    50,
		PremiumRSD:          5,
		SettlementDate:      now.Add(48 * time.Hour),
		BuyerAccountNumber:  "buyer-rsd",
		SellerAccountNumber: &sellerAccount,
		Status:              model.OtcOfferStatusActive,
		LastModified:        now,
		ModifiedBy:          10,
	}
	require.NoError(t, offerRepo.Create(context.Background(), offer))

	ownershipRepo.seed(model.AssetOwnership{
		AssetOwnershipID: 1,
		UserId:           20,
		OwnerType:        model.OwnerTypeClient,
		AssetID:          1,
		Amount:           100,
		PublicAmount:     100,
		ReservedAmount:   0,
	})

	contractRepo.createErr = errors.New("contract create failed")

	contract, err := svc.FinalizeAgreement(context.Background(), offer.OtcOfferID, 20)
	require.Nil(t, contract)
	require.Error(t, err)
	require.Len(t, bankingClient.payments, 2, "activation failure after payment should trigger compensation payment")

	require.Equal(t, "buyer-rsd", bankingClient.payments[0].PayerAccountNumber)
	require.Equal(t, "seller-rsd", bankingClient.payments[0].RecipientAccountNumber)
	require.Equal(t, "seller-rsd", bankingClient.payments[1].PayerAccountNumber)
	require.Equal(t, "buyer-rsd", bankingClient.payments[1].RecipientAccountNumber)

	require.Empty(t, contractRepo.contracts)
	require.Empty(t, reservationRepo.byContract)

	storedOffer := offerRepo.offers[offer.OtcOfferID]
	require.Equal(t, model.OtcOfferStatusActive, storedOffer.Status)
	require.Nil(t, storedOffer.OptionContractID)
}

func TestOtcDealProcessingServiceExerciseContractCompletesSaga(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	svc, _, contractRepo, reservationRepo, executionRepo, ownershipRepo, bankingClient := newProcessingServiceForTest(now)

	contract := &model.OtcOptionContract{
		OtcOfferID:          1,
		BuyerID:             10,
		SellerID:            20,
		StockAssetID:        1,
		Amount:              10,
		StrikePriceRSD:      50,
		PremiumRSD:          5,
		SettlementDate:      now.Add(24 * time.Hour),
		BuyerAccountNumber:  "buyer-rsd",
		SellerAccountNumber: "seller-rsd",
		Status:              model.OtcOptionContractStatusActive,
	}
	require.NoError(t, contractRepo.Create(context.Background(), contract))
	require.NoError(t, reservationRepo.Create(context.Background(), &model.OtcShareReservation{
		ContractID:     contract.OtcOptionContractID,
		SellerID:       20,
		OwnerType:      model.OwnerTypeClient,
		StockAssetID:   1,
		ReservedAmount: 10,
		Status:         model.OtcShareReservationStatusActive,
	}))

	ownershipRepo.seed(model.AssetOwnership{
		AssetOwnershipID: 1,
		UserId:           20,
		OwnerType:        model.OwnerTypeClient,
		AssetID:          1,
		Amount:           100,
		PublicAmount:     100,
		ReservedAmount:   10,
	})

	execution, err := svc.ExerciseContract(context.Background(), contract.OtcOptionContractID)
	require.NoError(t, err)
	require.NotNil(t, execution)
	require.Equal(t, model.OtcExecutionStatusCompleted, execution.Status)
	require.Equal(t, model.OtcExecutionStepCompleted, execution.CurrentStep)
	require.Len(t, bankingClient.reserveCalls, 1)
	require.Len(t, bankingClient.commitCalls, 1)

	storedExecution := executionRepo.byID[execution.OtcExecutionSagaID]
	require.Equal(t, model.OtcExecutionStatusCompleted, storedExecution.Status)

	storedContract := contractRepo.contracts[contract.OtcOptionContractID]
	require.Equal(t, model.OtcOptionContractStatusExercised, storedContract.Status)
	require.NotNil(t, storedContract.ExercisedAt)

	storedReservation := reservationRepo.byContract[contract.OtcOptionContractID]
	require.Equal(t, model.OtcShareReservationStatusConsumed, storedReservation.Status)

	sellerOwnership := ownershipRepo.ownerships[processingOwnershipKey(20, model.OwnerTypeClient, 1)]
	require.Equal(t, 90.0, sellerOwnership.Amount)
	require.Equal(t, 90.0, sellerOwnership.PublicAmount)
	require.Equal(t, 0.0, sellerOwnership.ReservedAmount)

	buyerOwnership := ownershipRepo.ownerships[processingOwnershipKey(10, model.OwnerTypeClient, 1)]
	require.Equal(t, 10.0, buyerOwnership.Amount)
	require.Equal(t, 50.0, buyerOwnership.AvgBuyPriceRSD)
}

func TestOtcDealProcessingServiceExerciseContractRejectsAlreadyExercisedContract(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	svc, _, contractRepo, reservationRepo, _, ownershipRepo, bankingClient := newProcessingServiceForTest(now)

	contract := &model.OtcOptionContract{
		OtcOfferID:          1,
		BuyerID:             10,
		SellerID:            20,
		StockAssetID:        1,
		Amount:              10,
		StrikePriceRSD:      50,
		PremiumRSD:          5,
		SettlementDate:      now.Add(24 * time.Hour),
		BuyerAccountNumber:  "buyer-rsd",
		SellerAccountNumber: "seller-rsd",
		Status:              model.OtcOptionContractStatusActive,
	}
	require.NoError(t, contractRepo.Create(context.Background(), contract))
	require.NoError(t, reservationRepo.Create(context.Background(), &model.OtcShareReservation{
		ContractID:     contract.OtcOptionContractID,
		SellerID:       20,
		OwnerType:      model.OwnerTypeClient,
		StockAssetID:   1,
		ReservedAmount: 10,
		Status:         model.OtcShareReservationStatusActive,
	}))
	ownershipRepo.seed(model.AssetOwnership{
		AssetOwnershipID: 1,
		UserId:           20,
		OwnerType:        model.OwnerTypeClient,
		AssetID:          1,
		Amount:           100,
		PublicAmount:     100,
		ReservedAmount:   10,
	})

	first, err := svc.ExerciseContract(context.Background(), contract.OtcOptionContractID)
	require.NoError(t, err)
	require.Equal(t, model.OtcExecutionStatusCompleted, first.Status)

	second, err := svc.ExerciseContract(context.Background(), contract.OtcOptionContractID)
	require.Nil(t, second)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already been exercised")
	require.Len(t, bankingClient.reserveCalls, 1)
	require.Len(t, bankingClient.commitCalls, 1)
}

func TestOtcDealProcessingServiceProcessExpiredContractsReleasesReservation(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	svc, _, contractRepo, reservationRepo, _, ownershipRepo, _ := newProcessingServiceForTest(now)

	contract := &model.OtcOptionContract{
		OtcOfferID:          1,
		BuyerID:             10,
		SellerID:            20,
		StockAssetID:        1,
		Amount:              10,
		StrikePriceRSD:      50,
		PremiumRSD:          5,
		SettlementDate:      now.Add(-time.Hour),
		BuyerAccountNumber:  "buyer-rsd",
		SellerAccountNumber: "seller-rsd",
		Status:              model.OtcOptionContractStatusActive,
	}
	require.NoError(t, contractRepo.Create(context.Background(), contract))
	require.NoError(t, reservationRepo.Create(context.Background(), &model.OtcShareReservation{
		ContractID:     contract.OtcOptionContractID,
		SellerID:       20,
		OwnerType:      model.OwnerTypeClient,
		StockAssetID:   1,
		ReservedAmount: 10,
		Status:         model.OtcShareReservationStatusActive,
	}))
	ownershipRepo.seed(model.AssetOwnership{
		AssetOwnershipID: 1,
		UserId:           20,
		OwnerType:        model.OwnerTypeClient,
		AssetID:          1,
		Amount:           100,
		PublicAmount:     100,
		ReservedAmount:   10,
	})

	require.NoError(t, svc.ProcessExpiredContracts(context.Background()))

	storedContract := contractRepo.contracts[contract.OtcOptionContractID]
	require.Equal(t, model.OtcOptionContractStatusExpired, storedContract.Status)

	storedReservation := reservationRepo.byContract[contract.OtcOptionContractID]
	require.Equal(t, model.OtcShareReservationStatusReleased, storedReservation.Status)

	sellerOwnership := ownershipRepo.ownerships[processingOwnershipKey(20, model.OwnerTypeClient, 1)]
	require.Equal(t, 0.0, sellerOwnership.ReservedAmount)
}

func TestOtcDealProcessingServiceExerciseContractExpiresPastDueContract(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	svc, _, contractRepo, reservationRepo, _, ownershipRepo, bankingClient := newProcessingServiceForTest(now)

	contract := &model.OtcOptionContract{
		OtcOfferID:          1,
		BuyerID:             10,
		SellerID:            20,
		StockAssetID:        1,
		Amount:              10,
		StrikePriceRSD:      50,
		PremiumRSD:          5,
		SettlementDate:      now.Add(-time.Minute),
		BuyerAccountNumber:  "buyer-rsd",
		SellerAccountNumber: "seller-rsd",
		Status:              model.OtcOptionContractStatusActive,
	}
	require.NoError(t, contractRepo.Create(context.Background(), contract))
	require.NoError(t, reservationRepo.Create(context.Background(), &model.OtcShareReservation{
		ContractID:     contract.OtcOptionContractID,
		SellerID:       20,
		OwnerType:      model.OwnerTypeClient,
		StockAssetID:   1,
		ReservedAmount: 10,
		Status:         model.OtcShareReservationStatusActive,
	}))
	ownershipRepo.seed(model.AssetOwnership{
		AssetOwnershipID: 1,
		UserId:           20,
		OwnerType:        model.OwnerTypeClient,
		AssetID:          1,
		Amount:           100,
		PublicAmount:     100,
		ReservedAmount:   10,
	})

	execution, err := svc.ExerciseContract(context.Background(), contract.OtcOptionContractID)
	require.Nil(t, execution)
	require.Error(t, err)
	require.Contains(t, err.Error(), "expired")
	require.Empty(t, bankingClient.reserveCalls)

	storedContract := contractRepo.contracts[contract.OtcOptionContractID]
	require.Equal(t, model.OtcOptionContractStatusExpired, storedContract.Status)

	storedReservation := reservationRepo.byContract[contract.OtcOptionContractID]
	require.Equal(t, model.OtcShareReservationStatusReleased, storedReservation.Status)

	sellerOwnership := ownershipRepo.ownerships[processingOwnershipKey(20, model.OwnerTypeClient, 1)]
	require.Equal(t, 0.0, sellerOwnership.ReservedAmount)
}

func TestOtcDealProcessingServiceExerciseContractRequiresActiveReservation(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	svc, _, contractRepo, _, _, _, _ := newProcessingServiceForTest(now)

	contract := &model.OtcOptionContract{
		OtcOfferID:          1,
		BuyerID:             10,
		SellerID:            20,
		StockAssetID:        1,
		Amount:              10,
		StrikePriceRSD:      50,
		PremiumRSD:          5,
		SettlementDate:      now.Add(24 * time.Hour),
		BuyerAccountNumber:  "buyer-rsd",
		SellerAccountNumber: "seller-rsd",
		Status:              model.OtcOptionContractStatusActive,
	}
	require.NoError(t, contractRepo.Create(context.Background(), contract))

	execution, err := svc.ExerciseContract(context.Background(), contract.OtcOptionContractID)
	require.Nil(t, execution)
	require.Error(t, err)
	require.Contains(t, err.Error(), "active OTC share reservation is required")
}

// --- failure-path tests ---

// setupExerciseScenario seeds a baseline contract+reservation+seller-ownership
// state so each failure test only needs to inject the specific fault it cares about.
func setupExerciseScenario(t *testing.T, now time.Time) (
	*OtcDealProcessingService,
	*processingContractRepo,
	*processingShareReservationRepo,
	*processingExecutionRepo,
	*processingOwnershipRepo,
	*processingBankingClient,
	*model.OtcOptionContract,
) {
	t.Helper()
	svc, _, contractRepo, reservationRepo, executionRepo, ownershipRepo, bankingClient := newProcessingServiceForTest(now)

	contract := &model.OtcOptionContract{
		OtcOfferID:          1,
		BuyerID:             10,
		SellerID:            20,
		StockAssetID:        1,
		Amount:              10,
		StrikePriceRSD:      50,
		PremiumRSD:          5,
		SettlementDate:      now.Add(24 * time.Hour),
		BuyerAccountNumber:  "buyer-rsd",
		SellerAccountNumber: "seller-rsd",
		Status:              model.OtcOptionContractStatusActive,
	}
	require.NoError(t, contractRepo.Create(context.Background(), contract))
	require.NoError(t, reservationRepo.Create(context.Background(), &model.OtcShareReservation{
		ContractID:     contract.OtcOptionContractID,
		SellerID:       20,
		OwnerType:      model.OwnerTypeClient,
		StockAssetID:   1,
		ReservedAmount: 10,
		Status:         model.OtcShareReservationStatusActive,
	}))
	ownershipRepo.seed(model.AssetOwnership{
		AssetOwnershipID: 1,
		UserId:           20,
		OwnerType:        model.OwnerTypeClient,
		AssetID:          1,
		Amount:           100,
		PublicAmount:     100,
		ReservedAmount:   10,
	})

	return svc, contractRepo, reservationRepo, executionRepo, ownershipRepo, bankingClient, contract
}

func TestOtcDealProcessingServiceExerciseContractMarksFailedOnTerminalReserveError(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	svc, _, reservationRepo, executionRepo, _, bankingClient, contract := setupExerciseScenario(t, now)

	bankingClient.reserveErr = status.Error(codes.InvalidArgument, "bad request")

	execution, err := svc.ExerciseContract(context.Background(), contract.OtcOptionContractID)
	require.NoError(t, err)
	require.NotNil(t, execution)
	require.Equal(t, model.OtcExecutionStatusFailed, execution.Status)
	require.Equal(t, model.OtcExecutionStepInit, execution.CurrentStep)
	require.Contains(t, execution.LastError, "bad request")
	require.Equal(t, 0, execution.RetryCount)

	require.Len(t, bankingClient.reserveCalls, 1)
	require.Empty(t, bankingClient.releaseCalls, "no funds were ever reserved on the banking side")
	require.Empty(t, bankingClient.commitCalls)
	require.Empty(t, bankingClient.refundCalls)

	storedReservation := reservationRepo.byContract[contract.OtcOptionContractID]
	require.Equal(t, model.OtcShareReservationStatusActive, storedReservation.Status)

	storedExecution := executionRepo.byID[execution.OtcExecutionSagaID]
	require.Equal(t, model.OtcExecutionStatusFailed, storedExecution.Status)
}

func TestOtcDealProcessingServiceExerciseContractRetriesOnTransientReserveError(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	svc, _, _, _, _, bankingClient, contract := setupExerciseScenario(t, now)

	bankingClient.reserveErr = status.Error(codes.Unavailable, "transient")

	execution, err := svc.ExerciseContract(context.Background(), contract.OtcOptionContractID)
	require.NoError(t, err)
	require.NotNil(t, execution)
	require.Equal(t, model.OtcExecutionStatusInProgress, execution.Status, "transient errors keep the saga in progress")
	require.Equal(t, model.OtcExecutionStepInit, execution.CurrentStep)
	require.Greater(t, execution.RetryCount, 0)
	require.NotNil(t, execution.NextRetryAt)
	require.True(t, execution.NextRetryAt.After(now), "next retry must be scheduled in the future")

	require.NotEmpty(t, bankingClient.reserveCalls)
	require.Empty(t, bankingClient.releaseCalls)
	require.Empty(t, bankingClient.commitCalls)
	require.Empty(t, bankingClient.refundCalls)
}

func TestOtcDealProcessingServiceExerciseContractReleasesFundsWhenSharesConfirmFails(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	svc, contractRepo, reservationRepo, _, ownershipRepo, bankingClient, contract := setupExerciseScenario(t, now)

	// Seller no longer has enough shares to settle (e.g. shares were liquidated elsewhere
	// after the contract was activated).
	ownershipRepo.seed(model.AssetOwnership{
		AssetOwnershipID: 1,
		UserId:           20,
		OwnerType:        model.OwnerTypeClient,
		AssetID:          1,
		Amount:           5,
		PublicAmount:     5,
		ReservedAmount:   10,
	})

	execution, err := svc.ExerciseContract(context.Background(), contract.OtcOptionContractID)
	require.NoError(t, err)
	require.NotNil(t, execution)
	require.Equal(t, model.OtcExecutionStatusFailed, execution.Status)
	require.Equal(t, model.OtcExecutionStepFundsReserved, execution.CurrentStep)

	require.Len(t, bankingClient.reserveCalls, 1)
	require.Len(t, bankingClient.releaseCalls, 1, "buyer's reserved funds must be released after share validation fails")
	require.Empty(t, bankingClient.commitCalls)
	require.Empty(t, bankingClient.refundCalls)

	storedContract := contractRepo.contracts[contract.OtcOptionContractID]
	require.Equal(t, model.OtcOptionContractStatusActive, storedContract.Status)
	require.Nil(t, storedContract.ExercisedAt)

	storedReservation := reservationRepo.byContract[contract.OtcOptionContractID]
	require.Equal(t, model.OtcShareReservationStatusActive, storedReservation.Status)
}

func TestOtcDealProcessingServiceExerciseContractFailsOnTerminalCommitError(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	svc, contractRepo, reservationRepo, _, _, bankingClient, contract := setupExerciseScenario(t, now)

	bankingClient.commitErr = status.Error(codes.FailedPrecondition, "commit rejected")

	execution, err := svc.ExerciseContract(context.Background(), contract.OtcOptionContractID)
	require.NoError(t, err)
	require.NotNil(t, execution)
	require.Equal(t, model.OtcExecutionStatusFailed, execution.Status)
	require.Equal(t, model.OtcExecutionStepFundsReserved, execution.CurrentStep)
	require.Contains(t, execution.LastError, "commit rejected")

	require.Len(t, bankingClient.reserveCalls, 1)
	require.Len(t, bankingClient.commitCalls, 1)
	require.Len(t, bankingClient.releaseCalls, 1, "terminal commit failure should release reserved funds")
	require.Empty(t, bankingClient.refundCalls)

	storedContract := contractRepo.contracts[contract.OtcOptionContractID]
	require.Equal(t, model.OtcOptionContractStatusActive, storedContract.Status)
	storedReservation := reservationRepo.byContract[contract.OtcOptionContractID]
	require.Equal(t, model.OtcShareReservationStatusActive, storedReservation.Status)
}

func TestOtcDealProcessingServiceExerciseContractRetriesOnTransientCommitError(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	svc, _, _, _, _, bankingClient, contract := setupExerciseScenario(t, now)

	bankingClient.commitErr = status.Error(codes.Unavailable, "commit transient")

	execution, err := svc.ExerciseContract(context.Background(), contract.OtcOptionContractID)
	require.NoError(t, err)
	require.NotNil(t, execution)
	require.Equal(t, model.OtcExecutionStatusInProgress, execution.Status)
	require.Equal(t, model.OtcExecutionStepSharesConfirmed, execution.CurrentStep)
	require.Greater(t, execution.RetryCount, 0)
	require.NotNil(t, execution.NextRetryAt)

	require.Len(t, bankingClient.reserveCalls, 1)
	require.GreaterOrEqual(t, len(bankingClient.commitCalls), 1)
	require.Empty(t, bankingClient.releaseCalls)
	require.Empty(t, bankingClient.refundCalls)
}

func TestOtcDealProcessingServiceExerciseContractRefundsFundsWhenOwnershipTransferFails(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	svc, contractRepo, reservationRepo, _, ownershipRepo, bankingClient, contract := setupExerciseScenario(t, now)

	// Simulate a DB-level failure during ownership upsert, which only happens
	// AFTER funds have been committed at the banking side. The saga must compensate
	// by refunding the transfer.
	ownershipRepo.upsertErr = errors.New("simulated db failure")

	execution, err := svc.ExerciseContract(context.Background(), contract.OtcOptionContractID)
	require.NoError(t, err)
	require.NotNil(t, execution)
	require.Equal(t, model.OtcExecutionStatusFailed, execution.Status)
	require.Equal(t, model.OtcExecutionStepFundsCommitted, execution.CurrentStep)

	require.Len(t, bankingClient.reserveCalls, 1)
	require.Len(t, bankingClient.commitCalls, 1)
	require.Len(t, bankingClient.refundCalls, 1, "committed funds must be refunded after ownership transfer fails")
	require.Empty(t, bankingClient.releaseCalls)

	storedContract := contractRepo.contracts[contract.OtcOptionContractID]
	require.Equal(t, model.OtcOptionContractStatusActive, storedContract.Status)
	require.Nil(t, storedContract.ExercisedAt)

	storedReservation := reservationRepo.byContract[contract.OtcOptionContractID]
	require.Equal(t, model.OtcShareReservationStatusActive, storedReservation.Status)
}

func TestOtcDealProcessingServiceExerciseContractRetriesCompensationWhenRefundFails(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	svc, _, _, executionRepo, ownershipRepo, bankingClient, contract := setupExerciseScenario(t, now)

	ownershipRepo.upsertErr = errors.New("simulated db failure")
	bankingClient.refundErr = status.Error(codes.Unavailable, "refund transient")

	execution, err := svc.ExerciseContract(context.Background(), contract.OtcOptionContractID)
	require.NoError(t, err)
	require.NotNil(t, execution)
	require.Equal(t, model.OtcExecutionStatusCompensating, execution.Status)
	require.Equal(t, model.OtcExecutionStepFundsCommitted, execution.CurrentStep)
	require.Greater(t, execution.RetryCount, 0)
	require.NotNil(t, execution.NextRetryAt)
	require.Len(t, bankingClient.refundCalls, 1)

	ownershipRepo.upsertErr = nil
	bankingClient.refundErr = nil
	svc.now = func() time.Time { return now.Add(time.Minute) }

	require.NoError(t, svc.ProcessPendingExecutions(context.Background()))

	stored := executionRepo.byID[execution.OtcExecutionSagaID]
	require.Equal(t, model.OtcExecutionStatusFailed, stored.Status)
	require.Equal(t, model.OtcExecutionStepFundsCommitted, stored.CurrentStep)
	require.Nil(t, stored.NextRetryAt)
	require.Len(t, bankingClient.refundCalls, 2, "worker should retry the refund compensation")
}

func TestOtcDealProcessingServiceExerciseContractCanRetryAfterFailed(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	svc, _, _, executionRepo, _, bankingClient, contract := setupExerciseScenario(t, now)

	// First attempt: terminal banking error.
	bankingClient.reserveErr = status.Error(codes.InvalidArgument, "first attempt")
	failed, err := svc.ExerciseContract(context.Background(), contract.OtcOptionContractID)
	require.NoError(t, err)
	require.Equal(t, model.OtcExecutionStatusFailed, failed.Status)
	firstKey := failed.ExecutionKey
	require.NotEmpty(t, firstKey)

	// Banking recovers; advance the clock so the regenerated key differs.
	bankingClient.reserveErr = nil
	svc.now = func() time.Time { return now.Add(time.Second) }

	completed, err := svc.ExerciseContract(context.Background(), contract.OtcOptionContractID)
	require.NoError(t, err)
	require.Equal(t, model.OtcExecutionStatusCompleted, completed.Status)
	require.Equal(t, model.OtcExecutionStepCompleted, completed.CurrentStep)
	require.NotEqual(t, firstKey, completed.ExecutionKey, "execution key should be regenerated on retry")
	require.Equal(t, 0, completed.RetryCount)
	require.Empty(t, completed.LastError)

	require.Len(t, bankingClient.reserveCalls, 2)
	require.Len(t, bankingClient.commitCalls, 1)

	// Same saga row was reused (no second saga created).
	require.Len(t, executionRepo.byID, 1)
	require.Equal(t, completed.OtcExecutionSagaID, failed.OtcExecutionSagaID)
}

func TestOtcDealProcessingServiceProcessPendingExecutionsResumesScheduledRetry(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	svc, _, _, executionRepo, _, bankingClient, contract := setupExerciseScenario(t, now)

	bankingClient.reserveErr = status.Error(codes.Unavailable, "transient")

	execution, err := svc.ExerciseContract(context.Background(), contract.OtcOptionContractID)
	require.NoError(t, err)
	require.Equal(t, model.OtcExecutionStatusInProgress, execution.Status)
	require.NotNil(t, execution.NextRetryAt)

	bankingClient.reserveErr = nil
	svc.now = func() time.Time { return now.Add(time.Minute) }

	require.NoError(t, svc.ProcessPendingExecutions(context.Background()))

	stored := executionRepo.byID[execution.OtcExecutionSagaID]
	require.Equal(t, model.OtcExecutionStatusCompleted, stored.Status)
	require.Equal(t, model.OtcExecutionStepCompleted, stored.CurrentStep)
	require.GreaterOrEqual(t, len(bankingClient.reserveCalls), 2, "worker should retry the failed reserve")
	require.GreaterOrEqual(t, len(bankingClient.commitCalls), 1)
}
