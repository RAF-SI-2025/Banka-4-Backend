package grpc

import (
	"context"
	stderrors "errors"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/repository"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/service"
)

type BankingService struct {
	pb.UnimplementedBankingServiceServer
	accountRepo          repository.AccountRepository
	accountService       *service.AccountService
	loanRepo             repository.LoanRepository
	paymentService       *service.PaymentService
	transactionRepo      repository.TransactionRepository
	transactionProcessor *service.TransactionProcessor
	exchangeService      *service.ExchangeService
}

func NewBankingService(
	accountRepo repository.AccountRepository,
	loanRepo repository.LoanRepository,
	paymentService *service.PaymentService,
	accountService *service.AccountService,
	transactionRepo repository.TransactionRepository,
	transactionProcessor *service.TransactionProcessor,
	exchangeService *service.ExchangeService,
) *BankingService {
	return &BankingService{
		accountRepo:          accountRepo,
		loanRepo:             loanRepo,
		paymentService:       paymentService,
		accountService:       accountService,
		transactionRepo:      transactionRepo,
		transactionProcessor: transactionProcessor,
		exchangeService:      exchangeService,
	}
}

func (s *BankingService) GetAccountByNumber(ctx context.Context, req *pb.GetAccountByNumberRequest) (*pb.GetAccountByNumberResponse, error) {
	account, err := s.accountRepo.FindByAccountNumber(ctx, req.AccountNumber)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch account: %v", err)
	}
	if account == nil {
		return nil, status.Errorf(codes.NotFound, "account %s not found", req.AccountNumber)
	}
	return &pb.GetAccountByNumberResponse{
		AccountNumber:    account.AccountNumber,
		ClientId:         uint64(account.ClientID),
		AccountType:      string(account.AccountType),
		CurrencyCode:     string(account.Currency.Code),
		AvailableBalance: account.AvailableBalance,
	}, nil
}

func (s *BankingService) HasActiveLoan(ctx context.Context, req *pb.HasActiveLoanRequest) (*pb.HasActiveLoanResponse, error) {
	hasActiveLoan, err := s.loanRepo.HasActiveByClientID(ctx, uint(req.GetClientId()))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check active loans: %v", err)
	}

	return &pb.HasActiveLoanResponse{HasActiveLoan: hasActiveLoan}, nil
}

func (s *BankingService) CreatePaymentWithoutVerification(ctx context.Context, req *pb.CreatePaymentRequest) (*pb.CreatePaymentResponse, error) {
	payment, err := s.paymentService.CreatePaymentWithoutVerification(ctx, dto.CreatePaymentRequest{
		PayerAccountNumber:     req.PayerAccountNumber,
		RecipientAccountNumber: req.RecipientAccountNumber,
		RecipientName:          req.RecipientName,
		Amount:                 req.Amount,
		ReferenceNumber:        req.ReferenceNumber,
		PaymentCode:            req.PaymentCode,
		Purpose:                req.Purpose,
		CommissionExempt:       req.CommissionExempt,
	})
	if err != nil {
		return nil, errors.MapGrpcToHttpError(err)
	}
	return &pb.CreatePaymentResponse{
		PaymentId:     uint64(payment.PaymentID),
		TransactionId: uint64(payment.TransactionID),
		Status:        string(payment.Transaction.Status),
	}, nil
}

func (s *BankingService) GetAccountsByClientID(ctx context.Context, req *pb.GetAccountsByClientIDRequest) (*pb.GetAccountsByClientIDResponse, error) {
	accounts, err := s.accountRepo.FindByClientID(ctx, uint(req.ClientId))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch accounts: %v", err)
	}

	pbAccounts := make([]*pb.AccountInfo, 0, len(accounts))
	for _, acc := range accounts {
		pbAccounts = append(pbAccounts, &pb.AccountInfo{
			AccountNumber: acc.AccountNumber,
			CurrencyCode:  string(acc.Currency.Code),
		})
	}

	return &pb.GetAccountsByClientIDResponse{Accounts: pbAccounts}, nil
}

