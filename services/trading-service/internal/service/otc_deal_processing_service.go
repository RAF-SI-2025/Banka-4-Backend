package service

import (
	"context"
	stderrors "errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	appErrors "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	otcWorkerPollInterval   = 15 * time.Second
	otcExecutionRetryDelay  = 30 * time.Second
	maxOtcExecutionsPerRun  = 25
	maxExpiredContractsRun  = 25
	maxExecutionStepsPerRun = 8
)

type OtcDealProcessingService struct {
	offerRepo            repository.OtcOfferRepository
	optionContractRepo   repository.OtcOptionContractRepository
	shareReservationRepo repository.OtcShareReservationRepository
	executionRepo        repository.OtcExecutionSagaRepository
	assetOwnershipRepo   repository.AssetOwnershipRepository
	txManager            repository.TransactionManager
	bankingClient        client.BankingClient
	otcTaxService        *OtcTaxService

	now func() time.Time

	lifecycleMu sync.Mutex
	cancel      context.CancelFunc
}

func NewOtcDealProcessingService(
	offerRepo repository.OtcOfferRepository,
	optionContractRepo repository.OtcOptionContractRepository,
	shareReservationRepo repository.OtcShareReservationRepository,
	executionRepo repository.OtcExecutionSagaRepository,
	assetOwnershipRepo repository.AssetOwnershipRepository,
	txManager repository.TransactionManager,
	bankingClient client.BankingClient,
	otcTaxService *OtcTaxService,
) *OtcDealProcessingService {
	return &OtcDealProcessingService{
		offerRepo:            offerRepo,
		optionContractRepo:   optionContractRepo,
		shareReservationRepo: shareReservationRepo,
		executionRepo:        executionRepo,
		assetOwnershipRepo:   assetOwnershipRepo,
		txManager:            txManager,
		bankingClient:        bankingClient,
		otcTaxService:        otcTaxService,
		now:                  time.Now,
	}
}

func (s *OtcDealProcessingService) Start() {
	s.lifecycleMu.Lock()
	if s.cancel != nil {
		s.lifecycleMu.Unlock()
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.lifecycleMu.Unlock()

	ticker := time.NewTicker(otcWorkerPollInterval)
	go func() {
		defer ticker.Stop()
		s.runMaintenance(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.runMaintenance(ctx)
			}
		}
	}()
}

func (s *OtcDealProcessingService) Stop() {
	s.lifecycleMu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.lifecycleMu.Unlock()

	if cancel != nil {
		cancel()
	}
}

