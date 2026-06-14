package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/audit"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	appErrors "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/permission"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/config"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/repository"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/service"
	uservalidator "github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/validator"
)

type userHandlerFixtures struct {
	adminIdentityID  uint
	adminEmployeeID  uint
	targetEmployeeID uint
	agentEmployeeID  uint
	clientID         uint
}

type fakeUserMailer struct {
	sentTo string
}

func (f *fakeUserMailer) Send(to, _, _ string) error {
	f.sentTo = to
	return nil
}

func setupUserHandlerDB(t *testing.T) *gorm.DB {
	t.Helper()

	uservalidator.RegisterValidators()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Identity{},
		&model.ActivationToken{},
		&model.Client{},
		&model.ClientPermission{},
		&model.Position{},
		&model.Employee{},
		&model.EmployeePermission{},
		&model.ActuaryInfo{},
		&audit.AuditLog{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func seedUserHandlerFixtures(t *testing.T, db *gorm.DB) userHandlerFixtures {
	t.Helper()

	position := model.Position{Title: "Trader"}
	if err := db.Create(&position).Error; err != nil {
		t.Fatalf("create position: %v", err)
	}

	adminIdentity := model.Identity{Email: "admin@example.com", Username: "admin", Type: auth.IdentityEmployee, Active: true}
	targetIdentity := model.Identity{Email: "target@example.com", Username: "target", Type: auth.IdentityEmployee, Active: true}
	agentIdentity := model.Identity{Email: "agent@example.com", Username: "agent", Type: auth.IdentityEmployee, Active: true}
	clientIdentity := model.Identity{Email: "client@example.com", Username: "client", Type: auth.IdentityClient, Active: true}
	if err := db.Create(&[]*model.Identity{&adminIdentity, &targetIdentity, &agentIdentity, &clientIdentity}).Error; err != nil {
		t.Fatalf("create identities: %v", err)
	}

	adminPermissions := make([]model.EmployeePermission, 0, len(permission.All))
	for _, p := range permission.All {
		adminPermissions = append(adminPermissions, model.EmployeePermission{Permission: p})
	}
	admin := model.Employee{
		IdentityID:  adminIdentity.ID,
		FirstName:   "Admin",
		LastName:    "User",
		Department:  "Backoffice",
		PositionID:  position.PositionID,
		Permissions: adminPermissions,
	}
	target := model.Employee{
		IdentityID: clientIdentity.ID + 1,
		FirstName:  "Target",
		LastName:   "Employee",
		Department: "Trading",
		PositionID: position.PositionID,
	}
	target.IdentityID = targetIdentity.ID
	agent := model.Employee{
		IdentityID:  agentIdentity.ID,
		FirstName:   "Agent",
		LastName:    "User",
		Department:  "Trading",
		PositionID:  position.PositionID,
		ActuaryInfo: &model.ActuaryInfo{IsAgent: true, Limit: 1000, UsedLimit: 200, NeedApproval: true},
	}
	if err := db.Create(&[]*model.Employee{&admin, &target, &agent}).Error; err != nil {
		t.Fatalf("create employees: %v", err)
	}

	client := model.Client{
		IdentityID:               clientIdentity.ID,
		FirstName:                "Client",
		LastName:                 "One",
		MobileVerificationSecret: "MOBILESECRET",
		Gender:                   "F",
		PhoneNumber:              "0601234567",
		Address:                  "Main 1",
	}
	if err := db.Create(&client).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}

	return userHandlerFixtures{
		adminIdentityID:  adminIdentity.ID,
		adminEmployeeID:  admin.EmployeeID,
		targetEmployeeID: target.EmployeeID,
		agentEmployeeID:  agent.EmployeeID,
		clientID:         client.ClientID,
	}
}

func newUserClientHandler(db *gorm.DB, mailer service.Mailer) *ClientHandler {
	return NewClientHandler(service.NewClientService(
		repository.NewClientRepository(db),
		repository.NewIdentityRepository(db),
		repository.NewActivationTokenRepository(db),
		mailer,
		&config.Configuration{URLs: config.URLConfig{FrontendBaseURL: "http://frontend.test"}},
		repository.NewGormTransactionManager(db),
	))
}

func newUserEmployeeHandler(db *gorm.DB) *EmployeeHandler {
	return NewEmployeeHandler(service.NewEmployeeService(
		repository.NewEmployeeRepository(db),
		repository.NewIdentityRepository(db),
		repository.NewActivationTokenRepository(db),
		repository.NewPositionRepository(db),
		&fakeUserMailer{},
		&config.Configuration{URLs: config.URLConfig{FrontendBaseURL: "http://frontend.test"}},
		repository.NewGormTransactionManager(db),
		nil,
	))
}

func newUserActuaryHandler(db *gorm.DB) *ActuaryHandler {
	return NewActuaryHandler(service.NewActuaryService(
		repository.NewActuaryRepository(db),
		repository.NewEmployeeRepository(db),
		nil,
		audit.NewService(audit.NewRepository(db)),
	))
}

func performUserJSON(router http.Handler, method, path string, body any) *httptest.ResponseRecorder {
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

func TestClientHandlerRegisterListUpdateAndSecret(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	db := setupUserHandlerDB(t)
	fixtures := seedUserHandlerFixtures(t, db)
	mailer := &fakeUserMailer{}
	h := newUserClientHandler(db, mailer)

	router := gin.New()
	router.Use(appErrors.ErrorHandler())
	router.Use(func(c *gin.Context) {
		auth.SetAuth(c, &auth.AuthContext{
			IdentityID:   300,
			IdentityType: auth.IdentityClient,
			ClientID:     &fixtures.clientID,
		})
		c.Next()
	})
	router.POST("/clients/register", h.Register)
	router.GET("/clients", h.ListClients)
	router.PATCH("/clients/:id", h.UpdateClient)
	router.GET("/secret-mobile", h.GetMobileSecret)

	rec := performUserJSON(router, http.MethodPost, "/clients/register", gin.H{
		"first_name":    "New",
		"last_name":     "Client",
		"date_of_birth": time.Date(1995, 3, 2, 0, 0, 0, 0, time.UTC),
		"gender":        "M",
		"email":         "new.client@example.com",
		"username":      "newclient",
		"phone_number":  "0600000000",
		"address":       "Side 2",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("register client status = %d body=%s", rec.Code, rec.Body.String())
	}
	if mailer.sentTo != "new.client@example.com" {
		t.Fatalf("activation mail sent to %q", mailer.sentTo)
	}

	rec = performUserJSON(router, http.MethodGet, "/clients?page=1&page_size=10", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list clients status = %d body=%s", rec.Code, rec.Body.String())
	}
	var clients struct {
		Total    int64 `json:"total"`
		Page     int   `json:"page"`
		PageSize int   `json:"page_size"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &clients); err != nil {
		t.Fatalf("decode clients: %v", err)
	}
	if clients.Total == 0 || clients.Page != 1 || clients.PageSize != 10 {
		t.Fatalf("unexpected clients response %#v", clients)
	}

	updatedName := "Updated"
	rec = performUserJSON(router, http.MethodPatch, "/clients/"+strconv.FormatUint(uint64(fixtures.clientID), 10), gin.H{"first_name": updatedName})
	if rec.Code != http.StatusOK {
		t.Fatalf("update client status = %d body=%s", rec.Code, rec.Body.String())
	}
	var updatedClient struct {
		FirstName string `json:"first_name"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &updatedClient); err != nil {
		t.Fatalf("decode updated client: %v", err)
	}
	if updatedClient.FirstName != updatedName {
		t.Fatalf("updated first name = %q", updatedClient.FirstName)
	}

	rec = performUserJSON(router, http.MethodGet, "/secret-mobile", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("mobile secret status = %d body=%s", rec.Code, rec.Body.String())
	}
	var secret struct {
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &secret); err != nil {
		t.Fatalf("decode secret: %v", err)
	}
	if secret.Secret != "MOBILESECRET" {
		t.Fatalf("mobile secret = %q", secret.Secret)
	}

	rec = performUserJSON(router, http.MethodPatch, "/clients/bad", gin.H{"first_name": "Bad"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad client id status = %d body=%s", rec.Code, rec.Body.String())
	}
	rec = performUserJSON(router, http.MethodPost, "/clients/register", gin.H{"first_name": "missing"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid register status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestEmployeeHandlerListGetUpdateAndDeactivate(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	db := setupUserHandlerDB(t)
	fixtures := seedUserHandlerFixtures(t, db)
	h := newUserEmployeeHandler(db)

	router := gin.New()
	router.Use(appErrors.ErrorHandler())
	router.Use(func(c *gin.Context) {
		auth.SetAuth(c, &auth.AuthContext{
			IdentityID:   fixtures.adminIdentityID,
			IdentityType: auth.IdentityEmployee,
			EmployeeID:   &fixtures.adminEmployeeID,
			Permissions:  permission.All,
		})
		c.Next()
	})
	router.GET("/employees", h.ListEmployees)
	router.GET("/employees/:id", h.GetEmployee)
	router.PATCH("/employees/:id", h.UpdateEmployee)
	router.POST("/employees/:id/deactivate", h.DeactivateEmployee)

	rec := performUserJSON(router, http.MethodGet, "/employees?page=1&page_size=10", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list employees status = %d body=%s", rec.Code, rec.Body.String())
	}
	var employees struct {
		Total int64 `json:"total"`
		Data  []struct {
			ID        uint   `json:"id"`
			FirstName string `json:"first_name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &employees); err != nil {
		t.Fatalf("decode employees: %v", err)
	}
	if employees.Total != 3 || len(employees.Data) != 3 {
		t.Fatalf("unexpected employees response %#v", employees)
	}

	targetPath := "/employees/" + strconv.FormatUint(uint64(fixtures.targetEmployeeID), 10)
	rec = performUserJSON(router, http.MethodGet, targetPath, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get employee status = %d body=%s", rec.Code, rec.Body.String())
	}

	department := "Risk"
	rec = performUserJSON(router, http.MethodPatch, targetPath, gin.H{"department": department})
	if rec.Code != http.StatusOK {
		t.Fatalf("update employee status = %d body=%s", rec.Code, rec.Body.String())
	}
	var updatedEmployee struct {
		Department string `json:"department"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &updatedEmployee); err != nil {
		t.Fatalf("decode updated employee: %v", err)
	}
	if updatedEmployee.Department != department {
		t.Fatalf("department = %q", updatedEmployee.Department)
	}

	rec = performUserJSON(router, http.MethodPost, targetPath+"/deactivate", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("deactivate employee status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = performUserJSON(router, http.MethodGet, "/employees/bad", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad employee id status = %d body=%s", rec.Code, rec.Body.String())
	}
	rec = performUserJSON(router, http.MethodGet, "/employees/999999", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing employee status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestActuaryHandlerListUpdateAndReset(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	db := setupUserHandlerDB(t)
	fixtures := seedUserHandlerFixtures(t, db)
	h := newUserActuaryHandler(db)

	router := gin.New()
	router.Use(appErrors.ErrorHandler())
	router.Use(func(c *gin.Context) {
		auth.SetAuth(c, &auth.AuthContext{
			IdentityID:   fixtures.adminIdentityID,
			IdentityType: auth.IdentityEmployee,
			EmployeeID:   &fixtures.adminEmployeeID,
			Permissions:  permission.All,
		})
		c.Next()
	})
	router.GET("/actuaries", h.ListActuaries)
	router.PATCH("/actuaries/:id", h.UpdateActuarySettings)
	router.POST("/actuaries/:id/reset-used-limit", h.ResetUsedLimit)

	rec := performUserJSON(router, http.MethodGet, "/actuaries?page=1&page_size=10&type=agent", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list actuaries status = %d body=%s", rec.Code, rec.Body.String())
	}
	var list struct {
		Total int64 `json:"total"`
		Data  []struct {
			ID        uint    `json:"id"`
			Limit     float64 `json:"limit"`
			UsedLimit float64 `json:"used_limit"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode actuaries: %v", err)
	}
	if list.Total != 1 || len(list.Data) != 1 || list.Data[0].ID != fixtures.agentEmployeeID {
		t.Fatalf("unexpected actuary list %#v", list)
	}

	agentPath := "/actuaries/" + strconv.FormatUint(uint64(fixtures.agentEmployeeID), 10)
	rec = performUserJSON(router, http.MethodPatch, agentPath, gin.H{"limit": 1500.0, "need_approval": false})
	if rec.Code != http.StatusOK {
		t.Fatalf("update actuary status = %d body=%s", rec.Code, rec.Body.String())
	}
	var updated struct {
		Limit        float64 `json:"limit"`
		NeedApproval bool    `json:"need_approval"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode updated actuary: %v", err)
	}
	if updated.Limit != 1500 || updated.NeedApproval {
		t.Fatalf("unexpected updated actuary %#v", updated)
	}

	rec = performUserJSON(router, http.MethodPost, agentPath+"/reset-used-limit", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("reset used limit status = %d body=%s", rec.Code, rec.Body.String())
	}
	var reset struct {
		UsedLimit float64 `json:"used_limit"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &reset); err != nil {
		t.Fatalf("decode reset actuary: %v", err)
	}
	if reset.UsedLimit != 0 {
		t.Fatalf("used limit after reset = %v", reset.UsedLimit)
	}

	rec = performUserJSON(router, http.MethodPatch, "/actuaries/bad", gin.H{"limit": 10})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad actuary id status = %d body=%s", rec.Code, rec.Body.String())
	}
	rec = performUserJSON(router, http.MethodPost, "/actuaries/999999/reset-used-limit", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing actuary status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthHandlerRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	uservalidator.RegisterValidators()
	h := NewAuthHandler(nil)

	router := gin.New()
	router.Use(appErrors.ErrorHandler())
	router.POST("/auth/login", h.Login)
	router.POST("/auth/refresh", h.RefreshToken)
	router.POST("/auth/activate", h.Activate)
	router.POST("/auth/resend-activation", h.ResendActivation)
	router.POST("/auth/forgot-password", h.ForgotPassword)
	router.POST("/auth/reset-password", h.ResetPassword)
	router.POST("/auth/change-password", h.ChangePassword)

	cases := []struct {
		name string
		path string
		body any
	}{
		{name: "login missing password", path: "/auth/login", body: gin.H{"email": "client@example.com"}},
		{name: "refresh missing token", path: "/auth/refresh", body: gin.H{}},
		{name: "activate weak password", path: "/auth/activate", body: gin.H{"token": "activation-token", "password": "short"}},
		{name: "resend invalid email", path: "/auth/resend-activation", body: gin.H{"email": "bad-email"}},
		{name: "forgot invalid email", path: "/auth/forgot-password", body: gin.H{"email": "bad-email"}},
		{name: "reset weak password", path: "/auth/reset-password", body: gin.H{"token": "reset-token", "new_password": "short"}},
		{name: "change weak password", path: "/auth/change-password", body: gin.H{"old_password": "Oldpass12", "new_password": "short"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := performUserJSON(router, http.MethodPost, tc.path, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("%s status = %d body=%s", tc.path, rec.Code, rec.Body.String())
			}
		})
	}
}
