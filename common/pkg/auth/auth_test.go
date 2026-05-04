package auth_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/jwt"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/logging"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/permission"
)

type fakeTokenVerifier struct {
	claims *jwt.Claims
	err    error
}

func (f *fakeTokenVerifier) VerifyToken(_ string) (*jwt.Claims, error) {
	return f.claims, f.err
}

type fakePermissionProvider struct {
	perms []permission.Permission
	err   error
}

func (f *fakePermissionProvider) GetPermissions(_ context.Context, _ *jwt.Claims) ([]permission.Permission, error) {
	return f.perms, f.err
}

func init() {
	gin.SetMode(gin.TestMode)
	_ = logging.Init("test")
}

func middlewareRouter(verifier auth.TokenVerifier, provider auth.PermissionProvider) *gin.Engine {
	router := gin.New()
	router.Use(errors.ErrorHandler())
	router.Use(auth.Middleware(verifier, provider))
	router.GET("/test", func(c *gin.Context) {
		ac := auth.GetAuth(c)
		c.JSON(http.StatusOK, gin.H{
			"identity_id": ac.IdentityID,
			"client_id":   ac.ClientID,
			"employee_id": ac.EmployeeID,
		})
	})
	return router
}

func doGet(router *gin.Engine, authHeader string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	router.ServeHTTP(w, req)
	return w
}

func doGetPath(router *gin.Engine, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	router.ServeHTTP(w, req)
	return w
}

func TestMiddleware_ValidToken(t *testing.T) {
	t.Parallel()

	router := middlewareRouter(
		&fakeTokenVerifier{claims: &jwt.Claims{IdentityID: 42, IdentityType: "employee", EmployeeID: uintPtr(8)}},
		&fakePermissionProvider{perms: []permission.Permission{permission.EmployeeView}},
	)

	w := doGet(router, "Bearer valid-token")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "42")
}

