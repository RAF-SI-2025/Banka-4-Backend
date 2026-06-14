package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	commonauth "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	commonerrors "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/model"
)

type supervisorEmployeeRepo struct {
	byID         map[uint]*model.Employee
	byIdentityID map[uint]*model.Employee
}

func (r *supervisorEmployeeRepo) Create(_ context.Context, _ *model.Employee) error {
	return nil
}

func (r *supervisorEmployeeRepo) FindByID(_ context.Context, id uint) (*model.Employee, error) {
	return r.byID[id], nil
}

func (r *supervisorEmployeeRepo) FindByIdentityID(_ context.Context, identityID uint) (*model.Employee, error) {
	return r.byIdentityID[identityID], nil
}

func (r *supervisorEmployeeRepo) Update(_ context.Context, _ *model.Employee) error {
	return nil
}

func (r *supervisorEmployeeRepo) GetAll(_ context.Context, _, _, _, _ string, _, _ int) ([]model.Employee, int64, error) {
	return nil, 0, nil
}

func supervisorRouter(repo *supervisorEmployeeRepo, ac *commonauth.AuthContext) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(commonerrors.ErrorHandler())
	if ac != nil {
		router.Use(func(c *gin.Context) {
			commonauth.SetAuth(c, ac)
			c.Next()
		})
	}
	router.GET("/supervisor", RequireSupervisor(repo), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	return router
}

func TestRequireSupervisorAllowsSupervisorByEmployeeID(t *testing.T) {
	t.Parallel()

	employeeID := uint(7)
	repo := &supervisorEmployeeRepo{byID: map[uint]*model.Employee{
		employeeID: {EmployeeID: employeeID, ActuaryInfo: &model.ActuaryInfo{IsSupervisor: true}},
	}}
	router := supervisorRouter(repo, &commonauth.AuthContext{
		IdentityID:   100,
		IdentityType: commonauth.IdentityEmployee,
		EmployeeID:   &employeeID,
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/supervisor", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
}

func TestRequireSupervisorFallsBackToIdentityID(t *testing.T) {
	t.Parallel()

	repo := &supervisorEmployeeRepo{byIdentityID: map[uint]*model.Employee{
		100: {EmployeeID: 7, IdentityID: 100, ActuaryInfo: &model.ActuaryInfo{IsSupervisor: true}},
	}}
	router := supervisorRouter(repo, &commonauth.AuthContext{
		IdentityID:   100,
		IdentityType: commonauth.IdentityEmployee,
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/supervisor", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
}

func TestRequireSupervisorRejectsNonSupervisor(t *testing.T) {
	t.Parallel()

	employeeID := uint(7)
	repo := &supervisorEmployeeRepo{byID: map[uint]*model.Employee{
		employeeID: {EmployeeID: employeeID, ActuaryInfo: &model.ActuaryInfo{IsAgent: true}},
	}}
	router := supervisorRouter(repo, &commonauth.AuthContext{
		IdentityID:   100,
		IdentityType: commonauth.IdentityEmployee,
		EmployeeID:   &employeeID,
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/supervisor", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestRequireSupervisorRejectsMissingAuth(t *testing.T) {
	t.Parallel()

	router := supervisorRouter(&supervisorEmployeeRepo{}, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/supervisor", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}
