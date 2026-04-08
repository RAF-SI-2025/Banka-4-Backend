package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	commonerrors "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
)

const (
	otcExecutionPollInterval  = 2 * time.Second
	otcExecutionRetryInterval = 10 * time.Second
	maxOtcExecutionsPerTick   = 25
)

type OtcExecutionService struct {
	contractRepo         repository.OtcContractRepository
	shareReservationRepo repository.OtcShareReservationRepository
	executionRepo        repository.OtcExecutionSagaRepository
	assetOwnershipRepo   repository.AssetOwnershipRepository
	txManager            repository.TransactionManager
	bankingClient        client.BankingClient

	now func() time.Time

	lockMu sync.Mutex
	locks  map[string]*sync.Mutex

	mu     sync.Mutex
	cancel context.CancelFunc
}

func NewOtcExecutionService(
	contractRepo repository.OtcContractRepository,
	shareReservationRepo repository.OtcShareReservationRepository,
	executionRepo repository.OtcExecutionSagaRepository,
	assetOwnershipRepo repository.AssetOwnershipRepository,
	txManager repository.TransactionManager,
	bankingClient client.BankingClient,
) *OtcExecutionService {
	return &OtcExecutionService{
		contractRepo:         contractRepo,
		shareReservationRepo: shareReservationRepo,
		executionRepo:        executionRepo,
		assetOwnershipRepo:   assetOwnershipRepo,
		txManager:            txManager,
		bankingClient:        bankingClient,
		now:                  time.Now,
		locks:                make(map[string]*sync.Mutex),
	}
}

func (s *OtcExecutionService) Start() {
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.mu.Unlock()

	ticker := time.NewTicker(otcExecutionPollInterval)
	go func() {
		defer ticker.Stop()
		_ = s.ProcessPendingExecutions(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = s.ProcessPendingExecutions(ctx)
			}
		}
	}()
}

func (s *OtcExecutionService) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

func (s *OtcExecutionService) ExerciseContract(ctx context.Context, contractID uint, buyerAccountNumber string) (*model.OtcExecutionSaga, error) {
	saga, err := s.ensureSaga(ctx, contractID, buyerAccountNumber)
	if err != nil {
		return nil, err
	}

	err = s.processExecution(ctx, saga.OtcExecutionSagaID)
	latest, latestErr := s.GetExecutionStatus(ctx, saga.OtcExecutionSagaID)
	if latestErr != nil {
		if err != nil {
			return nil, err
		}
		return nil, latestErr
	}

	return latest, err
}

func (s *OtcExecutionService) GetExecutionStatus(ctx context.Context, executionID uint) (*model.OtcExecutionSaga, error) {
	execution, err := s.executionRepo.FindByID(ctx, executionID)
	if err != nil {
		return nil, commonerrors.InternalErr(err)
	}
	if execution == nil {
		return nil, commonerrors.NotFoundErr("OTC execution not found")
	}

	return execution, nil
}

func (s *OtcExecutionService) ProcessPendingExecutions(ctx context.Context) error {
	executions, err := s.executionRepo.FindPendingForExecution(ctx, s.now(), maxOtcExecutionsPerTick)
	if err != nil {
		return commonerrors.InternalErr(err)
	}

	for _, execution := range executions {
		if err := s.processExecution(ctx, execution.OtcExecutionSagaID); err != nil {
			continue
		}
	}

	return nil
}

