//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	commonjwt "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/jwt"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/logging"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/permission"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/config"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/handler"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/repository"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/server"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/service"

	"github.com/gin-gonic/gin"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var sharedDB *gorm.DB
var uniqueCounter atomic.Uint64

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	_ = logging.Init("test")

	ctx := context.Background()
	container, err := tcpostgres.Run(
		ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("banking_service_test"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		log.Fatalf("start postgres container: %v", err)
	}

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("build postgres connection string: %v", err)
	}

	db, err := gorm.Open(gormpostgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("open gorm db: %v", err)
	}

	if err := db.AutoMigrate(
		&model.Currency{},
		&model.WorkCode{},
		&model.Company{},
		&model.Account{},
		&model.AuthorizedPerson{},
		&model.Card{},
		&model.CardRequest{},
		&model.Transaction{},
		&model.Payment{},
		&model.Transfer{},
		&model.Payee{},
		&model.LoanType{},
		&model.LoanRequest{},
		&model.Loan{},
		&model.LoanInstallment{},
		&model.VerificationToken{},
		&model.ExchangeRate{},
	); err != nil {
		log.Fatalf("auto migrate test schema: %v", err)
	}

	sharedDB = db
	code := m.Run()

	sqlDB, _ := db.DB()
	_ = sqlDB.Close()
	_ = container.Terminate(ctx)

	os.Exit(code)
}

type fakeMailer struct{}

func (f *fakeMailer) Send(_, _, _ string) error {
	return nil
}

type fakeUserClient struct{}

func (f *fakeUserClient) GetClientByID(_ context.Context, id uint) (*pb.GetClientByIdResponse, error) {
	return &pb.GetClientByIdResponse{
		Id:       uint64(id),
		Email:    fmt.Sprintf("client-%d@example.com", id),
		FullName: fmt.Sprintf("Client %d", id),
	}, nil
}

func (f *fakeUserClient) GetEmployeeByID(_ context.Context, id uint) (*pb.GetEmployeeByIdResponse, error) {
	return &pb.GetEmployeeByIdResponse{
		Id:       uint64(id),
		Email:    fmt.Sprintf("employee-%d@example.com", id),
		FullName: fmt.Sprintf("Employee %d", id),
	}, nil
}

const testTOTPSecret = "JBSWY3DPEHPK3PXP"

type fakeMobileSecretClient struct{}

func (f *fakeMobileSecretClient) GetMobileSecret(_ context.Context, _ string) (string, error) {
	return testTOTPSecret, nil
}

type fakePermissionProvider struct{}

func (f *fakePermissionProvider) GetPermissions(_ context.Context, _ *commonjwt.Claims) ([]permission.Permission, error) {
	return nil, nil
}

type fakeCurrencyConverter struct{}

func (f *fakeCurrencyConverter) Convert(_ context.Context, amount float64, from, to model.CurrencyCode) (float64, error) {
	if from == to {
		return amount, nil
	}
	return amount * 0.0085, nil
}

func (f *fakeCurrencyConverter) CalculateFee(amount float64) float64 {
	if amount <= 0 {
		return 0
	}
	return amount * model.BankCommission
}

func testConfig() *config.Configuration {
	return &config.Configuration{
		Env:       "test",
		JWTSecret: "test-secret",
		URLs: config.URLConfig{
			FrontendBaseURL: "http://localhost:5173",
		},
	}
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	tx := sharedDB.Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}

	t.Cleanup(func() {
		tx.Rollback()
	})

	return tx
}