// FinalizeAgreement turns an accepted OTC offer into an active option contract
// by charging the premium, reserving the seller's shares, and persisting the
// contract activation state with compensation if local activation fails.
func (s *OtcDealProcessingService) FinalizeAgreement(ctx context.Context, offerID uint, acceptedBy uint) (*model.OtcOptionContract, error) {
	var offerSnapshot *model.OtcOffer
	var existingContractID uint

	err := s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		offer, err := s.offerRepo.FindByIDForUpdate(ctx, offerID)
		if err != nil {
			return appErrors.InternalErr(err)
		}

		if offer == nil {
			return appErrors.NotFoundErr("offer not found")
		}

		if offer.Status == model.OtcOfferStatusAccepted && offer.OptionContractID != nil {
			existingContractID = *offer.OptionContractID
			return nil
		}

		if offer.Status != model.OtcOfferStatusActive {
			return appErrors.BadRequestErr("offer is not active")
		}

		if offer.SellerAccountNumber == nil || strings.TrimSpace(*offer.SellerAccountNumber) == "" {
			return appErrors.BadRequestErr("seller account number is missing")
		}

		if err := s.validateSellerCapacityForActivation(ctx, offer); err != nil {
			return err
		}

		offerSnapshot = new(*offer)
		return nil
	})

	if err != nil {
		return nil, err
	}

	if existingContractID != 0 {
		return s.optionContractRepo.FindByID(ctx, existingContractID)
	}

	if offerSnapshot == nil {
		return nil, appErrors.InternalErr(fmt.Errorf("offer snapshot missing during OTC finalization"))
	}

	if _, err := s.bankingClient.CreatePaymentWithoutVerification(ctx, &pb.CreatePaymentRequest{
		PayerAccountNumber:     offerSnapshot.BuyerAccountNumber,
		RecipientAccountNumber: *offerSnapshot.SellerAccountNumber,
		Amount:                 offerSnapshot.PremiumRSD,
		PaymentCode:            "289",
		Purpose:                fmt.Sprintf("OTC premium for offer #%d", offerSnapshot.OtcOfferID),
	}); err != nil {
		return nil, appErrors.InternalErr(fmt.Errorf("premium transfer failed: %w", err))
	}

	var contractID uint
	err = s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		offer, err := s.offerRepo.FindByIDForUpdate(ctx, offerID)
		if err != nil {
			return appErrors.InternalErr(err)
		}

		if offer == nil {
			return appErrors.NotFoundErr("offer not found")
		}

		if offer.Status == model.OtcOfferStatusAccepted && offer.OptionContractID != nil {
			contractID = *offer.OptionContractID
			return nil
		}

		if offer.Status != model.OtcOfferStatusActive {
			return appErrors.BadRequestErr("offer is not active")
		}

		if offer.SellerAccountNumber == nil || strings.TrimSpace(*offer.SellerAccountNumber) == "" {
			return appErrors.BadRequestErr("seller account number is missing")
		}

		if err := s.validateSellerCapacityForActivation(ctx, offer); err != nil {
			return err
		}

		now := s.now()
		contract := &model.OtcOptionContract{
			OtcOfferID:          offer.OtcOfferID,
			BuyerID:             offer.BuyerID,
			SellerID:            offer.SellerID,
			StockAssetID:        offer.StockAssetID,
			Amount:              offer.Amount,
			StrikePriceRSD:      offer.PricePerStockRSD,
			PremiumRSD:          offer.PremiumRSD,
			SettlementDate:      offer.SettlementDate,
			BuyerAccountNumber:  offer.BuyerAccountNumber,
			SellerAccountNumber: *offer.SellerAccountNumber,
			Status:              model.OtcOptionContractStatusActive,
			CreatedAt:           now,
			UpdatedAt:           now,
		}

		if err := s.optionContractRepo.Create(ctx, contract); err != nil {
			return appErrors.InternalErr(err)
		}

		reservation := &model.OtcShareReservation{
			ContractID:     contract.OtcOptionContractID,
			SellerID:       offer.SellerID,
			OwnerType:      model.OwnerTypeClient,
			StockAssetID:   offer.StockAssetID,
			ReservedAmount: float64(offer.Amount),
			Status:         model.OtcShareReservationStatusActive,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		if err := s.shareReservationRepo.Create(ctx, reservation); err != nil {
			return appErrors.InternalErr(err)
		}

		sellerOwnership, err := s.assetOwnershipRepo.FindByUserAndAssetForUpdate(ctx, offer.SellerID, model.OwnerTypeClient, offer.StockAssetID)
		if err != nil {
			return appErrors.InternalErr(err)
		}

		if sellerOwnership == nil {
			return appErrors.BadRequestErr("seller does not own the offered stock")
		}

		sellerOwnership.ReservedAmount += float64(offer.Amount)
		sellerOwnership.UpdatedAt = now
		if err := s.assetOwnershipRepo.Upsert(ctx, sellerOwnership); err != nil {
			return appErrors.InternalErr(err)
		}

		offer.Status = model.OtcOfferStatusAccepted
		offer.OptionContractID = &contract.OtcOptionContractID
		offer.LastModified = now
		offer.ModifiedBy = acceptedBy
		if err := s.offerRepo.Save(ctx, offer); err != nil {
			return appErrors.InternalErr(err)
		}

		contractID = contract.OtcOptionContractID
		return nil
	})

	if err != nil {
		if compensateErr := s.compensatePremiumTransfer(ctx, offerSnapshot); compensateErr != nil {
			return nil, appErrors.InternalErr(fmt.Errorf("activation failed after premium transfer: %w; compensation failed: %v", err, compensateErr))
		}

		return nil, err
	}

	contract, err := s.optionContractRepo.FindByID(ctx, contractID)
	if err != nil {
		return nil, appErrors.InternalErr(err)
	}

	// Seller's premium is taxable income (15%). Best-effort: a tax-recording
	// failure must not roll back an already-settled premium transfer.
	if s.otcTaxService != nil {
		if taxErr := s.otcTaxService.RecordPremiumTax(ctx, contract); taxErr != nil {
			log.Printf("[otc-tax] failed to record premium tax for contract %d: %v", contract.OtcOptionContractID, taxErr)
		}
	}

	return contract, nil
}

// ExerciseContract starts or resumes the OTC exercise saga for an active
// contract and returns the latest persisted execution state after processing.
func (s *OtcDealProcessingService) ExerciseContract(ctx context.Context, contractID uint) (*model.OtcExecutionSaga, error) {
	execution, err := s.ensureExecutionSaga(ctx, contractID)
	if err != nil {
		return nil, err
	}

	processErr := s.processExecution(ctx, execution.OtcExecutionSagaID)
	latest, latestErr := s.GetExecutionStatus(ctx, execution.OtcExecutionSagaID)
	if latestErr != nil {
		if processErr != nil {
			return nil, processErr
		}

		return nil, latestErr
	}

	return latest, processErr
}

// GetExecutionStatus returns the current persisted state of an OTC execution saga.
func (s *OtcDealProcessingService) GetExecutionStatus(ctx context.Context, executionID uint) (*model.OtcExecutionSaga, error) {
	execution, err := s.executionRepo.FindByID(ctx, executionID)
	if err != nil {
		return nil, appErrors.InternalErr(err)
	}

	if execution == nil {
		return nil, appErrors.NotFoundErr("OTC execution not found")
	}

	return execution, nil
}