func TestMiddleware_MissingHeader(t *testing.T) {
	t.Parallel()

	router := middlewareRouter(
		&fakeTokenVerifier{},
		&fakePermissionProvider{},
	)

	w := doGet(router, "")
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMiddleware_MalformedHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header string
	}{
		{"no bearer prefix", "Token abc123"},
		{"basic auth", "Basic abc123"},
		{"only token value", "abc123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			router := middlewareRouter(
				&fakeTokenVerifier{claims: &jwt.Claims{IdentityID: 1, IdentityType: "employee"}},
				&fakePermissionProvider{perms: []permission.Permission{}},
			)

			w := doGet(router, tt.header)
			require.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

func TestMiddleware_InvalidToken(t *testing.T) {
	t.Parallel()

	router := middlewareRouter(
		&fakeTokenVerifier{err: fmt.Errorf("invalid token")},
		&fakePermissionProvider{},
	)

	w := doGet(router, "Bearer bad-token")
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMiddleware_PermissionProviderError(t *testing.T) {
	t.Parallel()

	router := middlewareRouter(
		&fakeTokenVerifier{claims: &jwt.Claims{IdentityID: 1, IdentityType: "employee"}},
		&fakePermissionProvider{err: fmt.Errorf("db down")},
	)

	w := doGet(router, "Bearer valid-token")
	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestMiddleware_SetsAuthContext(t *testing.T) {
	t.Parallel()

	var captured *auth.AuthContext
	router := gin.New()
	router.Use(auth.Middleware(
		&fakeTokenVerifier{claims: &jwt.Claims{IdentityID: 7, IdentityType: "employee", EmployeeID: uintPtr(3)}},
		&fakePermissionProvider{perms: []permission.Permission{permission.EmployeeView, permission.EmployeeCreate}},
	))
	router.GET("/test", func(c *gin.Context) {
		captured = auth.GetAuth(c)
		c.Status(http.StatusOK)
	})

	doGet(router, "Bearer valid-token")

	require.NotNil(t, captured)
	require.Equal(t, uint(7), captured.IdentityID)
	require.Equal(t, auth.IdentityEmployee, captured.IdentityType)
	require.NotNil(t, captured.EmployeeID)
	require.Equal(t, uint(3), *captured.EmployeeID)
	require.Nil(t, captured.ClientID)
	require.ElementsMatch(t, []permission.Permission{permission.EmployeeView, permission.EmployeeCreate}, captured.Permissions)
}

func TestMiddleware_SetsStdlibContext(t *testing.T) {
	t.Parallel()

	var captured *auth.AuthContext
	router := gin.New()
	router.Use(auth.Middleware(
		&fakeTokenVerifier{claims: &jwt.Claims{IdentityID: 7, IdentityType: "client", ClientID: uintPtr(22)}},
		&fakePermissionProvider{perms: []permission.Permission{permission.EmployeeView}},
	))
	router.GET("/test", func(c *gin.Context) {
		captured = auth.GetAuthFromContext(c.Request.Context())
		c.Status(http.StatusOK)
	})

	doGet(router, "Bearer valid-token")

	require.NotNil(t, captured)
	require.Equal(t, uint(7), captured.IdentityID)
	require.Equal(t, auth.IdentityClient, captured.IdentityType)
	require.NotNil(t, captured.ClientID)
	require.Equal(t, uint(22), *captured.ClientID)
	require.Nil(t, captured.EmployeeID)
}

func requirePermissionRouter(verifier auth.TokenVerifier, provider auth.PermissionProvider, required ...permission.Permission) *gin.Engine {
	router := gin.New()
	router.Use(errors.ErrorHandler())
	router.Use(auth.Middleware(verifier, provider))
	router.GET("/test", auth.RequirePermission(required...), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return router
}

func TestRequirePermission_HasPermission(t *testing.T) {
	t.Parallel()

	router := requirePermissionRouter(
		&fakeTokenVerifier{claims: &jwt.Claims{IdentityID: 1, IdentityType: "employee"}},
		&fakePermissionProvider{perms: []permission.Permission{permission.EmployeeView, permission.EmployeeCreate}},
		permission.EmployeeView,
	)

	w := doGet(router, "Bearer valid-token")
	require.Equal(t, http.StatusOK, w.Code)
}

func TestRequirePermission_HasMultipleRequired(t *testing.T) {
	t.Parallel()

	router := requirePermissionRouter(
		&fakeTokenVerifier{claims: &jwt.Claims{IdentityID: 1, IdentityType: "employee"}},
		&fakePermissionProvider{perms: []permission.Permission{permission.EmployeeView, permission.EmployeeCreate, permission.EmployeeUpdate}},
		permission.EmployeeView, permission.EmployeeCreate,
	)

	w := doGet(router, "Bearer valid-token")
	require.Equal(t, http.StatusOK, w.Code)
}

func TestRequirePermission_MissingPermission(t *testing.T) {
	t.Parallel()

	router := requirePermissionRouter(
		&fakeTokenVerifier{claims: &jwt.Claims{IdentityID: 1, IdentityType: "employee"}},
		&fakePermissionProvider{perms: []permission.Permission{permission.EmployeeView}},
		permission.EmployeeDelete,
	)

	w := doGet(router, "Bearer valid-token")
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequirePermission_NoAuthContext(t *testing.T) {
	t.Parallel()

	router := gin.New()
	router.Use(errors.ErrorHandler())
	router.GET("/test", auth.RequirePermission(permission.EmployeeView), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := doGet(router, "")
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequirePermission_EmptyUserPermissions(t *testing.T) {
	t.Parallel()

	router := requirePermissionRouter(
		&fakeTokenVerifier{claims: &jwt.Claims{IdentityID: 1, IdentityType: "employee"}},
		&fakePermissionProvider{perms: []permission.Permission{}},
		permission.EmployeeView,
	)

	w := doGet(router, "Bearer valid-token")
	require.Equal(t, http.StatusForbidden, w.Code)
}

func clientPathMatchRouter(authContext *auth.AuthContext) *gin.Engine {
	router := gin.New()
	router.Use(errors.ErrorHandler())
	if authContext != nil {
		router.Use(func(c *gin.Context) {
			auth.SetAuth(c, authContext)
			c.Next()
		})
	}

	router.GET("/clients/:clientId/transfers", auth.RequireClientSelf("clientId", true), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	return router
}

func TestRequireClientPathMatchOrEmployee_ClientMatch(t *testing.T) {
	t.Parallel()

	clientID := uint(5)
	router := clientPathMatchRouter(&auth.AuthContext{
		IdentityType: auth.IdentityClient,
		ClientID:     &clientID,
	})

	w := doGetPath(router, "/clients/5/transfers")
	require.Equal(t, http.StatusOK, w.Code)
}

func TestRequireClientPathMatchOrEmployee_ClientMismatch(t *testing.T) {
	t.Parallel()

	clientID := uint(5)
	router := clientPathMatchRouter(&auth.AuthContext{
		IdentityType: auth.IdentityClient,
		ClientID:     &clientID,
	})

	w := doGetPath(router, "/clients/6/transfers")
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequireClientPathMatchOrEmployee_EmployeeAllowed(t *testing.T) {
	t.Parallel()

	employeeID := uint(11)
	router := clientPathMatchRouter(&auth.AuthContext{
		IdentityType: auth.IdentityEmployee,
		EmployeeID:   &employeeID,
	})

	w := doGetPath(router, "/clients/99/transfers")
	require.Equal(t, http.StatusOK, w.Code)
}

func TestRequireClientPathMatchOrEmployee_InvalidPathClientID(t *testing.T) {
	t.Parallel()

	clientID := uint(5)
	router := clientPathMatchRouter(&auth.AuthContext{
		IdentityType: auth.IdentityClient,
		ClientID:     &clientID,
	})

	w := doGetPath(router, "/clients/not-a-number/transfers")
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRequireClientPathMatchOrEmployee_NoAuth(t *testing.T) {
	t.Parallel()

	router := clientPathMatchRouter(nil)
	w := doGetPath(router, "/clients/5/transfers")
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetSubjectFromContext_Client(t *testing.T) {
	t.Parallel()

	clientID := uint(22)
	ctx := auth.SetAuthOnContext(context.Background(), &auth.AuthContext{
		IdentityType: auth.IdentityClient,
		ClientID:     &clientID,
	})

	subjectID, err := auth.GetSubjectFromContext(ctx)
	require.NoError(t, err)
	require.Equal(t, uint(22), subjectID)
}

func TestGetSubjectFromContext_Employee(t *testing.T) {
	t.Parallel()

	employeeID := uint(7)
	ctx := auth.SetAuthOnContext(context.Background(), &auth.AuthContext{
		IdentityType: auth.IdentityEmployee,
		EmployeeID:   &employeeID,
	})

	subjectID, err := auth.GetSubjectFromContext(ctx)
	require.NoError(t, err)
	require.Equal(t, uint(7), subjectID)
}

func TestGetSubjectFromContext_NoAuth(t *testing.T) {
	t.Parallel()

	subjectID, err := auth.GetSubjectFromContext(context.Background())
	require.Error(t, err)
	require.Zero(t, subjectID)
	require.Contains(t, err.Error(), "not authenticated")
}

func TestGetSubjectFromContext_MissingClientID(t *testing.T) {
	t.Parallel()

	ctx := auth.SetAuthOnContext(context.Background(), &auth.AuthContext{
		IdentityType: auth.IdentityClient,
	})

	subjectID, err := auth.GetSubjectFromContext(ctx)
	require.Error(t, err)
	require.Zero(t, subjectID)
	require.Contains(t, err.Error(), "not authenticated")
}

func TestGetSubjectFromContext_InvalidIdentityType(t *testing.T) {
	t.Parallel()

	ctx := auth.SetAuthOnContext(context.Background(), &auth.AuthContext{
		IdentityType: auth.IdentityType("partner"),
	})

	subjectID, err := auth.GetSubjectFromContext(ctx)
	require.Error(t, err)
	require.Zero(t, subjectID)
	require.Contains(t, err.Error(), "access denied")
}

func uintPtr(v uint) *uint {
	return &v
}

func TestAnyOf_AllowsSingleExecution(t *testing.T) {
	t.Parallel()

	var executions int32

	router := gin.New()
	router.Use(errors.ErrorHandler())
	router.GET(
		"/test",
		auth.AnyOf(
			func(c *gin.Context) {
				_ = c.Error(errors.ForbiddenErr("first denied"))
				c.Abort()
			},
			func(c *gin.Context) {
				c.Next()
			},
		),
		func(c *gin.Context) {
			atomic.AddInt32(&executions, 1)
			c.JSON(http.StatusOK, gin.H{"ok": true})
		},
	)

	w := doGetPath(router, "/test")
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, int32(1), atomic.LoadInt32(&executions))
}

func TestAnyOf_DeniesWhenNoMiddlewareMatches(t *testing.T) {
	t.Parallel()

	router := gin.New()
	router.Use(errors.ErrorHandler())
	router.GET(
		"/test",
		auth.AnyOf(
			func(c *gin.Context) {
				_ = c.Error(errors.ForbiddenErr("first denied"))
				c.Abort()
			},
			func(c *gin.Context) {
				_ = c.Error(errors.ForbiddenErr("second denied"))
				c.Abort()
			},
		),
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)

	w := doGetPath(router, "/test")
	require.Equal(t, http.StatusForbidden, w.Code)
}