func setupTestRouter(t *testing.T, db *gorm.DB) *gin.Engine {
	t.Helper()

	cfg := testConfig()

	var userCl client.UserClient = &fakeUserClient{}
	var mobileSecretCl client.MobileSecretClient = &fakeMobileSecretClient{}
	var mailer service.Mailer = &fakeMailer{}
	var converter service.CurrencyConverter = &fakeCurrencyConverter{}
	var permProvider auth.PermissionProvider = &fakePermissionProvider{}

	accountRepo := repository.NewAccountRepository(db)
	currencyRepo := repository.NewCurrencyRepository(db)
	verificationRepo := repository.NewVerificationTokenRepository(db)
	companyRepo := repository.NewCompanyRepository(db)
	payeeRepo := repository.NewPayeeRepository(db)
	cardRepo := repository.NewCardRepository(db)
	cardRequestRepo := repository.NewCardRequestRepository(db)
	authorizedPersonRepo := repository.NewAuthorizedPersonRepository(db)
	paymentRepo := repository.NewPaymentRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	transferRepo := repository.NewTransferRepository(db)
	loanRepo := repository.NewLoanRepository(db)
	loanRequestRepo := repository.NewLoanRequestRepository(db)
	loanTypeRepo := repository.NewLoanTypeRepository(db)
	exchangeRateRepo := repository.NewExchangeRateRepository(db)
	txManager := repository.NewGormTransactionManager(db)

	cardSvc := service.NewCardService(accountRepo, cardRepo, authorizedPersonRepo, cardRequestRepo, userCl, mailer, txManager)
	accountSvc := service.NewAccountService(accountRepo, currencyRepo, verificationRepo, userCl, cardSvc, mobileSecretCl, converter, txManager, mailer)
	companySvc := service.NewCompanyService(companyRepo, userCl, db, nil)
	payeeSvc := service.NewPayeeService(payeeRepo)
	exchangeSvc := service.NewExchangeService(exchangeRateRepo, nil)
	transactionProcessor := service.NewTransactionProcessor(accountRepo, transactionRepo, txManager)
	paymentSvc := service.NewPaymentService(paymentRepo, transactionRepo, accountRepo, mobileSecretCl, converter, txManager, transactionProcessor)
	transferSvc := service.NewTransferService(transferRepo, transactionRepo, accountRepo, converter, txManager, transactionProcessor)
	loanSvc := service.NewLoanService(accountRepo, loanTypeRepo, loanRequestRepo, loanRepo, transactionProcessor, txManager, userCl, mailer)

	healthHandler := handler.NewHealthHandler()
	accountHandler := handler.NewAccountHandler(accountSvc)
	companyHandler := handler.NewCompanyHandler(companySvc)
	payeeHandler := handler.NewPayeeHandler(payeeSvc)
	exchangeHandler := handler.NewExchangeHandler(exchangeSvc)
	paymentHandler := handler.NewPaymentHandler(paymentSvc, accountSvc)
	cardHandler := handler.NewCardHandler(cardSvc)
	loanHandler := handler.NewLoanHandler(loanSvc)
	transferHandler := handler.NewTransferHandler(transferSvc)

	verifier := auth.TokenVerifier(commonjwt.NewJWTVerifier(cfg.JWTSecret))

	r := gin.New()
	server.InitRouter(r, cfg)
	server.SetupRoutes(
		r,
		healthHandler,
		accountHandler,
		companyHandler,
		transferHandler,
		payeeHandler,
		exchangeHandler,
		paymentHandler,
		cardHandler,
		loanHandler,
		verifier,
		permProvider,
	)

	return r
}

func seedCurrency(t *testing.T, db *gorm.DB, code model.CurrencyCode) *model.Currency {
	t.Helper()

	currency := &model.Currency{
		Name:   string(code),
		Code:   code,
		Symbol: string(code),
		Status: "Active",
	}

	if err := db.Create(currency).Error; err != nil {
		t.Fatalf("seed currency: %v", err)
	}

	return currency
}

func seedWorkCode(t *testing.T, db *gorm.DB) *model.WorkCode {
	t.Helper()

	wc := &model.WorkCode{
		Code:        fmt.Sprintf("WC%d", uniqueCounter.Add(1)%10000000),
		Description: "Test work code",
	}

	if err := db.Create(wc).Error; err != nil {
		t.Fatalf("seed work code: %v", err)
	}

	return wc
}

func seedCompany(t *testing.T, db *gorm.DB, ownerID uint, workCodeID uint) *model.Company {
	t.Helper()

	company := &model.Company{
		Name:               uniqueValue(t, "company"),
		RegistrationNumber: fmt.Sprintf("%08d", uniqueCounter.Add(1)%100000000),
		TaxNumber:          fmt.Sprintf("%09d", uniqueCounter.Add(1)%1000000000),
		WorkCodeID:         workCodeID,
		Address:            "Test Address",
		OwnerID:            ownerID,
	}

	if err := db.Create(company).Error; err != nil {
		t.Fatalf("seed company: %v", err)
	}

	return company
}