// ProcessPendingExecutions resumes retryable OTC executions whose scheduled retry
// time has arrived.
func (s *OtcDealProcessingService) ProcessPendingExecutions(ctx context.Context) error {
	executions, err := s.executionRepo.FindPendingForExecution(ctx, s.now(), maxOtcExecutionsPerRun)
	if err != nil {
		return appErrors.InternalErr(err)
	}

	for i := range executions {
		_ = s.processExecution(ctx, executions[i].OtcExecutionSagaID)
	}

	return nil
}

// ProcessExpiredContracts expires active OTC contracts whose settlement time has
// passed and releases their share reservations.
func (s *OtcDealProcessingService) ProcessExpiredContracts(ctx context.Context) error {
	contracts, err := s.optionContractRepo.FindExpiredActive(ctx, s.now(), maxExpiredContractsRun)
	if err != nil {
		return appErrors.InternalErr(err)
	}

	for i := range contracts {
		_ = s.expireContract(ctx, contracts[i].OtcOptionContractID)
	}

	return nil
}

// runMaintenance performs periodic OTC background work for contract expiry and
// execution recovery.
func (s *OtcDealProcessingService) runMaintenance(ctx context.Context) {
	_ = s.ProcessExpiredContracts(ctx)
	_ = s.ProcessPendingExecutions(ctx)
}

// ensureExecutionSaga prepares the persisted execution state for exercising an
// OTC contract. It locks and validates the contract and its active share
// reservation, then either:
// - returns the existing in-progress execution,
// - resets a failed execution back to INIT for a new attempt, or
// - creates a brand-new execution saga when none exists.
// If the contract was already exercised, it returns a conflict error.
func (s *OtcDealProcessingService) ensureExecutionSaga(ctx context.Context, contractID uint) (*model.OtcExecutionSaga, error) {
	var execution *model.OtcExecutionSaga
	expired := false
	err := s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		contract, err := s.optionContractRepo.FindByIDForUpdate(ctx, contractID)
		if err != nil {
			return appErrors.InternalErr(err)
		}

		if contract == nil {
			return appErrors.NotFoundErr("OTC contract not found")
		}

		if s.shouldExpireContract(contract) {
			if err := s.expireLockedContract(ctx, contract); err != nil {
				return err
			}
			expired = true
			return nil
		}

		if err := s.validateContractForExecution(ctx, contract); err != nil {
			return err
		}

		reservation, err := s.shareReservationRepo.FindByContractIDForUpdate(ctx, contractID)
		if err != nil {
			return appErrors.InternalErr(err)
		}

		if reservation == nil || reservation.Status != model.OtcShareReservationStatusActive {
			return appErrors.BadRequestErr("active OTC share reservation is required")
		}

		execution, err = s.executionRepo.FindByContractIDForUpdate(ctx, contractID)
		if err != nil {
			return appErrors.InternalErr(err)
		}

		if execution != nil {
			switch execution.Status {
			case model.OtcExecutionStatusCompleted:
				return appErrors.ConflictErr("OTC contract has already been exercised")
			case model.OtcExecutionStatusFailed:
				execution.ExecutionKey = s.newExecutionKey(contractID)
				execution.CurrentStep = model.OtcExecutionStepInit
				execution.Status = model.OtcExecutionStatusInProgress
				execution.RetryCount = 0
				execution.NextRetryAt = nil
				execution.LastError = ""
				execution.CompletedAt = nil
				execution.UpdatedAt = s.now()
				return s.executionRepo.Save(ctx, execution)
			default:
				return nil
			}
		}

		now := s.now()
		execution = &model.OtcExecutionSaga{
			ContractID:   contractID,
			ExecutionKey: s.newExecutionKey(contractID),
			CurrentStep:  model.OtcExecutionStepInit,
			Status:       model.OtcExecutionStatusInProgress,
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		return s.executionRepo.Create(ctx, execution)
	})

	if err != nil {
		return nil, err
	}

	if expired {
		return nil, appErrors.BadRequestErr("OTC contract has expired")
	}

	return execution, nil
}

// processExecution drives an OTC execution saga forward by repeatedly loading
// the latest persisted state and applying the next saga step. It stops when the
// execution reaches a terminal state, switches into compensation flow, or can
// no longer advance in the current run. A fixed step limit prevents accidental
// infinite looping if state transitions become inconsistent.
func (s *OtcDealProcessingService) processExecution(ctx context.Context, executionID uint) error {
	for i := 0; i < maxExecutionStepsPerRun; i++ {
		execution, err := s.executionRepo.FindByID(ctx, executionID)
		if err != nil {
			return appErrors.InternalErr(err)
		}

		if execution == nil {
			return appErrors.NotFoundErr("OTC execution not found")
		}

		switch execution.Status {
		case model.OtcExecutionStatusCompleted, model.OtcExecutionStatusFailed:
			return nil
		case model.OtcExecutionStatusCompensating:
			return s.handleCompensation(ctx, execution)
		}

		advanced, err := s.processStep(ctx, execution)
		if err != nil {
			return err
		}

		if !advanced {
			return nil
		}
	}

	return nil
}