func (s *OtcExecutionService) ensureSaga(ctx context.Context, contractID uint, buyerAccountNumber string) (*model.OtcExecutionSaga, error) {
	buyerAccountNumber = strings.TrimSpace(buyerAccountNumber)
	if buyerAccountNumber == "" {
		return nil, commonerrors.BadRequestErr("buyer account number is required")
	}

	var execution *model.OtcExecutionSaga
	err := s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		contract, err := s.contractRepo.FindByIDForUpdate(ctx, contractID)
		if err != nil {
			return commonerrors.InternalErr(err)
		}
		if contract == nil {
			return commonerrors.NotFoundErr("OTC contract not found")
		}

		if err := s.validateContractForExecution(ctx, contract); err != nil {
			return err
		}

		reservation, err := s.shareReservationRepo.FindByContractIDForUpdate(ctx, contractID)
		if err != nil {
			return commonerrors.InternalErr(err)
		}
		if reservation == nil || reservation.Status != model.OtcShareReservationStatusActive {
			return commonerrors.BadRequestErr("active OTC share reservation is required")
		}

		execution, err = s.executionRepo.FindByContractIDForUpdate(ctx, contractID)
		if err != nil {
			return commonerrors.InternalErr(err)
		}
		if execution != nil {
			switch execution.Status {
			case model.OtcExecutionStatusCompleted:
				return commonerrors.ConflictErr("OTC contract has already been exercised")
			case model.OtcExecutionStatusFailed:
				execution.BuyerAccountNumber = buyerAccountNumber
				execution.CurrentStep = model.OtcExecutionStepInit
				execution.Status = model.OtcExecutionStatusInProgress
				execution.RetryCount = 0
				execution.NextRetryAt = nil
				execution.LastError = ""
				execution.CompletedAt = nil
				execution.UpdatedAt = s.now()
				return s.executionRepo.Save(ctx, execution)
			default:
				if execution.BuyerAccountNumber != buyerAccountNumber {
					return commonerrors.ConflictErr("OTC execution is already running with another buyer account")
				}
				return nil
			}
		}

		execution = &model.OtcExecutionSaga{
			ContractID:         contractID,
			BuyerAccountNumber: buyerAccountNumber,
			CurrentStep:        model.OtcExecutionStepInit,
			Status:             model.OtcExecutionStatusInProgress,
			CreatedAt:          s.now(),
			UpdatedAt:          s.now(),
		}
		if err := s.executionRepo.Create(ctx, execution); err != nil {
			return commonerrors.InternalErr(err)
		}

		execution.ExecutionKey = "otc-execution-" + strconv.FormatUint(uint64(execution.OtcExecutionSagaID), 10)
		execution.BankingReservationID = execution.ExecutionKey
		execution.UpdatedAt = s.now()
		return s.executionRepo.Save(ctx, execution)
	})
	if err != nil {
		return nil, err
	}

	return execution, nil
}

