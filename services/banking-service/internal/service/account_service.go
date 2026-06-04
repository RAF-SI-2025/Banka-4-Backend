package service

import (
	"context"
	"fmt"
	mathrand "math/rand"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/repository"
)

type AccountService struct {
	repo               repository.AccountRepository
	currencyRepo       repository.CurrencyRepository
	verificationRepo   repository.VerificationTokenRepository
	userClient         client.UserClient
	cardService        *CardService
	mobileSecretClient client.MobileSecretClient
	exchangeService    CurrencyConverter
	txManager          repository.TransactionManager
	mailer             Mailer
}

func NewAccountService(
	repo repository.AccountRepository,
	currencyRepo repository.CurrencyRepository,
	verificationRepo repository.VerificationTokenRepository,
	userClient client.UserClient,
	cardService *CardService,
	mobileSecretClient client.MobileSecretClient,
	exchangeService CurrencyConverter,
	txManager repository.TransactionManager,
	mailer Mailer,
) *AccountService {
	return &AccountService{
		repo:               repo,
		currencyRepo:       currencyRepo,
		verificationRepo:   verificationRepo,
		userClient:         userClient,
		cardService:        cardService,
		mobileSecretClient: mobileSecretClient,
		exchangeService:    exchangeService,
		txManager:          txManager,
		mailer:             mailer,
	}
}

func (s *AccountService) generateAccountNumber(typeCode string) string {
	random := fmt.Sprintf("%09d", mathrand.Intn(1_000_000_000))
	return model.BankCode + model.BranchCode + random + typeCode
}

func (s *AccountService) isValidAccountNumber(ctx context.Context, number string) bool {
	exists, _ := s.repo.AccountNumberExists(ctx, number)
	if exists {
		return false
	}

	sum := 0
	for _, ch := range number {
		sum += int(ch - '0')
	}
	return sum%11 != 0
}

func (s *AccountService) generateValidAccountNumber(ctx context.Context, accountKind model.AccountKind, accountType model.AccountType, subtype model.Subtype) string {
	typeCode := model.GetTypeCode(accountKind, accountType, subtype)
	for {
		number := s.generateAccountNumber(typeCode)
		if s.isValidAccountNumber(ctx, number) {
			return number
		}
	}
}

func (s *AccountService) Create(ctx context.Context, req dto.CreateAccountRequest) (*model.Account, error) {
	if _, err := s.userClient.GetClientByID(ctx, req.ClientID); err != nil {
		return nil, errors.NotFoundErr("client not found")
	}

	if _, err := s.userClient.GetEmployeeByID(ctx, req.EmployeeID); err != nil {
		return nil, errors.NotFoundErr("employee not found")
	}

	if req.AccountType == model.AccountTypeBusiness && req.CompanyID == nil {
		return nil, errors.BadRequestErr("business account requires a company")
	}

	if req.AccountType == model.AccountTypePersonal && req.CompanyID != nil {
		return nil, errors.BadRequestErr("personal account cannot have a company")
	}

	currencyCode := model.RSD
	if req.AccountKind == model.AccountKindForeign {
		if req.CurrencyCode == "" {
			return nil, errors.BadRequestErr("currency code is required for foreign accounts")
		}

		currencyCode = req.CurrencyCode
	}

	if req.AccountKind == model.AccountKindCurrent && req.Subtype == "" {
		return nil, errors.BadRequestErr("subtype is required for current accounts")
	}

	exists, err := s.repo.NameExistsForClient(ctx, req.ClientID, req.Name, "")
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	if exists {
		return nil, errors.ConflictErr("account with this name already exists")
	}

	currency, err := s.currencyRepo.FindByCode(ctx, currencyCode)
	if err != nil {
		return nil, err
	}

	dailyLimit := model.DefaultDailyLimitRSD
	monthlyLimit := model.DefaultMonthlyLimitRSD
	if req.AccountKind == model.AccountKindForeign {
		convertedDaily, err := s.exchangeService.Convert(ctx, model.DefaultDailyLimitRSD, model.RSD, currencyCode)
		if err != nil {
			return nil, err
		}
		convertedMonthly, err := s.exchangeService.Convert(ctx, model.DefaultMonthlyLimitRSD, model.RSD, currencyCode)
		if err != nil {
			return nil, err
		}
		dailyLimit = convertedDaily
		monthlyLimit = convertedMonthly
	}

	account := &model.Account{
		AccountNumber:    s.generateValidAccountNumber(ctx, req.AccountKind, req.AccountType, req.Subtype),
		Name:             req.Name,
		ClientID:         req.ClientID,
		EmployeeID:       req.EmployeeID,
		CompanyID:        req.CompanyID,
		Balance:          req.InitialBalance,
		AvailableBalance: req.InitialBalance,
		ExpiresAt:        req.ExpiresAt,
		CurrencyID:       currency.CurrencyID,
		AccountType:      req.AccountType,
		AccountKind:      req.AccountKind,
		Subtype:          req.Subtype,
		DailyLimit:       dailyLimit,
		MonthlyLimit:     monthlyLimit,
	}

	if req.GenerateCard {
		if err := s.txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
			if err := s.repo.Create(txCtx, account); err != nil {
				return errors.InternalErr(err)
			}

			if _, err := s.cardService.createCard(txCtx, account, nil); err != nil {
				return err
			}

			return nil
		}); err != nil {
			return nil, err
		}

		if err := s.sendAccountCreationEmail(ctx, account); err != nil {
			return nil, err
		}

		return account, nil
	}

	if err := s.repo.Create(ctx, account); err != nil {
		return nil, err
	}

	if err := s.sendAccountCreationEmail(ctx, account); err != nil {
		return nil, errors.InternalErr(err)
	}

	return account, nil
}