// processStep dispatches the next action for the execution based on its current
// persisted saga step and reports whether another step may be attempted in the
// same processing run.
func (s *OtcDealProcessingService) processStep(ctx context.Context, execution *model.OtcExecutionSaga) (bool, error) {
	switch execution.CurrentStep {
	case model.OtcExecutionStepInit:
		return true, s.reserveFunds(ctx, execution)
	case model.OtcExecutionStepFundsReserved:
		return true, s.confirmShares(ctx, execution)
	case model.OtcExecutionStepSharesConfirmed:
		return true, s.commitFunds(ctx, execution)
	case model.OtcExecutionStepFundsCommitted:
		return true, s.transferOwnership(ctx, execution)
	case model.OtcExecutionStepOwnershipTransferred:
		return true, s.completeExecution(ctx, execution)
	case model.OtcExecutionStepCompleted:
		return false, nil
	default:
		return false, appErrors.BadRequestErr("unknown OTC execution step")
	}
}

// reserveFunds performs the first settlement step by reserving the buyer's
// funds in banking. On success, it advances the saga to FUNDS_RESERVED; on
// failure it either marks the execution failed or schedules a retry depending
// on whether the banking error is terminal.
func (s *OtcDealProcessingService) reserveFunds(ctx context.Context, execution *model.OtcExecutionSaga) error {
	resp, err := s.bankingClient.ReserveOtcFunds(ctx, &pb.ReserveOtcFundsRequest{
		ExecutionId:         execution.ExecutionKey,
		BuyerAccountNumber:  execution.Contract.BuyerAccountNumber,
		SellerAccountNumber: execution.Contract.SellerAccountNumber,
		Amount:              float64(execution.Contract.Amount) * execution.Contract.StrikePriceRSD,
		CurrencyCode:        "RSD",
	})

	if err != nil {
		if isTerminalBankingError(err) {
			return s.markFailed(ctx, execution.OtcExecutionSagaID, execution.CurrentStep, err.Error())
		}

		return s.scheduleRetry(ctx, execution.OtcExecutionSagaID, execution.CurrentStep, model.OtcExecutionStatusInProgress, err.Error())
	}

	return s.advanceExecution(ctx, execution.OtcExecutionSagaID, model.OtcExecutionStepFundsReserved, model.OtcExecutionStatusInProgress, resp.GetExecutionId(), "")
}

// confirmShares revalidates the locked contract and its active share
// reservation before settlement continues. If the seller can no longer cover
// the reserved shares, the previously reserved funds are released and the
// execution fails; retryable errors are deferred for another attempt.
func (s *OtcDealProcessingService) confirmShares(ctx context.Context, execution *model.OtcExecutionSaga) error {
	expired := false
	err := s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		contract, err := s.optionContractRepo.FindByIDForUpdate(ctx, execution.ContractID)
		if err != nil {
			return appErrors.InternalErr(err)
		}

		if contract == nil {
			return appErrors.NotFoundErr("OTC contract not found")
		}

		if s.shouldExpireContract(contract) {
			if err := s.expireLockedContract(ctx, contract); err != nil {
				return err
			}
			expired = true
			return nil
		}

		if err := s.validateContractForExecution(ctx, contract); err != nil {
			return err
		}

		reservation, err := s.shareReservationRepo.FindByContractIDForUpdate(ctx, execution.ContractID)
		if err != nil {
			return appErrors.InternalErr(err)
		}

		if reservation == nil || reservation.Status != model.OtcShareReservationStatusActive {
			return appErrors.BadRequestErr("active OTC share reservation is required")
		}

		if err := s.ensureSellerCapacityForSettlement(ctx, contract, reservation); err != nil {
			return err
		}

		execution.CurrentStep = model.OtcExecutionStepSharesConfirmed
		execution.Status = model.OtcExecutionStatusInProgress
		execution.NextRetryAt = nil
		execution.LastError = ""
		execution.UpdatedAt = s.now()
		return s.executionRepo.Save(ctx, execution)
	})

	if err == nil {
		if expired {
			return s.releaseAndFail(ctx, execution, "OTC contract has expired")
		}
		return nil
	}

	if appErr, ok := stderrors.AsType[*appErrors.AppError](err); ok && appErr.Code < 500 {
		return s.releaseAndFail(ctx, execution, appErr.Error())
	}

	return s.scheduleRetry(ctx, execution.OtcExecutionSagaID, execution.CurrentStep, model.OtcExecutionStatusInProgress, err.Error())
}

