package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	appErrors "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/repository"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/service"
	bankingvalidator "github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/validator"
)

type handlerUserClient struct{}

type handlerMailer struct {
	sentTo string
}

func (m *handlerMailer) Send(to, _, _ string) error {
	m.sentTo = to
	return nil
}

func (handlerUserClient) GetClientByID(context.Context, uint) (*pb.GetClientByIdResponse, error) {
	return &pb.GetClientByIdResponse{Id: 9, FullName: "Client", Email: "client@example.com"}, nil
}

func (handlerUserClient) GetEmployeeByID(context.Context, uint) (*pb.GetEmployeeByIdResponse, error) {
	return &pb.GetEmployeeByIdResponse{Id: 1, FullName: "Employee One", Email: "employee@example.com"}, nil
}

func setupBankingHandlerDB(t *testing.T, models ...interface{}) *gorm.DB {
	t.Helper()

	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func requestJSON(router http.Handler, method, path string, body interface{}) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, _ := json.Marshal(body)
		reader = bytes.NewReader(payload)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestPayeeHandlerLifecycle(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	db := setupBankingHandlerDB(t, &model.Payee{}, &model.Account{})

	// Seed the account number the payee will reference
	db.Create(&model.Account{AccountNumber: "444000100000000001"})

	payeeHandler := NewPayeeHandler(service.NewPayeeService(repository.NewPayeeRepository(db), repository.NewAccountRepository(db)))
	clientID := uint(42)
	router := gin.New()
	router.Use(appErrors.ErrorHandler())
	router.Use(func(c *gin.Context) {
		auth.SetAuth(c, &auth.AuthContext{IdentityID: 100, IdentityType: auth.IdentityClient, ClientID: &clientID})
		c.Next()
	})
	router.GET("/payees", payeeHandler.GetAll)
	router.POST("/payees", payeeHandler.Create)
	router.PATCH("/payees/:id", payeeHandler.Update)
	router.DELETE("/payees/:id", payeeHandler.Delete)

	rec := requestJSON(router, http.MethodPost, "/payees", gin.H{"name": "Supplier", "account_number": "444000100000000001"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rec.Code, rec.Body.String())
	}
	var created struct {
		PayeeID uint `json:"payee_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created payee: %v", err)
	}
	if created.PayeeID == 0 {
		t.Fatal("expected created payee id")
	}

	rec = requestJSON(router, http.MethodGet, "/payees", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get all status = %d body=%s", rec.Code, rec.Body.String())
	}
	var list []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode payee list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len(list) = %d, want 1", len(list))
	}

	rec = requestJSON(router, http.MethodPatch, "/payees/1", gin.H{"name": "Updated Supplier"})
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = requestJSON(router, http.MethodDelete, "/payees/1", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = requestJSON(router, http.MethodPatch, "/payees/bad", gin.H{"name": "Bad"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad id status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestExchangeHandlerRatesAndCalculate(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	db := setupBankingHandlerDB(t, &model.ExchangeRate{})
	now := time.Now().UTC()
	if err := db.Create(&model.ExchangeRate{
		CurrencyCode:         model.EUR,
		BaseCurrency:         model.RSD,
		BuyRate:              117,
		MiddleRate:           118,
		SellRate:             119,
		ProviderUpdatedAt:    now,
		ProviderNextUpdateAt: now.Add(time.Hour),
	}).Error; err != nil {
		t.Fatalf("create exchange rate: %v", err)
	}

	exchangeHandler := NewExchangeHandler(service.NewExchangeService(repository.NewExchangeRateRepository(db), nil))
	router := gin.New()
	router.Use(appErrors.ErrorHandler())
	router.GET("/rates", exchangeHandler.GetRates)
	router.GET("/calculate", exchangeHandler.Calculate)

	rec := requestJSON(router, http.MethodGet, "/rates", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("rates status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = requestJSON(router, http.MethodGet, "/calculate?amount=119&from_currency=RSD&to_currency=EUR", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("calculate status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = requestJSON(router, http.MethodGet, "/calculate?amount=-1&from_currency=RSD&to_currency=EUR", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("negative amount status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = requestJSON(router, http.MethodGet, "/calculate?amount=10&from_currency=BAD&to_currency=EUR", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad currency status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCompanyHandlerListsAndCreates(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	db := setupBankingHandlerDB(t, &model.Company{}, &model.WorkCode{})
	workCode := model.WorkCode{Code: "62.01", Description: "Software"}
	if err := db.Create(&workCode).Error; err != nil {
		t.Fatalf("create work code: %v", err)
	}
	companyHandler := NewCompanyHandler(service.NewCompanyService(repository.NewCompanyRepository(db), handlerUserClient{}, db, nil))
	router := gin.New()
	router.Use(appErrors.ErrorHandler())
	router.GET("/companies", companyHandler.GetCompanies)
	router.GET("/work-codes", companyHandler.GetWorkCodes)
	router.POST("/companies", companyHandler.Create)

	rec := requestJSON(router, http.MethodGet, "/work-codes", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("work codes status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = requestJSON(router, http.MethodPost, "/companies", gin.H{
		"name":                "Acme",
		"registration_number": "12345678",
		"tax_number":          "123456789",
		"work_code_id":        workCode.WorkCodeID,
		"address":             "Main",
		"owner_id":            9,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create company status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = requestJSON(router, http.MethodGet, "/companies", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("companies status = %d body=%s", rec.Code, rec.Body.String())
	}
	var companies []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &companies); err != nil {
		t.Fatalf("decode companies: %v", err)
	}
	if len(companies) != 1 {
		t.Fatalf("len(companies) = %d, want 1", len(companies))
	}

	rec = requestJSON(router, http.MethodPost, "/companies", gin.H{"name": "missing fields"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid company status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPaymentHandlerReadListAndReceipt(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	db := setupBankingHandlerDB(t,
		&model.Currency{},
		&model.Account{},
		&model.Transaction{},
		&model.Payment{},
	)
	rsd := model.Currency{Name: "Dinar", Code: model.RSD, Status: "Active"}
	if err := db.Create(&rsd).Error; err != nil {
		t.Fatalf("create currency: %v", err)
	}
	payer := model.Account{AccountNumber: "444000100000000101", Name: "Payer", ClientID: 55, EmployeeID: 1, CurrencyID: rsd.CurrencyID, Currency: rsd, Balance: 1000, AvailableBalance: 1000, Status: "Active", AccountType: model.AccountTypePersonal, AccountKind: model.AccountKindCurrent, Subtype: model.SubtypeStandard, ExpiresAt: time.Now().AddDate(1, 0, 0), DailyLimit: 1000, MonthlyLimit: 10000}
	recipient := model.Account{AccountNumber: "444000100000000102", Name: "Recipient", ClientID: 56, EmployeeID: 1, CurrencyID: rsd.CurrencyID, Currency: rsd, Balance: 1000, AvailableBalance: 1000, Status: "Active", AccountType: model.AccountTypePersonal, AccountKind: model.AccountKindCurrent, Subtype: model.SubtypeStandard, ExpiresAt: time.Now().AddDate(1, 0, 0), DailyLimit: 1000, MonthlyLimit: 10000}
	if err := db.Create(&[]model.Account{payer, recipient}).Error; err != nil {
		t.Fatalf("create accounts: %v", err)
	}
	tx := model.Transaction{
		PayerAccountNumber:     payer.AccountNumber,
		RecipientAccountNumber: recipient.AccountNumber,
		StartAmount:            250,
		StartCurrencyCode:      model.RSD,
		EndAmount:              250,
		EndCurrencyCode:        model.RSD,
		Status:                 model.TransactionCompleted,
		CreatedAt:              time.Now().UTC(),
	}
	if err := db.Create(&tx).Error; err != nil {
		t.Fatalf("create transaction: %v", err)
	}
	payment := model.Payment{TransactionID: tx.TransactionID, Transaction: tx, RecipientName: "Recipient", ReferenceNumber: "97", PaymentCode: "289", Purpose: "Invoice"}
	if err := db.Create(&payment).Error; err != nil {
		t.Fatalf("create payment: %v", err)
	}

	paymentService := service.NewPaymentService(
		repository.NewPaymentRepository(db),
		repository.NewTransactionRepository(db),
		repository.NewAccountRepository(db),
		nil,
		nil,
		repository.NewGormTransactionManager(db),
		nil,
		nil,
		nil,
	)
	h := NewPaymentHandler(paymentService, nil)
	router := gin.New()
	router.Use(appErrors.ErrorHandler())
	router.GET("/payments/:id", h.GetPaymentByID)
	router.GET("/payments/:id/receipt", h.GetReceipt)
	router.GET("/clients/:clientId/accounts/:accountNumber/payments", h.GetAccountPayments)
	router.GET("/clients/:clientId/payments", h.GetClientPayments)

	rec := requestJSON(router, http.MethodGet, "/payments/1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get payment status = %d body=%s", rec.Code, rec.Body.String())
	}
	var paymentResp struct {
		PaymentID uint    `json:"payment_id"`
		Amount    float64 `json:"amount"`
		Status    string  `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &paymentResp); err != nil {
		t.Fatalf("decode payment: %v", err)
	}
	if paymentResp.PaymentID != payment.PaymentID || paymentResp.Amount != 250 || paymentResp.Status != string(model.TransactionCompleted) {
		t.Fatalf("unexpected payment response %#v", paymentResp)
	}

	rec = requestJSON(router, http.MethodGet, "/payments/1/receipt", nil)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/pdf" || rec.Body.Len() == 0 {
		t.Fatalf("receipt status=%d content-type=%q len=%d", rec.Code, rec.Header().Get("Content-Type"), rec.Body.Len())
	}

	rec = requestJSON(router, http.MethodGet, "/clients/55/accounts/"+payer.AccountNumber+"/payments?page=1&page_size=10", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("account payments status = %d body=%s", rec.Code, rec.Body.String())
	}
	var listResp struct {
		Total      int64 `json:"total"`
		Page       int   `json:"page"`
		PageSize   int   `json:"page_size"`
		TotalPages int   `json:"total_pages"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode account payments: %v", err)
	}
	if listResp.Total != 1 || listResp.Page != 1 || listResp.PageSize != 10 || listResp.TotalPages != 1 {
		t.Fatalf("unexpected account payment list %#v", listResp)
	}

	rec = requestJSON(router, http.MethodGet, "/clients/55/payments?page=1&page_size=10&min_amount=100&max_amount=300", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("client payments status = %d body=%s", rec.Code, rec.Body.String())
	}
	rec = requestJSON(router, http.MethodGet, "/clients/55/accounts/"+payer.AccountNumber+"/payments?page=0&page_size=0", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid payment filters status = %d body=%s", rec.Code, rec.Body.String())
	}
	rec = requestJSON(router, http.MethodGet, "/payments/bad", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad payment id status = %d body=%s", rec.Code, rec.Body.String())
	}
	rec = requestJSON(router, http.MethodGet, "/clients/bad/payments", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad client id status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestTransferHandlerHistory(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	db := setupBankingHandlerDB(t, &model.Currency{}, &model.Account{}, &model.Transaction{}, &model.Transfer{})
	rsd := model.Currency{Name: "Dinar", Code: model.RSD, Status: "Active"}
	if err := db.Create(&rsd).Error; err != nil {
		t.Fatalf("create currency: %v", err)
	}
	account := model.Account{AccountNumber: "444000100000000201", Name: "Payer", ClientID: 55, EmployeeID: 1, CurrencyID: rsd.CurrencyID, Currency: rsd, Balance: 1000, AvailableBalance: 1000, Status: "Active", AccountType: model.AccountTypePersonal, AccountKind: model.AccountKindCurrent, Subtype: model.SubtypeStandard, ExpiresAt: time.Now().AddDate(1, 0, 0), DailyLimit: 1000, MonthlyLimit: 10000}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}
	tx := model.Transaction{
		PayerAccountNumber:     account.AccountNumber,
		RecipientAccountNumber: "444000100000000202",
		StartAmount:            75,
		StartCurrencyCode:      model.RSD,
		EndAmount:              75,
		EndCurrencyCode:        model.RSD,
		Status:                 model.TransactionCompleted,
		CreatedAt:              time.Now().UTC(),
	}
	if err := db.Create(&tx).Error; err != nil {
		t.Fatalf("create transaction: %v", err)
	}
	rate := 1.0
	if err := db.Create(&model.Transfer{TransactionID: tx.TransactionID, Transaction: tx, ExchangeRate: &rate}).Error; err != nil {
		t.Fatalf("create transfer: %v", err)
	}

	transferService := service.NewTransferService(
		repository.NewTransferRepository(db),
		repository.NewTransactionRepository(db),
		repository.NewAccountRepository(db),
		nil,
		repository.NewGormTransactionManager(db),
		nil,
		nil,
		nil,
	)
	h := NewTransferHandler(transferService)
	router := gin.New()
	router.Use(appErrors.ErrorHandler())
	router.GET("/clients/:clientId/transfers", h.GetTransferHistory)

	rec := requestJSON(router, http.MethodGet, "/clients/55/transfers?page=0&page_size=500", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("transfer history status = %d body=%s", rec.Code, rec.Body.String())
	}
	var history struct {
		Total    int64 `json:"total"`
		Page     int   `json:"page"`
		PageSize int   `json:"page_size"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &history); err != nil {
		t.Fatalf("decode transfer history: %v", err)
	}
	if history.Total != 1 || history.Page != 1 || history.PageSize != 10 {
		t.Fatalf("unexpected transfer history %#v", history)
	}

	rec = requestJSON(router, http.MethodGet, "/clients/bad/transfers", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad client id status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAccountHandlerRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	bankingvalidator.RegisterValidators()
	h := NewAccountHandler(nil)
	router := gin.New()
	router.Use(appErrors.ErrorHandler())
	router.POST("/accounts", h.Create)
	router.GET("/clients/:clientId/accounts", h.GetClientAccounts)
	router.GET("/clients/:clientId/accounts/:accountNumber", h.GetAccountDetails)
	router.PUT("/clients/:clientId/accounts/:accountNumber/name", h.UpdateAccountName)
	router.POST("/clients/:clientId/accounts/:accountNumber/limits/request", h.RequestLimitsChange)
	router.PUT("/clients/:clientId/accounts/:accountNumber/limits", h.ConfirmLimitsChange)

	cases := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{name: "create missing fields", method: http.MethodPost, path: "/accounts", body: gin.H{"name": "Primary"}},
		{name: "list bad client id", method: http.MethodGet, path: "/clients/bad/accounts"},
		{name: "details bad client id", method: http.MethodGet, path: "/clients/bad/accounts/444000100000000001"},
		{name: "name bad client id", method: http.MethodPut, path: "/clients/bad/accounts/444000100000000001/name", body: gin.H{"name": "Updated"}},
		{name: "name missing body", method: http.MethodPut, path: "/clients/55/accounts/444000100000000001/name", body: gin.H{}},
		{name: "limits bad values", method: http.MethodPost, path: "/clients/55/accounts/444000100000000001/limits/request", body: gin.H{"daily_limit": -1, "monthly_limit": 0}},
		{name: "limits bad code", method: http.MethodPut, path: "/clients/55/accounts/444000100000000001/limits", body: gin.H{"code": "123"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := requestJSON(router, tc.method, tc.path, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("%s %s status = %d body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestAccountHandlerReadsUpdatesAndRequestsLimits(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	db := setupBankingHandlerDB(t,
		&model.Currency{},
		&model.Company{},
		&model.Account{},
		&model.VerificationToken{},
	)
	rsd := model.Currency{Name: "Dinar", Code: model.RSD, Status: "Active"}
	if err := db.Create(&rsd).Error; err != nil {
		t.Fatalf("create currency: %v", err)
	}
	account := model.Account{
		AccountNumber:    "444000100000000301",
		Name:             "Primary",
		ClientID:         77,
		EmployeeID:       1,
		CurrencyID:       rsd.CurrencyID,
		Currency:         rsd,
		Balance:          1000,
		AvailableBalance: 900,
		Status:           "Active",
		AccountType:      model.AccountTypePersonal,
		AccountKind:      model.AccountKindCurrent,
		Subtype:          model.SubtypeStandard,
		ExpiresAt:        time.Now().AddDate(1, 0, 0),
		DailyLimit:       1000,
		MonthlyLimit:     10000,
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}

	accountService := service.NewAccountService(
		repository.NewAccountRepository(db),
		repository.NewCurrencyRepository(db),
		repository.NewVerificationTokenRepository(db),
		handlerUserClient{},
		nil,
		nil,
		nil,
		repository.NewGormTransactionManager(db),
		&handlerMailer{},
	)
	h := NewAccountHandler(accountService)
	router := gin.New()
	router.Use(appErrors.ErrorHandler())
	router.POST("/accounts", h.Create)
	router.GET("/accounts", h.ListAccounts)
	router.GET("/clients/:clientId/accounts", h.GetClientAccounts)
	router.GET("/clients/:clientId/accounts/:accountNumber", h.GetAccountDetails)
	router.PUT("/clients/:clientId/accounts/:accountNumber/name", h.UpdateAccountName)
	router.POST("/clients/:clientId/accounts/:accountNumber/limits/request", h.RequestLimitsChange)

	rec := requestJSON(router, http.MethodGet, "/accounts?page=1&page_size=10&status=Active", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list accounts status = %d body=%s", rec.Code, rec.Body.String())
	}
	var list struct {
		Total int64 `json:"total"`
		Data  []struct {
			AccountNumber string `json:"AccountNumber"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode accounts: %v", err)
	}
	if list.Total != 1 || len(list.Data) != 1 || list.Data[0].AccountNumber != account.AccountNumber {
		t.Fatalf("unexpected account list %#v", list)
	}

	rec = requestJSON(router, http.MethodGet, "/clients/77/accounts", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("client accounts status = %d body=%s", rec.Code, rec.Body.String())
	}
	var summaries []struct {
		AccountNumber string `json:"account_number"`
		Name          string `json:"name"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &summaries); err != nil {
		t.Fatalf("decode summaries: %v", err)
	}
	if len(summaries) != 1 || summaries[0].Name != "Primary" {
		t.Fatalf("unexpected summaries %#v", summaries)
	}

	rec = requestJSON(router, http.MethodGet, "/clients/77/accounts/"+account.AccountNumber, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("account details status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = requestJSON(router, http.MethodPut, "/clients/77/accounts/"+account.AccountNumber+"/name", gin.H{"name": "Savings"})
	if rec.Code != http.StatusOK {
		t.Fatalf("update account name status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = requestJSON(router, http.MethodPost, "/clients/77/accounts/"+account.AccountNumber+"/limits/request", gin.H{"daily_limit": 1500, "monthly_limit": 12000})
	if rec.Code != http.StatusOK {
		t.Fatalf("request limits status = %d body=%s", rec.Code, rec.Body.String())
	}
	var token model.VerificationToken
	if err := db.Where("account_number = ? AND client_id = ?", account.AccountNumber, 77).First(&token).Error; err != nil {
		t.Fatalf("find verification token: %v", err)
	}
	if token.NewDailyLimit != 1500 || token.NewMonthlyLimit != 12000 {
		t.Fatalf("unexpected token %#v", token)
	}

	rec = requestJSON(router, http.MethodPost, "/accounts", gin.H{
		"name":            "Created",
		"client_id":       77,
		"employee_id":     1,
		"account_type":    string(model.AccountTypePersonal),
		"account_kind":    string(model.AccountKindCurrent),
		"subtype":         string(model.SubtypeStandard),
		"initial_balance": 250,
		"expires_at":      time.Now().AddDate(1, 0, 0),
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create account status = %d body=%s", rec.Code, rec.Body.String())
	}
	var created struct {
		AccountNumber string `json:"account_number"`
		Name          string `json:"name"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created account: %v", err)
	}
	if created.AccountNumber == "" || created.Name != "Created" {
		t.Fatalf("unexpected created account %#v", created)
	}
}

func TestCardAndLoanHandlersRejectInvalidRequests(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	cardHandler := NewCardHandler(nil)
	loanHandler := NewLoanHandler(nil)
	router := gin.New()
	router.Use(appErrors.ErrorHandler())
	router.POST("/cards/request", cardHandler.RequestCard)
	router.POST("/cards/request/confirm", cardHandler.ConfirmCardRequest)
	router.PUT("/cards/:cardId/block", cardHandler.BlockCard)
	router.PUT("/cards/:cardId/unblock", cardHandler.UnblockCard)
	router.PUT("/cards/:cardId/deactivate", cardHandler.DeactivateCard)
	router.POST("/clients/:clientId/loans/request", loanHandler.SubmitLoanRequest)
	router.GET("/clients/:clientId/loans", loanHandler.GetLoans)
	router.GET("/clients/:clientId/loans/:loanId", loanHandler.GetLoanByID)
	router.PATCH("/loan-requests/:id/approve", loanHandler.ApproveLoanRequest)
	router.PATCH("/loan-requests/:id/reject", loanHandler.RejectLoanRequest)

	cases := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{name: "request card missing account", method: http.MethodPost, path: "/cards/request", body: gin.H{}},
		{name: "confirm card missing code", method: http.MethodPost, path: "/cards/request/confirm", body: gin.H{"account_number": "444000100000000001"}},
		{name: "block bad card id", method: http.MethodPut, path: "/cards/bad/block"},
		{name: "unblock bad card id", method: http.MethodPut, path: "/cards/bad/unblock"},
		{name: "deactivate bad card id", method: http.MethodPut, path: "/cards/bad/deactivate"},
		{name: "submit loan missing fields", method: http.MethodPost, path: "/clients/77/loans/request", body: gin.H{}},
		{name: "list loans bad client", method: http.MethodGet, path: "/clients/bad/loans"},
		{name: "loan details bad client", method: http.MethodGet, path: "/clients/bad/loans/1"},
		{name: "loan details bad loan", method: http.MethodGet, path: "/clients/77/loans/bad"},
		{name: "approve bad request id", method: http.MethodPatch, path: "/loan-requests/bad/approve"},
		{name: "reject bad request id", method: http.MethodPatch, path: "/loan-requests/bad/reject"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := requestJSON(router, tc.method, tc.path, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("%s %s status = %d body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
			}
		})
	}
}