func (s *OtcExecutionService) processExecution(ctx context.Context, executionID uint) error {
	lockKey := strconv.FormatUint(uint64(executionID), 10)
	return s.withExecutionLock(lockKey, func() error {
		for i := 0; i < 8; i++ {
			execution, err := s.executionRepo.FindByID(ctx, executionID)
			if err != nil {
				return commonerrors.InternalErr(err)
			}
			if execution == nil {
				return commonerrors.NotFoundErr("OTC execution not found")
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
	})
}

func (s *OtcExecutionService) processStep(ctx context.Context, execution *model.OtcExecutionSaga) (bool, error) {
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
		return false, s.completeExecution(ctx, execution)
	default:
		return false, commonerrors.BadRequestErr("unknown OTC execution step")
	}
}

func (s *OtcExecutionService) reserveFunds(ctx context.Context, execution *model.OtcExecutionSaga) error {
	resp, err := s.bankingClient.ReserveOtcFunds(ctx, &pb.ReserveOtcFundsRequest{
		ExecutionId:         execution.ExecutionKey,
		BuyerAccountNumber:  execution.BuyerAccountNumber,
		SellerAccountNumber: execution.Contract.SellerAccountNumber,
		Amount:              s.tradeAmount(execution.Contract),
		CurrencyCode:        execution.Contract.TradeCurrencyCode,
	})
	if err != nil {
		if isTerminalBankingError(err) {
			return s.markFailed(ctx, execution.OtcExecutionSagaID, execution.CurrentStep, err.Error())
		}
		return s.scheduleRetry(ctx, execution.OtcExecutionSagaID, execution.CurrentStep, model.OtcExecutionStatusInProgress, err.Error())
	}

	return s.advanceExecution(ctx, execution.OtcExecutionSagaID, model.OtcExecutionStepFundsReserved, model.OtcExecutionStatusInProgress, resp.GetExecutionId(), "")
}

func (s *OtcExecutionService) confirmShares(ctx context.Context, execution *model.OtcExecutionSaga) error {
	err := s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		contract, err := s.contractRepo.FindByIDForUpdate(ctx, execution.ContractID)
		if err != nil {
			return commonerrors.InternalErr(err)
		}
		if contract == nil {
			return commonerrors.NotFoundErr("OTC contract not found")
		}
		if err := s.validateContractForExecution(ctx, contract); err != nil {
			return err
		}

		reservation, err := s.shareReservationRepo.FindByContractIDForUpdate(ctx, contract.OtcContractID)
		if err != nil {
			return commonerrors.InternalErr(err)
		}
		if reservation == nil || reservation.Status != model.OtcShareReservationStatusActive {
			return commonerrors.BadRequestErr("active OTC share reservation is required")
		}
		if reservation.ReservedAmount < float64(contract.Quantity) {
			return commonerrors.BadRequestErr("OTC share reservation amount is insufficient")
		}

		if err := s.ensureSellerCapacity(ctx, contract, true); err != nil {
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
		return nil
	}

	var appErr *commonerrors.AppError
	if errorAs(err, &appErr) && appErr.Code < 500 {
		return s.releaseAndFail(ctx, execution, appErr.Error())
	}

	return s.scheduleRetry(ctx, execution.OtcExecutionSagaID, execution.CurrentStep, model.OtcExecutionStatusInProgress, err.Error())
}

func (s *OtcExecutionService) commitFunds(ctx context.Context, execution *model.OtcExecutionSaga) error {
	resp, err := s.bankingClient.CommitOtcFunds(ctx, execution.ExecutionKey)
	if err != nil {
		if isTerminalBankingError(err) {
			return s.releaseAndFail(ctx, execution, err.Error())
		}
		return s.scheduleRetry(ctx, execution.OtcExecutionSagaID, execution.CurrentStep, model.OtcExecutionStatusInProgress, err.Error())
	}

	return s.advanceExecution(ctx, execution.OtcExecutionSagaID, model.OtcExecutionStepFundsCommitted, model.OtcExecutionStatusInProgress, resp.GetExecutionId(), "")
}

func (s *OtcExecutionService) transferOwnership(ctx context.Context, execution *model.OtcExecutionSaga) error {
	err := s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		contract, err := s.contractRepo.FindByIDForUpdate(ctx, execution.ContractID)
		if err != nil {
			return commonerrors.InternalErr(err)
		}
		if contract == nil {
			return commonerrors.NotFoundErr("OTC contract not found")
		}
		if err := s.validateContractForExecution(ctx, contract); err != nil {
			return err
		}

		reservation, err := s.shareReservationRepo.FindByContractIDForUpdate(ctx, contract.OtcContractID)
		if err != nil {
			return commonerrors.InternalErr(err)
		}
		if reservation == nil || reservation.Status != model.OtcShareReservationStatusActive {
			return commonerrors.BadRequestErr("active OTC share reservation is required")
		}

		sellerOwnership, err := s.assetOwnershipRepo.FindByOwnerAndAssetForUpdate(ctx, contract.SellerIdentityID, contract.SellerOwnerType, contract.AssetID)
		if err != nil {
			return commonerrors.InternalErr(err)
		}
		if sellerOwnership == nil {
			return commonerrors.BadRequestErr("seller does not own the reserved asset")
		}

		otherReserved, err := s.shareReservationRepo.SumActiveReservedBySellerAsset(ctx, contract.SellerIdentityID, contract.SellerOwnerType, contract.AssetID, &contract.OtcContractID)
		if err != nil {
			return commonerrors.InternalErr(err)
		}

		quantity := float64(contract.Quantity)
		if sellerOwnership.Amount < quantity || sellerOwnership.Amount-quantity < otherReserved {
			return commonerrors.BadRequestErr("seller no longer has enough shares to settle this OTC contract")
		}

		buyerOwnership, err := s.assetOwnershipRepo.FindByOwnerAndAssetForUpdate(ctx, contract.BuyerIdentityID, contract.BuyerOwnerType, contract.AssetID)
		if err != nil {
			return commonerrors.InternalErr(err)
		}
		if buyerOwnership == nil {
			buyerOwnership = &model.AssetOwnership{
				IdentityID: contract.BuyerIdentityID,
				OwnerType:  contract.BuyerOwnerType,
				AssetID:    contract.AssetID,
				UpdatedAt:  s.now(),
			}
		}

		sellerOwnership.Amount -= quantity
		sellerOwnership.UpdatedAt = s.now()
		if err := s.assetOwnershipRepo.Upsert(ctx, sellerOwnership); err != nil {
			return commonerrors.InternalErr(err)
		}

		newAmount := buyerOwnership.Amount + quantity
		if newAmount > 0 {
			buyerOwnership.AvgBuyPrice = (buyerOwnership.AvgBuyPrice*buyerOwnership.Amount + contract.StrikePrice*quantity) / newAmount
		}
		buyerOwnership.Amount = newAmount
		buyerOwnership.UpdatedAt = s.now()
		if err := s.assetOwnershipRepo.Upsert(ctx, buyerOwnership); err != nil {
			return commonerrors.InternalErr(err)
		}

		now := s.now()
		contract.Status = model.OtcContractStatusExercised
		contract.ExercisedAt = &now
		contract.UpdatedAt = now
		if err := s.contractRepo.Save(ctx, contract); err != nil {
			return commonerrors.InternalErr(err)
		}

		reservation.Status = model.OtcShareReservationStatusConsumed
		reservation.UpdatedAt = now
		if err := s.shareReservationRepo.Save(ctx, reservation); err != nil {
			return commonerrors.InternalErr(err)
		}

		execution.CurrentStep = model.OtcExecutionStepOwnershipTransferred
		execution.Status = model.OtcExecutionStatusInProgress
		execution.NextRetryAt = nil
		execution.LastError = ""
		execution.UpdatedAt = now
		return s.executionRepo.Save(ctx, execution)
	})
	if err != nil {
		return s.scheduleCompensating(ctx, execution.OtcExecutionSagaID, model.OtcExecutionStepFundsCommitted, err.Error())
	}

	return nil
}

