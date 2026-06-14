package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	commonauth "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/service"
)

type fakePayeeRepo struct {
	payees  []model.Payee
	created *model.Payee
	updated *model.Payee
	deleted uint
}

func (f *fakePayeeRepo) FindAllByClientID(_ context.Context, clientID uint) ([]model.Payee, error) {
	out := make([]model.Payee, 0)
	for _, p := range f.payees {
		if p.ClientID == clientID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakePayeeRepo) FindByID(_ context.Context, id uint) (*model.Payee, error) {
	for i := range f.payees {
		if f.payees[i].PayeeID == id {
			return &f.payees[i], nil
		}
	}
	return nil, nil
}

func (f *fakePayeeRepo) Create(_ context.Context, payee *model.Payee) error {
	payee.PayeeID = 99
	f.created = payee
	return nil
}

func (f *fakePayeeRepo) Update(_ context.Context, payee *model.Payee) error {
	f.updated = payee
	return nil
}

func (f *fakePayeeRepo) Delete(_ context.Context, id uint) error {
	f.deleted = id
	return nil
}

type fakeAccountRepo struct {
	accounts []*model.Account
	created  *model.Account
	updated  *model.Account
}

func (f *fakeAccountRepo) Create(_ context.Context, account *model.Account) error {
	f.created = account
	f.accounts = append(f.accounts, account)
	return nil
}

func (f *fakeAccountRepo) AccountNumberExists(_ context.Context, accountNumber string) (bool, error) {
	for _, a := range f.accounts {
		if a.AccountNumber == accountNumber {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeAccountRepo) GetByAccountNumber(_ context.Context, accountNumber string) (*model.Account, error) {
	return f.FindByAccountNumber(context.Background(), accountNumber)
}

func (f *fakeAccountRepo) FindByAccountNumber(_ context.Context, accountNumber string) (*model.Account, error) {
	for _, a := range f.accounts {
		if a.AccountNumber == accountNumber {
			return a, nil
		}
	}
	return nil, nil
}

func (f *fakeAccountRepo) Update(_ context.Context, account *model.Account) error {
	f.updated = account
	return nil
}

func (f *fakeAccountRepo) UpdateBalance(_ context.Context, account *model.Account) error {
	f.updated = account
	return nil
}

func (f *fakeAccountRepo) FindAllByClientID(_ context.Context, clientID uint) ([]model.Account, error) {
	var result []model.Account
	for _, a := range f.accounts {
		if a.ClientID == clientID {
			result = append(result, *a)
		}
	}
	return result, nil
}

func (f *fakeAccountRepo) FindByClientID(_ context.Context, clientID uint) ([]model.Account, error) {
	return f.FindAllByClientID(context.Background(), clientID)
}

func (f *fakeAccountRepo) FindByAccountNumberAndClientID(_ context.Context, accountNumber string, clientID uint) (*model.Account, error) {
	for _, a := range f.accounts {
		if a.AccountNumber == accountNumber && a.ClientID == clientID {
			return a, nil
		}
	}
	return nil, nil
}

func (f *fakeAccountRepo) UpdateName(_ context.Context, accountNumber string, name string) error {
	for _, a := range f.accounts {
		if a.AccountNumber == accountNumber {
			a.Name = name
			return nil
		}
	}
	return nil
}

func (f *fakeAccountRepo) UpdateLimits(_ context.Context, accountNumber string, daily float64, monthly float64) error {
	for _, a := range f.accounts {
		if a.AccountNumber == accountNumber {
			a.DailyLimit = daily
			a.MonthlyLimit = monthly
			return nil
		}
	}
	return nil
}

func (f *fakeAccountRepo) NameExistsForClient(_ context.Context, clientID uint, name string, excludeNumber string) (bool, error) {
	for _, a := range f.accounts {
		if a.ClientID == clientID && a.Name == name && a.AccountNumber != excludeNumber {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeAccountRepo) FindAll(_ context.Context, _ *dto.ListAccountsQuery) ([]*model.Account, int64, error) {
	return f.accounts, int64(len(f.accounts)), nil
}

func (f *fakeAccountRepo) FindByAccountType(_ context.Context, accountType model.AccountType) (*model.Account, error) {
	for _, a := range f.accounts {
		if a.AccountType == accountType {
			return a, nil
		}
	}
	return nil, nil
}

func payeeRouter(repo *fakePayeeRepo, clientID uint, accountRepo *fakeAccountRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		commonauth.SetAuth(c, &commonauth.AuthContext{
			IdentityID:   10,
			IdentityType: commonauth.IdentityClient,
			ClientID:     &clientID,
		})
		c.Next()
	})

	h := NewPayeeHandler(service.NewPayeeService(repo, accountRepo))
	router.GET("/payees", h.GetAll)
	router.POST("/payees", h.Create)
	router.PATCH("/payees/:id", h.Update)
	router.DELETE("/payees/:id", h.Delete)
	return router
}

func TestPayeeHandlerGetAll(t *testing.T) {
	t.Parallel()

	repo := &fakePayeeRepo{payees: []model.Payee{
		{PayeeID: 1, ClientID: 7, Name: "Mine", AccountNumber: "444000100000000001"},
		{PayeeID: 2, ClientID: 8, Name: "Other", AccountNumber: "444000100000000002"},
	}}
	accountRepo := &fakeAccountRepo{
		accounts: []*model.Account{
			{AccountNumber: "444000100000000001", ClientID: 7},
			{AccountNumber: "444000100000000002", ClientID: 8},
		},
	}
	router := payeeRouter(repo, 7, accountRepo)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/payees", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body []dto.PayeeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(body) != 1 || body[0].PayeeID != 1 || body[0].Name != "Mine" {
		t.Fatalf("unexpected payees response %#v", body)
	}
}

func TestPayeeHandlerCreate(t *testing.T) {
	t.Parallel()

	repo := &fakePayeeRepo{}
	// Seed the account number that will be submitted in the request body
	accountRepo := &fakeAccountRepo{
		accounts: []*model.Account{
			{AccountNumber: "444000100000000003"},
		},
	}
	router := payeeRouter(repo, 7, accountRepo)
	body := bytes.NewBufferString(`{"name":"Landlord","account_number":"444000100000000003"}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/payees", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}
	if repo.created == nil {
		t.Fatal("expected payee to be created")
	}
	if repo.created.ClientID != 7 {
		t.Fatalf("created client id = %d, want 7", repo.created.ClientID)
	}
	if repo.created.Name != "Landlord" {
		t.Fatalf("created name = %q, want Landlord", repo.created.Name)
	}
}

func TestPayeeHandlerUpdateAndDelete(t *testing.T) {
	t.Parallel()

	repo := &fakePayeeRepo{payees: []model.Payee{
		{PayeeID: 5, ClientID: 7, Name: "Old", AccountNumber: "444000100000000004"},
	}}
	// Seed both the original and the new account number used in the update
	accountRepo := &fakeAccountRepo{
		accounts: []*model.Account{
			{AccountNumber: "444000100000000004", ClientID: 7},
			{AccountNumber: "444000100000000005", ClientID: 7},
		},
	}
	router := payeeRouter(repo, 7, accountRepo)

	updateBody := bytes.NewBufferString(`{"name":"New","account_number":"444000100000000005"}`)
	updateRec := httptest.NewRecorder()
	updateReq := httptest.NewRequest(http.MethodPatch, "/payees/5", updateBody)
	updateReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(updateRec, updateReq)

	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected update status %d, got %d", http.StatusOK, updateRec.Code)
	}
	if repo.updated == nil || repo.updated.Name != "New" || repo.updated.AccountNumber != "444000100000000005" {
		t.Fatalf("unexpected updated payee %#v", repo.updated)
	}

	deleteRec := httptest.NewRecorder()
	deleteReq := httptest.NewRequest(http.MethodDelete, "/payees/5", nil)
	router.ServeHTTP(deleteRec, deleteReq)

	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("expected delete status %d, got %d", http.StatusNoContent, deleteRec.Code)
	}
	if repo.deleted != 5 {
		t.Fatalf("deleted id = %d, want 5", repo.deleted)
	}
}