func seedAccount(t *testing.T, db *gorm.DB, clientID uint, currencyID uint, balance float64) *model.Account {
	t.Helper()

	accountNumber := fmt.Sprintf("444000100%09d", uniqueCounter.Add(1))

	account := &model.Account{
		AccountNumber:    accountNumber,
		Name:             uniqueValue(t, "account"),
		ClientID:         clientID,
		EmployeeID:       1,
		CurrencyID:       currencyID,
		Balance:          balance,
		AvailableBalance: balance,
		ExpiresAt:        time.Now().AddDate(5, 0, 0),
		Status:           "Active",
		AccountType:      model.AccountTypePersonal,
		AccountKind:      model.AccountKindCurrent,
		Subtype:          model.SubtypeStandard,
		DailyLimit:       model.DefaultDailyLimitRSD,
		MonthlyLimit:     model.DefaultMonthlyLimitRSD,
	}

	if err := db.Create(account).Error; err != nil {
		t.Fatalf("seed account: %v", err)
	}

	if err := db.Preload("Currency").First(account, "account_number = ?", account.AccountNumber).Error; err != nil {
		t.Fatalf("reload account with currency: %v", err)
	}

	return account
}

func seedBusinessAccount(t *testing.T, db *gorm.DB, clientID uint, companyID uint, currencyID uint, balance float64) *model.Account {
	t.Helper()

	accountNumber := fmt.Sprintf("444000200%09d", uniqueCounter.Add(1))

	account := &model.Account{
		AccountNumber:    accountNumber,
		Name:             uniqueValue(t, "bizaccount"),
		ClientID:         clientID,
		EmployeeID:       1,
		CompanyID:        &companyID,
		CurrencyID:       currencyID,
		Balance:          balance,
		AvailableBalance: balance,
		ExpiresAt:        time.Now().AddDate(5, 0, 0),
		Status:           "Active",
		AccountType:      model.AccountTypeBusiness,
		AccountKind:      model.AccountKindCurrent,
		Subtype:          model.SubtypeLLC,
		DailyLimit:       model.DefaultDailyLimitRSD,
		MonthlyLimit:     model.DefaultMonthlyLimitRSD,
	}

	if err := db.Create(account).Error; err != nil {
		t.Fatalf("seed business account: %v", err)
	}

	if err := db.Preload("Currency").First(account, "account_number = ?", account.AccountNumber).Error; err != nil {
		t.Fatalf("reload account with currency: %v", err)
	}

	return account
}

func seedCard(t *testing.T, db *gorm.DB, accountNumber string) *model.Card {
	t.Helper()

	card := &model.Card{
		CardNumber:    fmt.Sprintf("4532750000%06d", uniqueCounter.Add(1)),
		CardType:      model.CardTypeDebit,
		CardBrand:     model.CardBrandVisa,
		Name:          "Visa Debit",
		AccountNumber: accountNumber,
		CVV:           "123",
		Limit:         model.DefaultMonthlyLimitRSD,
		Status:        model.CardStatusActive,
		ExpiresAt:     time.Now().AddDate(4, 0, 0),
	}

	if err := db.Create(card).Error; err != nil {
		t.Fatalf("seed card: %v", err)
	}

	return card
}

func seedLoanType(t *testing.T, db *gorm.DB) *model.LoanType {
	t.Helper()

	lt := &model.LoanType{
		Name:               uniqueValue(t, "loantype"),
		Description:        "Test loan type",
		BankMargin:         2.5,
		BaseInterestRate:   3.0,
		MinRepaymentPeriod: 12,
		MaxRepaymentPeriod: 360,
	}

	if err := db.Create(lt).Error; err != nil {
		t.Fatalf("seed loan type: %v", err)
	}

	return lt
}

func seedExchangeRate(t *testing.T, db *gorm.DB, code model.CurrencyCode, buyRate, middleRate, sellRate float64) {
	t.Helper()

	rate := &model.ExchangeRate{
		CurrencyCode:         code,
		BaseCurrency:         model.RSD,
		BuyRate:              buyRate,
		MiddleRate:           middleRate,
		SellRate:             sellRate,
		ProviderUpdatedAt:    time.Now(),
		ProviderNextUpdateAt: time.Now().Add(2 * time.Hour),
	}

	if err := db.Save(rate).Error; err != nil {
		t.Fatalf("seed exchange rate: %v", err)
	}
}