func (s *BankingService) ConvertCurrency(ctx context.Context, req *pb.ConvertCurrencyRequest) (*pb.ConvertCurrencyResponse, error) {
	if req.Amount <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "amount must be positive")
	}
	if req.FromCode == "" || req.ToCode == "" {
		return nil, status.Errorf(codes.InvalidArgument, "from_code and to_code are required")
	}

	converted, err := s.exchangeService.Convert(
		ctx,
		req.Amount,
		model.CurrencyCode(req.FromCode),
		model.CurrencyCode(req.ToCode),
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "currency conversion failed: %v", err)
	}

	return &pb.ConvertCurrencyResponse{
		ConvertedAmount: converted,
	}, nil
}
func (s *BankingService) ExecuteTradeSettlement(ctx context.Context, req *pb.ExecuteTradeSettlementRequest) (*pb.ExecuteTradeSettlementResponse, error) {
	accountNumber := strings.TrimSpace(req.GetAccountNumber())
	if accountNumber == "" {
		return nil, status.Error(codes.InvalidArgument, "account_number is required")
	}

	tradeCurrencyCode := strings.ToUpper(strings.TrimSpace(req.GetTradeCurrencyCode()))
	if tradeCurrencyCode == "" {
		return nil, status.Error(codes.InvalidArgument, "trade_currency_code is required")
	}

	direction := req.GetDirection()
	if direction == pb.TradeSettlementDirection_TRADE_SETTLEMENT_DIRECTION_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "direction is required")
	}

	amount := req.GetAmount()

	if amount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be greater than zero")
	}

	customerAccount, err := s.accountRepo.FindByAccountNumber(ctx, accountNumber)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch account: %v", err)
	}
	if customerAccount == nil {
		return nil, status.Errorf(codes.NotFound, "account %s not found", accountNumber)
	}

	bankAccountNumber, err := resolveBankAccountNumber(tradeCurrencyCode)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	bankAccount, err := s.accountRepo.FindByAccountNumber(ctx, bankAccountNumber)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch bank settlement account: %v", err)
	}
	if bankAccount == nil {
		return nil, status.Errorf(codes.NotFound, "bank settlement account %s not found", bankAccountNumber)
	}

	sourceAccount := customerAccount
	destinationAccount := bankAccount
	amountIsSource := false

	switch direction {
	case pb.TradeSettlementDirection_TRADE_SETTLEMENT_DIRECTION_BUY:
	case pb.TradeSettlementDirection_TRADE_SETTLEMENT_DIRECTION_SELL:
		sourceAccount = bankAccount
		destinationAccount = customerAccount
		amountIsSource = true
	default:
		return nil, status.Error(codes.InvalidArgument, "direction must be BUY or SELL")
	}

	sourceAmount := amount
	destinationAmount := amount
	if sourceAccount.Currency.Code != destinationAccount.Currency.Code {
		if amountIsSource {
			destinationAmount, err = s.exchangeService.Convert(ctx, amount, sourceAccount.Currency.Code, destinationAccount.Currency.Code)
		} else {
			sourceAmount, err = s.exchangeService.Convert(ctx, amount, destinationAccount.Currency.Code, sourceAccount.Currency.Code)
		}
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to convert currencies: %v", err)
		}
	}

	transaction := &model.Transaction{
		PayerAccountNumber:     sourceAccount.AccountNumber,
		RecipientAccountNumber: destinationAccount.AccountNumber,
		StartAmount:            sourceAmount,
		StartCurrencyCode:      sourceAccount.Currency.Code,
		EndAmount:              destinationAmount,
		EndCurrencyCode:        destinationAccount.Currency.Code,
		Status:                 model.TransactionProcessing,
	}

	if err := s.transactionRepo.Create(ctx, transaction); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create transaction: %v", err)
	}

	if err := s.transactionProcessor.ProcessTradeSettlement(ctx, transaction.TransactionID); err != nil {
		return nil, mapTradeSettlementError(err)
	}

	return &pb.ExecuteTradeSettlementResponse{
		TransactionId:           uint64(transaction.TransactionID),
		SourceAmount:            sourceAmount,
		SourceCurrencyCode:      string(sourceAccount.Currency.Code),
		DestinationAmount:       destinationAmount,
		DestinationCurrencyCode: string(destinationAccount.Currency.Code),
	}, nil
}

func resolveBankAccountNumber(currencyCode string) (string, error) {
	accountNumber, ok := service.BankAccounts[model.CurrencyCode(currencyCode)]
	if !ok {
		return "", fmt.Errorf("unsupported trade currency: %s", currencyCode)
	}

	return accountNumber, nil
}

func mapTradeSettlementError(err error) error {
	var appErr *errors.AppError
	if stderrors.As(err, &appErr) {
		switch appErr.Code {
		case 400:
			return status.Error(codes.FailedPrecondition, appErr.Message)
		case 404:
			return status.Error(codes.NotFound, appErr.Message)
		default:
			return status.Error(codes.Internal, appErr.Message)
		}
	}

	return status.Error(codes.Internal, err.Error())
}

func (s *BankingService) CreateFundAccount(ctx context.Context, req *pb.CreateFundAccountRequest) (*pb.CreateFundAccountResponse, error) {
	fundName := strings.TrimSpace(req.GetFundName())
	if fundName == "" {
		return nil, status.Error(codes.InvalidArgument, "fund_name is required")
	}
	if req.GetManagerId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "manager_id is required")
	}

	bankAccount, err := s.accountRepo.FindByAccountType(ctx, model.AccountTypeBank)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to find bank account: %v", err)
	}
	if bankAccount == nil {
		return nil, status.Error(codes.NotFound, "no bank-owned account found")
	}

	expiresAt := time.Now().AddDate(100, 0, 0)

	account, err := s.accountService.Create(ctx, dto.CreateAccountRequest{
		Name:           fundName,
		ClientID:       bankAccount.ClientID,
		EmployeeID:     uint(req.GetManagerId()),
		AccountType:    model.AccountTypeFund,
		AccountKind:    model.AccountKindInternal,
		InitialBalance: 0,
		ExpiresAt:      expiresAt,
		GenerateCard:   false,
	})
	if err != nil {
		return nil, errors.MapGrpcToHttpError(err)
	}

	return &pb.CreateFundAccountResponse{
		AccountNumber: account.AccountNumber,
	}, nil
}
