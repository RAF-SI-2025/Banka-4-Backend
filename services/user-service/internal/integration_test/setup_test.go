//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"common/pkg/auth"
	commonjwt "common/pkg/jwt"
	"common/pkg/logging"
	commonpermission "common/pkg/permission"
	"user-service/internal/config"
	"user-service/internal/handler"
	"user-service/internal/model"
	dbpermission "user-service/internal/permission"
	"user-service/internal/repository"
	"user-service/internal/server"
	"user-service/internal/service"

	"github.com/gin-gonic/gin"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"golang.org/x/crypto/bcrypt"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var testSetupOnce sync.Once
var uniqueCounter atomic.Uint64

func init() {
	testSetupOnce.Do(func() {
		gin.SetMode(gin.TestMode)
		_ = logging.Init("test")
	})
}

type fakeMailer struct{}

func (f *fakeMailer) Send(_, _, _ string) error {
	return nil
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	ctx := context.Background()
	container, err := tcpostgres.Run(
		ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("user_service_test"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.BasicWaitStrategies(),
	)

	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}

	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Fatalf("terminate postgres container: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("build postgres connection string: %v", err)
	}

	db, err := gorm.Open(gormpostgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open gorm db: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}

	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Fatalf("close sql db: %v", err)
		}
	})

	if err := db.AutoMigrate(
		&model.Identity{},
		&model.Employee{},
		&model.Client{},
		&model.Position{},
		&model.ActivationToken{},
		&model.ResetToken{},
		&model.RefreshToken{},
		&model.EmployeePermission{},
	); err != nil {
		t.Fatalf("auto migrate test schema: %v", err)
	}

	return db
}

func setupTestRouter(t *testing.T, db *gorm.DB) *gin.Engine {
	t.Helper()

	cfg := testConfig()

	identityRepo := repository.NewIdentityRepository(db)
	empRepo := repository.NewEmployeeRepository(db)
	clientRepo := repository.NewClientRepository(db)
	actTokenRepo := repository.NewActivationTokenRepository(db)
	resetTokenRepo := repository.NewResetTokenRepository(db)
	refreshTokenRepo := repository.NewRefreshTokenRepository(db)
	positionRepo := repository.NewPositionRepository(db)

	mailer := &fakeMailer{}

	authSvc := service.NewAuthService(
		identityRepo,
		empRepo,
		clientRepo,
		actTokenRepo,
		resetTokenRepo,
		refreshTokenRepo,
		mailer,
		cfg,
	)

	empSvc := service.NewEmployeeService(
		empRepo,
		identityRepo,
		actTokenRepo,
		positionRepo,
		mailer,
		cfg,
	)

	clientSvc := service.NewClientService(
		clientRepo,
		identityRepo,
		actTokenRepo,
		mailer,
		cfg,
	)

	authHandler := handler.NewAuthHandler(authSvc)
	empHandler := handler.NewEmployeeHandler(empSvc)
	clientHandler := handler.NewClientHandler(clientSvc)
	healthHandler := handler.NewHealthHandler()

	r := gin.New()
	server.InitRouter(r, cfg)
	server.SetupRoutes(
		r,
		healthHandler,
		authHandler,
		empHandler,
		clientHandler,
		auth.TokenVerifier(commonjwt.NewJWTVerifier(cfg.JWTSecret)),
		dbpermission.NewDBPermissionProvider(db),
	)

	return r
}

func testConfig() *config.Configuration {
	return &config.Configuration{
		Env:           "test",
		JWTSecret:     "test-secret",
		JWTExpiry:     15,
		RefreshExpiry: 10080,
		URLs: config.URLConfig{
			FrontendBaseURL: "http://localhost:5173",
		},
	}
}

func seedPosition(t *testing.T, db *gorm.DB) *model.Position {
	t.Helper()

	position := &model.Position{
		Title: uniqueValue(t, "Position"),
	}

	if err := db.Create(position).Error; err != nil {
		t.Fatalf("seed position: %v", err)
	}

	return position
}

func seedEmployee(t *testing.T, db *gorm.DB, positionID uint) (*model.Identity, *model.Employee) {
	t.Helper()
	return seedEmployeeWithPermissions(t, db, positionID)
}

func seedEmployeeWithPermissions(t *testing.T, db *gorm.DB, positionID uint, permissions ...commonpermission.Permission) (*model.Identity, *model.Employee) {
	t.Helper()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("Password12"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	now := time.Now().UTC()
	username := strings.ToLower(uniqueValue(t, "user"))
	email := uniqueValue(t, "user") + "@example.com"

	identity := &model.Identity{
		Email:        email,
		Username:     username,
		PasswordHash: string(hashedPassword),
		Type:         auth.IdentityEmployee,
		Active:       true,
	}
	if err := db.Create(identity).Error; err != nil {
		t.Fatalf("seed identity: %v", err)
	}

	employee := &model.Employee{
		IdentityID:  identity.ID,
		FirstName:   "Test",
		LastName:    uniqueValue(t, "User"),
		Gender:      "male",
		DateOfBirth: now.AddDate(-30, 0, 0),
		PhoneNumber: "0601234567",
		Address:     "Integration Street 1",
		Department:  "Engineering",
		PositionID:  positionID,
	}

	for _, perm := range permissions {
		employee.Permissions = append(employee.Permissions, model.EmployeePermission{Permission: perm})
	}

	if err := db.Create(employee).Error; err != nil {
		t.Fatalf("seed employee: %v", err)
	}

	employee.Identity = *identity
	return identity, employee
}

func authHeader(t *testing.T, identityID uint, employeeID ...uint) string {
	t.Helper()

	var employeeClaim *uint
	if len(employeeID) > 0 {
		employeeClaim = &employeeID[0]
	}

	token, err := commonjwt.GenerateToken(&commonjwt.Claims{
		IdentityID:   identityID,
		IdentityType: string(auth.IdentityEmployee),
		EmployeeID:   employeeClaim,
	}, testConfig().JWTSecret, testConfig().JWTExpiry)
	if err != nil {
		t.Fatalf("generate auth token: %v", err)
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

func performRawJSONRequest(t *testing.T, router *gin.Engine, method, path, rawBody, authorization string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(rawBody))
	req.Header.Set("Content-Type", "application/json")
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

type loginResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	User         struct {
		ID uint `json:"id"`
	} `json:"user"`
}

type employeeResponse struct {
	ID         uint   `json:"id"`
	Email      string `json:"email"`
	Username   string `json:"username"`
	Department string `json:"department"`
	PositionID uint   `json:"position_id"`
}

type listEmployeesResponse struct {
	Data       []employeeResponse `json:"data"`
	Total      int64              `json:"total"`
	Page       int                `json:"page"`
	PageSize   int                `json:"page_size"`
	TotalPages int                `json:"total_pages"`
}

type appErrorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func loginEmployee(t *testing.T, router *gin.Engine, email, password string) loginResponse {
	t.Helper()

	recorder := performRequest(t, router, http.MethodPost, "/api/auth/login", map[string]any{
		"email":    email,
		"password": password,
	}, "")

	requireStatus(t, recorder, http.StatusOK)
	return decodeResponse[loginResponse](t, recorder)
}

func verifyAccessToken(t *testing.T, token string) *commonjwt.Claims {
	t.Helper()

	verifier := commonjwt.NewJWTVerifier(testConfig().JWTSecret)
	claims, err := verifier.VerifyToken(token)
	if err != nil {
		t.Fatalf("verify access token: %v", err)
	}

	return claims
}