// commitFunds finalizes the previously reserved banking transfer. On success, it
// advances the saga to FUNDS_COMMITTED; on failure it either releases the
// reserved funds and fails the execution or schedules a retry, depending on
// whether the banking error is terminal.
func (s *OtcDealProcessingService) commitFunds(ctx context.Context, execution *model.OtcExecutionSaga) error {
	resp, err := s.bankingClient.CommitOtcFunds(ctx, execution.ExecutionKey)
	if err != nil {
		if isTerminalBankingError(err) {
			return s.releaseAndFail(ctx, execution, err.Error())
		}

		return s.scheduleRetry(ctx, execution.OtcExecutionSagaID, execution.CurrentStep, model.OtcExecutionStatusInProgress, err.Error())
	}

	return s.advanceExecution(ctx, execution.OtcExecutionSagaID, model.OtcExecutionStepFundsCommitted, model.OtcExecutionStatusInProgress, resp.GetExecutionId(), "")
}

// transferOwnership applies the local settlement effects after funds were
// committed: it moves the stock position from seller to buyer, marks the
// contract exercised, consumes the share reservation, and advances the saga to
// OWNERSHIP_TRANSFERRED. If any local update fails, the execution is switched
// into compensation mode so the committed funds can be refunded.
func (s *OtcDealProcessingService) transferOwnership(ctx context.Context, execution *model.OtcExecutionSaga) error {
	expired := false
	err := s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		contract, err := s.optionContractRepo.FindByIDForUpdate(ctx, execution.ContractID)
		if err != nil {
			return appErrors.InternalErr(err)
		}

		if contract == nil {
			return appErrors.NotFoundErr("OTC contract not found")
		}

		if s.shouldExpireContract(contract) {
			if err := s.expireLockedContract(ctx, contract); err != nil {
				return err
			}
			expired = true
			return nil
		}

		if err := s.validateContractForExecution(ctx, contract); err != nil {
			return err
		}

		reservation, err := s.shareReservationRepo.FindByContractIDForUpdate(ctx, execution.ContractID)
		if err != nil {
			return appErrors.InternalErr(err)
		}

		if reservation == nil || reservation.Status != model.OtcShareReservationStatusActive {
			return appErrors.BadRequestErr("active OTC share reservation is required")
		}

		if err := s.ensureSellerCapacityForSettlement(ctx, contract, reservation); err != nil {
			return err
		}

		now := s.now()
		sellerOwnership, err := s.assetOwnershipRepo.FindByUserAndAssetForUpdate(ctx, contract.SellerID, model.OwnerTypeClient, contract.StockAssetID)
		if err != nil {
			return appErrors.InternalErr(err)
		}

		if sellerOwnership == nil {
			return appErrors.BadRequestErr("seller does not own the reserved stock")
		}

		quantity := float64(contract.Amount)
		sellerOwnership.Amount -= quantity
		if sellerOwnership.PublicAmount >= quantity {
			sellerOwnership.PublicAmount -= quantity
		} else {
			sellerOwnership.PublicAmount = 0
		}

		if sellerOwnership.ReservedAmount >= quantity {
			sellerOwnership.ReservedAmount -= quantity
		} else {
			sellerOwnership.ReservedAmount = 0
		}

		sellerOwnership.UpdatedAt = now
		if err := s.assetOwnershipRepo.Upsert(ctx, sellerOwnership); err != nil {
			return appErrors.InternalErr(err)
		}

		buyerOwnership, err := s.assetOwnershipRepo.FindByUserAndAssetForUpdate(ctx, contract.BuyerID, model.OwnerTypeClient, contract.StockAssetID)
		if err != nil {
			return appErrors.InternalErr(err)
		}

		if buyerOwnership == nil {
			buyerOwnership = &model.AssetOwnership{
				UserId:         contract.BuyerID,
				OwnerType:      model.OwnerTypeClient,
				AssetID:        contract.StockAssetID,
				Amount:         0,
				AvgBuyPriceRSD: 0,
				PublicAmount:   0,
				ReservedAmount: 0,
			}
		}

		newAmount := buyerOwnership.Amount + quantity
		if newAmount > 0 {
			buyerOwnership.AvgBuyPriceRSD = (buyerOwnership.AvgBuyPriceRSD*buyerOwnership.Amount + contract.StrikePriceRSD*quantity) / newAmount
		}

		buyerOwnership.Amount = newAmount
		buyerOwnership.UpdatedAt = now
		if err := s.assetOwnershipRepo.Upsert(ctx, buyerOwnership); err != nil {
			return appErrors.InternalErr(err)
		}

		contract.Status = model.OtcOptionContractStatusExercised
		contract.ExercisedAt = &now
		contract.UpdatedAt = now
		if err := s.optionContractRepo.Save(ctx, contract); err != nil {
			return appErrors.InternalErr(err)
		}

		reservation.Status = model.OtcShareReservationStatusConsumed
		reservation.UpdatedAt = now
		if err := s.shareReservationRepo.Save(ctx, reservation); err != nil {
			return appErrors.InternalErr(err)
		}

		execution.CurrentStep = model.OtcExecutionStepOwnershipTransferred
		execution.Status = model.OtcExecutionStatusInProgress
		execution.NextRetryAt = nil
		execution.LastError = ""
		execution.UpdatedAt = now
		return s.executionRepo.Save(ctx, execution)
	})

	if expired {
		return s.scheduleCompensating(ctx, execution.OtcExecutionSagaID, model.OtcExecutionStepFundsCommitted, "OTC contract has expired")
	}

	if err != nil {
		return s.scheduleCompensating(ctx, execution.OtcExecutionSagaID, model.OtcExecutionStepFundsCommitted, err.Error())
	}

	// Ownership transfer succeeded — the buyer has exercised the option. The
	// realized gain ((market − strike) × qty − premium) is taxable (15%).
	// Best-effort: recorded outside the settlement transaction so a tax failure
	// never reverts a completed exercise.
	if s.otcTaxService != nil {
		exercised, fetchErr := s.optionContractRepo.FindByID(ctx, execution.ContractID)
		if fetchErr != nil {
			log.Printf("[otc-tax] failed to load exercised contract %d for tax: %v", execution.ContractID, fetchErr)
		} else if taxErr := s.otcTaxService.RecordExerciseTax(ctx, exercised); taxErr != nil {
			log.Printf("[otc-tax] failed to record exercise tax for contract %d: %v", execution.ContractID, taxErr)
		}
	}

	return nil
}