func (s *OtcExecutionService) completeExecution(ctx context.Context, execution *model.OtcExecutionSaga) error {
	return s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		current, err := s.executionRepo.FindByContractIDForUpdate(ctx, execution.ContractID)
		if err != nil {
			return commonerrors.InternalErr(err)
		}
		if current == nil {
			return commonerrors.NotFoundErr("OTC execution not found")
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

func (s *OtcExecutionService) handleCompensation(ctx context.Context, execution *model.OtcExecutionSaga) error {
	switch execution.CurrentStep {
	case model.OtcExecutionStepFundsReserved:
		_, err := s.bankingClient.ReleaseOtcFunds(ctx, execution.ExecutionKey)
		if err != nil {
			return s.scheduleCompensating(ctx, execution.OtcExecutionSagaID, execution.CurrentStep, err.Error())
		}
		return s.markFailed(ctx, execution.OtcExecutionSagaID, execution.CurrentStep, execution.LastError)
	case model.OtcExecutionStepFundsCommitted:
		_, err := s.bankingClient.RefundOtcFunds(ctx, execution.ExecutionKey)
		if err != nil {
			return s.scheduleCompensating(ctx, execution.OtcExecutionSagaID, execution.CurrentStep, err.Error())
		}
		return s.markFailed(ctx, execution.OtcExecutionSagaID, execution.CurrentStep, execution.LastError)
	default:
		return s.markFailed(ctx, execution.OtcExecutionSagaID, execution.CurrentStep, execution.LastError)
	}
}

func (s *OtcExecutionService) releaseAndFail(ctx context.Context, execution *model.OtcExecutionSaga, reason string) error {
	_, err := s.bankingClient.ReleaseOtcFunds(ctx, execution.ExecutionKey)
	if err != nil {
		return s.scheduleCompensating(ctx, execution.OtcExecutionSagaID, model.OtcExecutionStepFundsReserved, reason)
	}

	return s.markFailed(ctx, execution.OtcExecutionSagaID, model.OtcExecutionStepFundsReserved, reason)
}

func (s *OtcExecutionService) validateContractForExecution(ctx context.Context, contract *model.OtcContract) error {
	if contract.Status == model.OtcContractStatusExercised {
		return commonerrors.ConflictErr("OTC contract has already been exercised")
	}
	if contract.Status == model.OtcContractStatusCancelled {
		return commonerrors.BadRequestErr("OTC contract is cancelled")
	}
	if contract.Status == model.OtcContractStatusExpired || s.now().After(contract.SettlementDate) {
		contract.Status = model.OtcContractStatusExpired
		contract.UpdatedAt = s.now()
		if err := s.contractRepo.Save(ctx, contract); err != nil {
			return commonerrors.InternalErr(err)
		}
		return commonerrors.BadRequestErr("OTC contract has expired")
	}
	if contract.Status != model.OtcContractStatusActive {
		return commonerrors.BadRequestErr("OTC contract is not active")
	}

	return nil
}

func (s *OtcExecutionService) ensureSellerCapacity(ctx context.Context, contract *model.OtcContract, excludeCurrent bool) error {
	sellerOwnership, err := s.assetOwnershipRepo.FindByOwnerAndAssetForUpdate(ctx, contract.SellerIdentityID, contract.SellerOwnerType, contract.AssetID)
	if err != nil {
		return commonerrors.InternalErr(err)
	}
	if sellerOwnership == nil {
		return commonerrors.BadRequestErr("seller does not own the reserved asset")
	}

	var excludeID *uint
	if excludeCurrent {
		excludeID = &contract.OtcContractID
	}
	otherReserved, err := s.shareReservationRepo.SumActiveReservedBySellerAsset(ctx, contract.SellerIdentityID, contract.SellerOwnerType, contract.AssetID, excludeID)
	if err != nil {
		return commonerrors.InternalErr(err)
	}

	if sellerOwnership.Amount < float64(contract.Quantity)+otherReserved {
		return commonerrors.BadRequestErr("seller no longer has enough shares to settle this OTC contract")
	}

	return nil
}

func (s *OtcExecutionService) tradeAmount(contract model.OtcContract) float64 {
	return float64(contract.Quantity) * contract.StrikePrice
}

func (s *OtcExecutionService) advanceExecution(ctx context.Context, executionID uint, step model.OtcExecutionStep, statusValue model.OtcExecutionStatus, bankingReservationID, lastError string) error {
	return s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		execution, err := s.executionRepo.FindByID(ctx, executionID)
		if err != nil {
			return commonerrors.InternalErr(err)
		}
		if execution == nil {
			return commonerrors.NotFoundErr("OTC execution not found")
		}

		execution.CurrentStep = step
		execution.Status = statusValue
		execution.NextRetryAt = nil
		execution.LastError = lastError
		if bankingReservationID != "" {
			execution.BankingReservationID = bankingReservationID
		}
		execution.UpdatedAt = s.now()
		return s.executionRepo.Save(ctx, execution)
	})
}

