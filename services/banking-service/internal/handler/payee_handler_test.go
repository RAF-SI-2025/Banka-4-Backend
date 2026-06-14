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

func payeeRouter(repo *fakePayeeRepo, clientID uint) *gin.Engine {
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

	h := NewPayeeHandler(service.NewPayeeService(repo))
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
	router := payeeRouter(repo, 7)

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
	router := payeeRouter(repo, 7)
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
	router := payeeRouter(repo, 7)

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