// completeExecution marks the execution saga as successfully finished after all
// settlement side effects were applied.
func (s *OtcDealProcessingService) completeExecution(ctx context.Context, execution *model.OtcExecutionSaga) error {
	return s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		current, err := s.executionRepo.FindByContractIDForUpdate(ctx, execution.ContractID)
		if err != nil {
			return appErrors.InternalErr(err)
		}

		if current == nil {
			return appErrors.NotFoundErr("OTC execution not found")
		}

		if current.Status == model.OtcExecutionStatusCompleted {
			return nil
		}

		now := s.now()
		current.CurrentStep = model.OtcExecutionStepCompleted
		current.Status = model.OtcExecutionStatusCompleted
		current.NextRetryAt = nil
		current.LastError = ""
		current.CompletedAt = &now
		current.UpdatedAt = now
		return s.executionRepo.Save(ctx, current)
	})
}

// handleCompensation executes the required undo step for a compensating
// execution, releasing reserved funds or refunding committed funds depending on
// how far the saga progressed before failing.
func (s *OtcDealProcessingService) handleCompensation(ctx context.Context, execution *model.OtcExecutionSaga) error {
	switch execution.CurrentStep {
	case model.OtcExecutionStepFundsReserved:
		if _, err := s.bankingClient.ReleaseOtcFunds(ctx, execution.ExecutionKey); err != nil {
			return s.scheduleCompensating(ctx, execution.OtcExecutionSagaID, execution.CurrentStep, err.Error())
		}
		return s.markFailed(ctx, execution.OtcExecutionSagaID, execution.CurrentStep, execution.LastError)
	case model.OtcExecutionStepFundsCommitted:
		if _, err := s.bankingClient.RefundOtcFunds(ctx, execution.ExecutionKey); err != nil {
			return s.scheduleCompensating(ctx, execution.OtcExecutionSagaID, execution.CurrentStep, err.Error())
		}
		return s.markFailed(ctx, execution.OtcExecutionSagaID, execution.CurrentStep, execution.LastError)
	default:
		return s.markFailed(ctx, execution.OtcExecutionSagaID, execution.CurrentStep, execution.LastError)
	}
}

// releaseAndFail attempts to release previously reserved funds and then marks
// the execution as failed.
func (s *OtcDealProcessingService) releaseAndFail(ctx context.Context, execution *model.OtcExecutionSaga, reason string) error {
	if _, err := s.bankingClient.ReleaseOtcFunds(ctx, execution.ExecutionKey); err != nil {
		return s.scheduleCompensating(ctx, execution.OtcExecutionSagaID, model.OtcExecutionStepFundsReserved, reason)
	}

	return s.markFailed(ctx, execution.OtcExecutionSagaID, model.OtcExecutionStepFundsReserved, reason)
}

// expireContract locks an OTC contract and expires it if it is still active and
// its settlement time has already passed.
func (s *OtcDealProcessingService) expireContract(ctx context.Context, contractID uint) error {
	return s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		contract, err := s.optionContractRepo.FindByIDForUpdate(ctx, contractID)
		if err != nil {
			return appErrors.InternalErr(err)
		}

		if contract == nil {
			return appErrors.NotFoundErr("OTC contract not found")
		}

		if contract.Status != model.OtcOptionContractStatusActive || contract.SettlementDate.After(s.now()) {
			return nil
		}

		return s.expireLockedContract(ctx, contract)
	})
}