func (s *OtcExecutionService) scheduleRetry(ctx context.Context, executionID uint, currentStep model.OtcExecutionStep, statusValue model.OtcExecutionStatus, lastError string) error {
	return s.updateRetryState(ctx, executionID, currentStep, statusValue, lastError)
}

func (s *OtcExecutionService) scheduleCompensating(ctx context.Context, executionID uint, currentStep model.OtcExecutionStep, lastError string) error {
	return s.updateRetryState(ctx, executionID, currentStep, model.OtcExecutionStatusCompensating, lastError)
}

func (s *OtcExecutionService) updateRetryState(ctx context.Context, executionID uint, currentStep model.OtcExecutionStep, statusValue model.OtcExecutionStatus, lastError string) error {
	return s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		execution, err := s.executionRepo.FindByID(ctx, executionID)
		if err != nil {
			return commonerrors.InternalErr(err)
		}
		if execution == nil {
			return commonerrors.NotFoundErr("OTC execution not found")
		}

		nextRetryAt := s.now().Add(otcExecutionRetryInterval)
		execution.CurrentStep = currentStep
		execution.Status = statusValue
		execution.RetryCount++
		execution.NextRetryAt = &nextRetryAt
		execution.LastError = lastError
		execution.UpdatedAt = s.now()
		return s.executionRepo.Save(ctx, execution)
	})
}

func (s *OtcExecutionService) markFailed(ctx context.Context, executionID uint, currentStep model.OtcExecutionStep, lastError string) error {
	return s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		execution, err := s.executionRepo.FindByID(ctx, executionID)
		if err != nil {
			return commonerrors.InternalErr(err)
		}
		if execution == nil {
			return commonerrors.NotFoundErr("OTC execution not found")
		}

		execution.CurrentStep = currentStep
		execution.Status = model.OtcExecutionStatusFailed
		execution.NextRetryAt = nil
		execution.LastError = lastError
		execution.UpdatedAt = s.now()
		return s.executionRepo.Save(ctx, execution)
	})
}

func (s *OtcExecutionService) withExecutionLock(key string, fn func() error) error {
	mutex := s.executionMutex(key)
	mutex.Lock()
	defer mutex.Unlock()
	return fn()
}

func (s *OtcExecutionService) executionMutex(key string) *sync.Mutex {
	s.lockMu.Lock()
	defer s.lockMu.Unlock()

	mutex, ok := s.locks[key]
	if !ok {
		mutex = &sync.Mutex{}
		s.locks[key] = mutex
	}

	return mutex
}

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

func errorAs(err error, target **commonerrors.AppError) bool {
	appErr, ok := err.(*commonerrors.AppError)
	if ok {
		*target = appErr
		return true
	}
	return false
}

func (s *OtcExecutionService) String() string {
	return fmt.Sprintf("OtcExecutionService(retry=%s)", otcExecutionRetryInterval)
}
