package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/repository"
)

const (
	defaultTransferPage     = 1
	defaultTransferPageSize = 10
	maxTransferPageSize     = 100
)

type transferCalculation struct {
	fromAccount *model.Account
	toAccount   *model.Account
	initial     float64
	debit       float64
	final       float64
	exchange    *float64
	commission  float64
}

type TransferService struct {
	transferRepo         repository.TransferRepository
	transactionRepo      repository.TransactionRepository
	accountRepo          repository.AccountRepository
	exchangeService      CurrencyConverter
	txManager            repository.TransactionManager
	transactionProcessor *TransactionProcessor
	userClient           client.UserClient
	mailer               Mailer
}

func NewTransferService(
	transferRepo repository.TransferRepository,
	transactionRepo repository.TransactionRepository,
	accountRepo repository.AccountRepository,
	exchangeService CurrencyConverter,
	txManager repository.TransactionManager,
	transactionProcessor *TransactionProcessor,
	userClient client.UserClient,
	mailer Mailer,
) *TransferService {
	return &TransferService{
		transferRepo:         transferRepo,
		transactionRepo:      transactionRepo,
		accountRepo:          accountRepo,
		exchangeService:      exchangeService,
		txManager:            txManager,
		transactionProcessor: transactionProcessor,
		userClient:           userClient,
		mailer:               mailer,
	}
}

func (s *TransferService) ExecuteTransfer(ctx context.Context, req dto.TransferRequest) (*dto.TransferResponse, error) {
	clientID, err := auth.GetSubjectFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var createdTransfer *model.Transfer

	err = s.txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
		calculation, err := s.calculateTransfer(txCtx, clientID, req)
		if err != nil {
			return err
		}

		transaction := &model.Transaction{
			PayerAccountNumber:     calculation.fromAccount.AccountNumber,
			RecipientAccountNumber: calculation.toAccount.AccountNumber,
			StartAmount:            calculation.debit,
			StartCurrencyCode:      calculation.fromAccount.Currency.Code,
			EndAmount:              calculation.final,
			EndCurrencyCode:        calculation.toAccount.Currency.Code,
			Commission:             calculation.commission,
			Status:                 model.TransactionProcessing,
		}

		if err := s.transactionRepo.Create(txCtx, transaction); err != nil {
			return errors.InternalErr(err)
		}

		transfer := &model.Transfer{
			TransactionID: transaction.TransactionID,
			ExchangeRate:  calculation.exchange,
			Transaction:   *transaction,
		}

		if err := s.transferRepo.Create(txCtx, transfer); err != nil {
			return errors.InternalErr(err)
		}

		if err := s.transactionProcessor.Process(txCtx, transaction.TransactionID); err != nil {
			return err
		}

		createdTransfer = transfer
		return nil
	})

	if err != nil {
		return nil, err
	}

	if err := s.sendTransferExecutedEmail(ctx, clientID, createdTransfer); err != nil {
		return nil, err
	}

	response := dto.ToTransferResponse(createdTransfer)
	return &response, nil
}

func (s *TransferService) sendTransferExecutedEmail(ctx context.Context, clientID uint, transfer *model.Transfer) error {
	clientInfo, err := s.userClient.GetClientByID(ctx, clientID)
	if err != nil {
		return err
	}
	if clientInfo == nil {
		return errors.InternalErr(fmt.Errorf("client not found in user service"))
	}

	body := fmt.Sprintf(
		"Hello %s,\n\nYour transfer has been successfully executed.\n\nFrom: %s\nTo: %s\nAmount: %.2f %s",
		defaultContactName(clientInfo.FullName),
		transfer.Transaction.PayerAccountNumber,
		transfer.Transaction.RecipientAccountNumber,
		transfer.Transaction.StartAmount,
		transfer.Transaction.StartCurrencyCode,
	)

	if err := s.mailer.Send(clientInfo.Email, "Transfer executed", body); err != nil {
		return errors.ServiceUnavailableErr(err)
	}

	return nil
}

func (s *TransferService) GetTransferHistory(ctx context.Context, clientID uint, page, pageSize int) (*dto.ListTransfersResponse, error) {
	if page < 1 {
		page = defaultTransferPage
	}

	if pageSize < 1 || pageSize > maxTransferPageSize {
		pageSize = defaultTransferPageSize
	}

	transfers, total, err := s.transferRepo.ListByClientID(ctx, clientID, page, pageSize)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	items := make([]dto.TransferResponse, 0, len(transfers))
	for i := range transfers {
		items = append(items, dto.ToTransferResponse(&transfers[i]))
	}

	totalPages := 0
	if total > 0 {
		totalPages = (int(total) + pageSize - 1) / pageSize
	}

	return &dto.ListTransfersResponse{
		Data:       items,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

func (s *TransferService) calculateTransfer(ctx context.Context, clientID uint, req dto.TransferRequest) (*transferCalculation, error) {
	fromAccountNumber := strings.TrimSpace(req.FromAccountNumber)
	toAccountNumber := strings.TrimSpace(req.ToAccountNumber)
	if fromAccountNumber == "" || toAccountNumber == "" {
		return nil, errors.BadRequestErr("from_account_number and to_account_number are required")
	}

	if fromAccountNumber == toAccountNumber {
		return nil, errors.BadRequestErr("source and destination account must be different")
	}

	if req.Amount <= 0 {
		return nil, errors.BadRequestErr("amount must be greater than 0")
	}

	fromAccount, err := s.accountRepo.FindByAccountNumber(ctx, fromAccountNumber)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	if fromAccount == nil {
		return nil, errors.NotFoundErr("source account not found")
	}

	toAccount, err := s.accountRepo.FindByAccountNumber(ctx, toAccountNumber)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	if toAccount == nil {
		return nil, errors.NotFoundErr("destination account not found")
	}

	if fromAccount.ClientID != clientID || toAccount.ClientID != clientID {
		return nil, errors.ForbiddenErr("accounts do not belong to the provided client")
	}

	if !isAccountActive(fromAccount.Status) {
		return nil, errors.BadRequestErr("source account is not active")
	}

	if !isAccountActive(toAccount.Status) {
		return nil, errors.BadRequestErr("destination account is not active")
	}

	calculation := &transferCalculation{
		fromAccount: fromAccount,
		toAccount:   toAccount,
		initial:     req.Amount,
		debit:       req.Amount,
		final:       req.Amount,
		commission:  0,
	}

	if fromAccount.Currency.Code != toAccount.Currency.Code {
		convertedAmount, err := s.exchangeService.Convert(ctx, req.Amount, fromAccount.Currency.Code, toAccount.Currency.Code)
		if err != nil {
			return nil, err
		}

		commission := s.exchangeService.CalculateFee(req.Amount)
		exchangeRate := convertedAmount / req.Amount

		calculation.debit = req.Amount + commission
		calculation.final = convertedAmount
		calculation.exchange = &exchangeRate
		calculation.commission = commission
	}

	if fromAccount.AvailableBalance < calculation.debit {
		return nil, errors.BadRequestErr("insufficient funds")
	}

	return calculation, nil
}

func isAccountActive(status string) bool {
	return strings.EqualFold(strings.TrimSpace(status), "Active")
}