// validateContractForExecution verifies that the contract is still active and
// eligible for exercise, expiring it on the spot if its settlement time has
// already passed.
func (s *OtcDealProcessingService) validateContractForExecution(_ context.Context, contract *model.OtcOptionContract) error {
	if contract.Status == model.OtcOptionContractStatusExercised {
		return appErrors.ConflictErr("OTC contract has already been exercised")
	}

	if contract.Status == model.OtcOptionContractStatusCancelled {
		return appErrors.BadRequestErr("OTC contract is cancelled")
	}

	if contract.Status == model.OtcOptionContractStatusExpired || !contract.SettlementDate.After(s.now()) {
		return appErrors.BadRequestErr("OTC contract has expired")
	}

	if contract.Status != model.OtcOptionContractStatusActive {
		return appErrors.BadRequestErr("OTC contract is not active")
	}

	return nil
}

func (s *OtcDealProcessingService) shouldExpireContract(contract *model.OtcOptionContract) bool {
	return contract.Status == model.OtcOptionContractStatusActive && !contract.SettlementDate.After(s.now())
}

// validateSellerCapacityForActivation checks that the seller still has enough
// public shares to activate this offer into an OTC contract, after accounting
// for already reserved shares and other active offers for the same stock.
func (s *OtcDealProcessingService) validateSellerCapacityForActivation(ctx context.Context, offer *model.OtcOffer) error {
	sellerOwnership, err := s.assetOwnershipRepo.FindByUserAndAssetForUpdate(ctx, offer.SellerID, model.OwnerTypeClient, offer.StockAssetID)
	if err != nil {
		return appErrors.InternalErr(err)
	}

	if sellerOwnership == nil {
		return appErrors.BadRequestErr("seller does not own the offered stock")
	}

	activeOffers, err := s.offerRepo.FindActiveBySellerAndStock(ctx, offer.SellerID, offer.StockAssetID, &offer.OtcOfferID)
	if err != nil {
		return appErrors.InternalErr(err)
	}

	committedByOffers := 0.0
	for _, activeOffer := range activeOffers {
		committedByOffers += float64(activeOffer.Amount)
	}

	required := sellerOwnership.ReservedAmount + committedByOffers + float64(offer.Amount)
	if sellerOwnership.PublicAmount < required {
		return appErrors.BadRequestErr(fmt.Sprintf(
			"seller does not have enough public shares: public=%.0f, already committed=%.0f, additionally requested=%d",
			sellerOwnership.PublicAmount,
			sellerOwnership.ReservedAmount+committedByOffers,
			offer.Amount,
		))
	}

	return nil
}

// ensureSellerCapacityForSettlement verifies that the seller still holds enough
// shares to honor this reservation while also covering all other active OTC
// share reservations for the same stock.
func (s *OtcDealProcessingService) ensureSellerCapacityForSettlement(ctx context.Context, contract *model.OtcOptionContract, reservation *model.OtcShareReservation) error {
	sellerOwnership, err := s.assetOwnershipRepo.FindByUserAndAssetForUpdate(ctx, contract.SellerID, model.OwnerTypeClient, contract.StockAssetID)
	if err != nil {
		return appErrors.InternalErr(err)
	}

	if sellerOwnership == nil {
		return appErrors.BadRequestErr("seller does not own the reserved stock")
	}

	otherReserved, err := s.shareReservationRepo.SumActiveReservedBySellerAsset(ctx, contract.SellerID, model.OwnerTypeClient, contract.StockAssetID, &contract.OtcOptionContractID)
	if err != nil {
		return appErrors.InternalErr(err)
	}

	if sellerOwnership.Amount < reservation.ReservedAmount || sellerOwnership.Amount-reservation.ReservedAmount < otherReserved {
		return appErrors.BadRequestErr("seller no longer has enough shares to settle this OTC contract")
	}

	return nil
}

// expireLockedContract finalizes expiry for a locked OTC contract by freeing the
// reserved seller shares, marking the reservation as released, and saving the
// contract in EXPIRED status.
func (s *OtcDealProcessingService) expireLockedContract(ctx context.Context, contract *model.OtcOptionContract) error {
	now := s.now()
	reservation, err := s.shareReservationRepo.FindByContractIDForUpdate(ctx, contract.OtcOptionContractID)
	if err != nil {
		return appErrors.InternalErr(err)
	}

	if reservation != nil && reservation.Status == model.OtcShareReservationStatusActive {
		sellerOwnership, err := s.assetOwnershipRepo.FindByUserAndAssetForUpdate(ctx, contract.SellerID, model.OwnerTypeClient, contract.StockAssetID)
		if err != nil {
			return appErrors.InternalErr(err)
		}

		if sellerOwnership != nil {
			if sellerOwnership.ReservedAmount >= reservation.ReservedAmount {
				sellerOwnership.ReservedAmount -= reservation.ReservedAmount
			} else {
				sellerOwnership.ReservedAmount = 0
			}

			sellerOwnership.UpdatedAt = now
			if err := s.assetOwnershipRepo.Upsert(ctx, sellerOwnership); err != nil {
				return appErrors.InternalErr(err)
			}
		}

		reservation.Status = model.OtcShareReservationStatusReleased
		reservation.UpdatedAt = now
		if err := s.shareReservationRepo.Save(ctx, reservation); err != nil {
			return appErrors.InternalErr(err)
		}
	}

	contract.Status = model.OtcOptionContractStatusExpired
	contract.UpdatedAt = now
	return s.optionContractRepo.Save(ctx, contract)
}