func (s *AccountService) sendAccountCreationEmail(ctx context.Context, account *model.Account) error {
	client, err := s.userClient.GetClientByID(ctx, account.ClientID)
	if err != nil {
		return err
	}

	body := fmt.Sprintf(
		"Hello %s,\n\nA new %s account has been created successfully.\n\n"+
			"Account name: %s\nAccount number: %s\nAccount type: %s\nAccount subtype: %s\nCurrency: %s",
		defaultContactName(client.FullName),
		account.AccountKind,
		account.Name,
		account.AccountNumber,
		account.AccountType,
		account.Subtype,
		account.Currency.Code,
	)

	if err := s.mailer.Send(client.Email, "Card created successfully", body); err != nil {
		return errors.ServiceUnavailableErr(err)
	}

	return nil
}

func (s *AccountService) GetAllAccounts(ctx context.Context, query *dto.ListAccountsQuery) ([]*model.Account, int64, error) {
	return s.repo.FindAll(ctx, query)
}

func (s *AccountService) GetClientAccounts(ctx context.Context, clientID uint) ([]model.Account, error) {
	accounts, err := s.repo.FindAllByClientID(ctx, clientID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	return accounts, nil
}

func (s *AccountService) GetAccountDetails(ctx context.Context, accountNumber string, clientID uint) (*model.Account, error) {
	account, err := s.repo.FindByAccountNumberAndClientID(ctx, accountNumber, clientID)
	if err != nil {
		return nil, errors.NotFoundErr("account not found")
	}
	return account, nil
}

func (s *AccountService) UpdateAccountName(ctx context.Context, accountNumber string, clientID uint, name string) error {
	account, err := s.repo.FindByAccountNumberAndClientID(ctx, accountNumber, clientID)
	if err != nil {
		return errors.NotFoundErr("account not found")
	}

	if account.Name == name {
		return errors.BadRequestErr("new name is the same as the current name")
	}

	exists, err := s.repo.NameExistsForClient(ctx, clientID, name, accountNumber)
	if err != nil {
		return errors.InternalErr(err)
	}
	if exists {
		return errors.ConflictErr("an account with this name already exists")
	}

	if err := s.repo.UpdateName(ctx, accountNumber, name); err != nil {
		return errors.InternalErr(err)
	}
	return nil
}

func (s *AccountService) RequestLimitsChange(ctx context.Context, accountNumber string, clientID uint, daily float64, monthly float64) error {
	if _, err := s.repo.FindByAccountNumberAndClientID(ctx, accountNumber, clientID); err != nil {
		return errors.NotFoundErr("account not found")
	}

	token := &model.VerificationToken{
		ClientID:        clientID,
		AccountNumber:   accountNumber,
		NewDailyLimit:   daily,
		NewMonthlyLimit: monthly,
	}
	return s.txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.verificationRepo.DeleteByAccountAndClient(txCtx, accountNumber, clientID); err != nil {
			return errors.InternalErr(err)
		}
		if err := s.verificationRepo.Create(txCtx, token); err != nil {
			return errors.InternalErr(err)
		}

		return nil
	})
}

func (s *AccountService) ConfirmLimitsChange(ctx context.Context, accountNumber string, clientID uint, code, authorizationHeader string) error {

	token, err := s.verificationRepo.FindByAccountAndClient(ctx, accountNumber, clientID)
	if err != nil {
		return errors.NotFoundErr("no pending limits change for this account")
	}

	secret, err := s.mobileSecretClient.GetMobileSecret(ctx, authorizationHeader)
	if err != nil {
		return errors.ServiceUnavailableErr(err)
	}

	if !verifyTOTPCode(secret, code, time.Now(), totpAllowedSkew) {
		return errors.BadRequestErr("invalid verification code")
	}

	if err := s.txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.repo.UpdateLimits(txCtx, accountNumber, token.NewDailyLimit, token.NewMonthlyLimit); err != nil {
			return errors.InternalErr(err)
		}
		if err := s.verificationRepo.DeleteByAccountAndClient(txCtx, accountNumber, clientID); err != nil {
			return errors.InternalErr(err)
		}
		return nil
	}); err != nil {
		return err
	}

	return s.sendLimitsChangedEmail(ctx, accountNumber, clientID, token.NewDailyLimit, token.NewMonthlyLimit)
}

func (s *AccountService) sendLimitsChangedEmail(ctx context.Context, accountNumber string, clientID uint, daily, monthly float64) error {
	clientInfo, err := s.userClient.GetClientByID(ctx, clientID)
	if err != nil {
		return err
	}
	if clientInfo == nil {
		return errors.InternalErr(fmt.Errorf("client not found in user service"))
	}

	body := fmt.Sprintf(
		"Hello %s,\n\nThe limits for your account %s have been updated.\n\nNew daily limit: %.2f\nNew monthly limit: %.2f",
		defaultContactName(clientInfo.FullName),
		accountNumber,
		daily,
		monthly,
	)

	if err := s.mailer.Send(clientInfo.Email, "Account limits changed", body); err != nil {
		return errors.ServiceUnavailableErr(err)
	}

	return nil
}

// ...existing code...