func seedBankAccounts(t *testing.T, db *gorm.DB, rsdCurrencyID uint) {
	t.Helper()

	for code, accNum := range service.BankAccounts {
		currencyID := rsdCurrencyID
		var currency model.Currency
		if err := db.Where("code = ?", string(code)).First(&currency).Error; err == nil {
			currencyID = currency.CurrencyID
		}

		bankAccount := &model.Account{
			AccountNumber:    accNum,
			Name:             fmt.Sprintf("Bank-%s", string(code)),
			ClientID:         0,
			EmployeeID:       0,
			CurrencyID:       currencyID,
			Balance:          10_000_000,
			AvailableBalance: 10_000_000,
			ExpiresAt:        time.Now().AddDate(100, 0, 0),
			Status:           "Active",
			AccountType:      model.AccountTypeBank,
			AccountKind:      model.AccountKindInternal,
			DailyLimit:       1_000_000_000,
			MonthlyLimit:     10_000_000_000,
		}

		if err := db.Create(bankAccount).Error; err != nil {
			t.Fatalf("seed bank account %s: %v", accNum, err)
		}
	}
}

func authHeaderForClient(t *testing.T, identityID uint, clientID uint) string {
	t.Helper()

	cid := clientID
	token, err := commonjwt.GenerateToken(&commonjwt.Claims{
		IdentityID:   identityID,
		IdentityType: string(auth.IdentityClient),
		ClientID:     &cid,
	}, testConfig().JWTSecret, 15)
	if err != nil {
		t.Fatalf("generate client auth token: %v", err)
	}

	return "Bearer " + token
}

func authHeaderForEmployee(t *testing.T, identityID uint, employeeID uint) string {
	t.Helper()

	eid := employeeID
	token, err := commonjwt.GenerateToken(&commonjwt.Claims{
		IdentityID:   identityID,
		IdentityType: string(auth.IdentityEmployee),
		EmployeeID:   &eid,
	}, testConfig().JWTSecret, 15)
	if err != nil {
		t.Fatalf("generate employee auth token: %v", err)
	}

	return "Bearer " + token
}

func uniqueValue(t *testing.T, prefix string) string {
	t.Helper()
	name := strings.NewReplacer("/", "-", " ", "-", ":", "-").Replace(strings.ToLower(t.Name()))
	return fmt.Sprintf("%s-%s-%d-%d", prefix, name, time.Now().UnixNano(), uniqueCounter.Add(1))
}

func performRequest(t *testing.T, router *gin.Engine, method, path string, body any, authorization string) *httptest.ResponseRecorder {
	t.Helper()

	var bodyReader *bytes.Reader
	if body == nil {
		bodyReader = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}

		bodyReader = bytes.NewReader(payload)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func decodeResponse[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()

	var response T
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response body: %v; body=%s", err, recorder.Body.String())
	}

	return response
}

func requireStatus(t *testing.T, recorder *httptest.ResponseRecorder, expected int) {
	t.Helper()

	if recorder.Code != expected {
		t.Fatalf("expected status %d, got %d, body=%s", expected, recorder.Code, recorder.Body.String())
	}
}

type appErrorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func generateTOTPCode(t *testing.T) string {
	t.Helper()

	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(testTOTPSecret)
	if err != nil {
		t.Fatalf("decode totp secret: %v", err)
	}

	counter := time.Now().Unix() / 30
	var message [8]byte
	binary.BigEndian.PutUint64(message[:], uint64(counter))

	h := hmac.New(sha1.New, secret)
	_, _ = h.Write(message[:])
	hash := h.Sum(nil)

	offset := hash[len(hash)-1] & 0x0f
	binaryCode := (int(hash[offset])&0x7f)<<24 |
		(int(hash[offset+1])&0xff)<<16 |
		(int(hash[offset+2])&0xff)<<8 |
		(int(hash[offset+3]) & 0xff)
	otp := binaryCode % 1000000

	return fmt.Sprintf("%06d", otp)
}