// compensatePremiumTransfer attempts to reverse a previously successful premium
// payment when OTC agreement activation fails after the external transfer step.
func (s *OtcDealProcessingService) compensatePremiumTransfer(ctx context.Context, offer *model.OtcOffer) error {
	_, err := s.bankingClient.CreatePaymentWithoutVerification(ctx, &pb.CreatePaymentRequest{
		PayerAccountNumber:     *offer.SellerAccountNumber,
		RecipientAccountNumber: offer.BuyerAccountNumber,
		Amount:                 offer.PremiumRSD,
		PaymentCode:            "289",
		Purpose:                fmt.Sprintf("OTC premium compensation for offer #%d", offer.OtcOfferID),
	})

	return err
}

// advanceExecution persists a successful saga transition, moving the execution
// to the next step and clearing any pending retry scheduling or stale error state.
func (s *OtcDealProcessingService) advanceExecution(ctx context.Context, executionID uint, step model.OtcExecutionStep, statusValue model.OtcExecutionStatus, executionKey, lastError string) error {
	return s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		execution, err := s.executionRepo.FindByID(ctx, executionID)
		if err != nil {
			return appErrors.InternalErr(err)
		}

		if execution == nil {
			return appErrors.NotFoundErr("OTC execution not found")
		}

		execution.CurrentStep = step
		execution.Status = statusValue
		execution.NextRetryAt = nil
		execution.LastError = lastError

		if strings.TrimSpace(executionKey) != "" {
			execution.ExecutionKey = executionKey
		}

		execution.UpdatedAt = s.now()
		return s.executionRepo.Save(ctx, execution)
	})
}

// scheduleRetry marks the execution for a later retry without switching it
// into compensation mode.
func (s *OtcDealProcessingService) scheduleRetry(ctx context.Context, executionID uint, currentStep model.OtcExecutionStep, statusValue model.OtcExecutionStatus, lastError string) error {
	return s.updateRetryState(ctx, executionID, currentStep, statusValue, lastError)
}

// scheduleCompensating marks the execution for a later compensation attempt
// after a failure that occurred past a compensatable saga step.
func (s *OtcDealProcessingService) scheduleCompensating(ctx context.Context, executionID uint, currentStep model.OtcExecutionStep, lastError string) error {
	return s.updateRetryState(ctx, executionID, currentStep, model.OtcExecutionStatusCompensating, lastError)
}

// updateRetryState stores a retryable execution state, recording the current
// step, retry count, last error, and the next scheduled retry time.
func (s *OtcDealProcessingService) updateRetryState(ctx context.Context, executionID uint, currentStep model.OtcExecutionStep, statusValue model.OtcExecutionStatus, lastError string) error {
	return s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		execution, err := s.executionRepo.FindByID(ctx, executionID)
		if err != nil {
			return appErrors.InternalErr(err)
		}

		if execution == nil {
			return appErrors.NotFoundErr("OTC execution not found")
		}

		execution.CurrentStep = currentStep
		execution.Status = statusValue
		execution.RetryCount++
		execution.NextRetryAt = new(s.now().Add(otcExecutionRetryDelay))
		execution.LastError = lastError
		execution.UpdatedAt = s.now()
		return s.executionRepo.Save(ctx, execution)
	})
}

// markFailed persists a terminal failed state for the execution, clearing any
// scheduled retry and recording the step and error that caused the failure.
func (s *OtcDealProcessingService) markFailed(ctx context.Context, executionID uint, currentStep model.OtcExecutionStep, lastError string) error {
	return s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		execution, err := s.executionRepo.FindByID(ctx, executionID)
		if err != nil {
			return appErrors.InternalErr(err)
		}

		if execution == nil {
			return appErrors.NotFoundErr("OTC execution not found")
		}

		execution.CurrentStep = currentStep
		execution.Status = model.OtcExecutionStatusFailed
		execution.NextRetryAt = nil
		execution.LastError = lastError
		execution.UpdatedAt = s.now()
		return s.executionRepo.Save(ctx, execution)
	})
}

// isTerminalBankingError reports whether a banking gRPC error is safe to treat
// as non-retryable, meaning the current OTC saga step should fail or compensate
// instead of being retried automatically.
//
// This method returns true for banking errors that should not be retried
func isTerminalBankingError(err error) bool {
	st, ok := status.FromError(err)
	if !ok {
		return false
	}

	switch st.Code() {
	case codes.InvalidArgument, codes.NotFound, codes.FailedPrecondition, codes.AlreadyExists, codes.PermissionDenied:
		return true
	default:
		return false
	}
}

func (s *OtcDealProcessingService) newExecutionKey(contractID uint) string {
	return fmt.Sprintf("otc-execution-%d-%d", contractID, s.now().UnixNano())
}
