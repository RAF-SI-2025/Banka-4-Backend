package service

import (
	"context"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/audit"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/permission"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/config"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/model"
)

type fakeIdentityRepo struct {
	byID       *model.Identity
	byEmail    *model.Identity
	byUsername *model.Identity

	emailExists    bool
	usernameExists bool

	findErr   error
	createErr error
	updateErr error
	existsErr error

	createdIdentity *model.Identity
	updatedIdentity *model.Identity
}

func (f *fakeIdentityRepo) Create(_ context.Context, identity *model.Identity) error {
	f.createdIdentity = identity
	identity.ID = 1
	return f.createErr
}

func (f *fakeIdentityRepo) FindByID(_ context.Context, _ uint) (*model.Identity, error) {
	return f.byID, f.findErr
}

func (f *fakeIdentityRepo) FindByEmail(_ context.Context, _ string) (*model.Identity, error) {
	return f.byEmail, f.findErr
}

func (f *fakeIdentityRepo) FindByUsername(_ context.Context, _ string) (*model.Identity, error) {
	return f.byUsername, f.findErr
}

func (f *fakeIdentityRepo) Update(_ context.Context, identity *model.Identity) error {
	f.updatedIdentity = identity
	return f.updateErr
}

func (f *fakeIdentityRepo) EmailExists(_ context.Context, _ string) (bool, error) {
	return f.emailExists, f.existsErr
}

func (f *fakeIdentityRepo) UsernameExists(_ context.Context, _ string) (bool, error) {
	return f.usernameExists, f.existsErr
}

type fakeEmployeeRepo struct {
	byID         *model.Employee
	byIdentityID *model.Employee
	byIDs        map[uint]*model.Employee
	allEmps      []model.Employee
	allTotal     int64

	findErr   error
	createErr error
	updateErr error
	getAllErr error

	createdEmployee *model.Employee
	updatedEmployee *model.Employee
}

func (f *fakeEmployeeRepo) FindByID(_ context.Context, id uint) (*model.Employee, error) {
	if f.byIDs != nil {
		if employee, ok := f.byIDs[id]; ok {
			return employee, f.findErr
		}
	}

	return f.byID, f.findErr
}

func (f *fakeEmployeeRepo) FindByIdentityID(_ context.Context, _ uint) (*model.Employee, error) {
	return f.byIdentityID, f.findErr
}

func (f *fakeEmployeeRepo) Create(_ context.Context, emp *model.Employee) error {
	f.createdEmployee = emp
	return f.createErr
}

func (f *fakeEmployeeRepo) Update(_ context.Context, emp *model.Employee) error {
	f.updatedEmployee = emp
	return f.updateErr
}

func (f *fakeEmployeeRepo) GetAll(_ context.Context, _, _, _, _ string, _, _ int) ([]model.Employee, int64, error) {
	return f.allEmps, f.allTotal, f.getAllErr
}

type fakeActuaryRepo struct {
	byEmployeeID map[uint]*model.ActuaryInfo
	allEmployees []model.Employee
	allTotal     int64

	findErr      error
	getAllErr    error
	saveErr      error
	incrementErr error
	resetErr     error
	resetAllErr  error
}

func (f *fakeActuaryRepo) FindByEmployeeID(_ context.Context, employeeID uint) (*model.ActuaryInfo, error) {
	if f.byEmployeeID == nil {
		return nil, f.findErr
	}

	return f.byEmployeeID[employeeID], f.findErr
}

func (f *fakeActuaryRepo) GetAll(_ context.Context, _, _, _, _, _, _ string, _, _ *bool, _, _ int) ([]model.Employee, int64, error) {
	return f.allEmployees, f.allTotal, f.getAllErr
}

func (f *fakeActuaryRepo) Save(_ context.Context, actuary *model.ActuaryInfo) error {
	if f.byEmployeeID == nil {
		f.byEmployeeID = map[uint]*model.ActuaryInfo{}
	}

	copy := *actuary
	f.byEmployeeID[actuary.EmployeeID] = &copy
	return f.saveErr
}

func (f *fakeActuaryRepo) IncrementUsedLimit(_ context.Context, employeeID uint, amount float64) (*model.ActuaryInfo, error) {
	if f.incrementErr != nil {
		return nil, f.incrementErr
	}
	if f.byEmployeeID == nil || f.byEmployeeID[employeeID] == nil {
		return nil, nil
	}

	f.byEmployeeID[employeeID].UsedLimit += amount
	return f.byEmployeeID[employeeID], nil
}

func (f *fakeActuaryRepo) ResetUsedLimit(_ context.Context, employeeID uint) error {
	if f.byEmployeeID != nil && f.byEmployeeID[employeeID] != nil {
		f.byEmployeeID[employeeID].UsedLimit = 0
	}

	return f.resetErr
}

func (f *fakeActuaryRepo) ResetAllUsedLimits(_ context.Context) error {
	for _, actuary := range f.byEmployeeID {
		if actuary != nil && actuary.IsAgent {
			actuary.UsedLimit = 0
		}
	}

	return f.resetAllErr
}

type fakeClientRepo struct {
	byID         *model.Client
	byIDs        []model.Client
	byIdentityID *model.Client
	allClients   []model.Client
	allTotal     int64

	findErr   error
	createErr error
	updateErr error
	getAllErr error

	createdClient *model.Client
	updatedClient *model.Client
}

func (f *fakeClientRepo) Create(_ context.Context, client *model.Client) error {
	f.createdClient = client
	return f.createErr
}

func (f *fakeClientRepo) FindByIdentityID(_ context.Context, _ uint) (*model.Client, error) {
	return f.byIdentityID, f.findErr
}

func (f *fakeClientRepo) FindByID(_ context.Context, id uint) (*model.Client, error) {
	return f.byID, f.findErr
}

func (f *fakeClientRepo) FindByIDs(_ context.Context, id []uint) ([]model.Client, error) {
	return f.byIDs, f.findErr
}

func (f *fakeClientRepo) FindAll(_ context.Context, _ *dto.ListClientsQuery) ([]model.Client, int64, error) {
	return f.allClients, f.allTotal, f.getAllErr
}

func (f *fakeClientRepo) Update(_ context.Context, client *model.Client) error {
	f.updatedClient = client
	return f.updateErr
}

type fakeActivationTokenRepo struct {
	token     *model.ActivationToken
	findErr   error
	createErr error
	deleteErr error
}

func (f *fakeActivationTokenRepo) Create(_ context.Context, _ *model.ActivationToken) error {
	return f.createErr
}

func (f *fakeActivationTokenRepo) FindByToken(_ context.Context, _ string) (*model.ActivationToken, error) {
	return f.token, f.findErr
}

func (f *fakeActivationTokenRepo) Delete(_ context.Context, _ *model.ActivationToken) error {
	return f.deleteErr
}

func (f *fakeActivationTokenRepo) DeleteByIdentityID(_ context.Context, _ uint) error {
	return f.deleteErr
}

type fakeResetTokenRepo struct {
	token     *model.ResetToken
	findErr   error
	createErr error
	deleteErr error
}

func (f *fakeResetTokenRepo) Create(_ context.Context, _ *model.ResetToken) error {
	return f.createErr
}

func (f *fakeResetTokenRepo) FindByToken(_ context.Context, _ string) (*model.ResetToken, error) {
	return f.token, f.findErr
}

func (f *fakeResetTokenRepo) DeleteByIdentityID(_ context.Context, _ uint) error {
	return f.deleteErr
}

type fakeRefreshTokenRepo struct {
	token     *model.RefreshToken
	findErr   error
	createErr error
	deleteErr error
}

func (f *fakeRefreshTokenRepo) Create(_ context.Context, _ *model.RefreshToken) error {
	return f.createErr
}

func (f *fakeRefreshTokenRepo) FindByToken(_ context.Context, _ string) (*model.RefreshToken, error) {
	return f.token, f.findErr
}

func (f *fakeRefreshTokenRepo) DeleteByIdentityID(_ context.Context, _ uint) error {
	return f.deleteErr
}

type fakePositionRepo struct {
	exists    bool
	existsErr error
}

func (f *fakePositionRepo) Exists(_ context.Context, _ uint) (bool, error) {
	return f.exists, f.existsErr
}

// --- Mailer Fake ---

type fakeMailer struct {
	sendErr error
	sent    bool
}

func (f *fakeMailer) Send(_, _, _ string) error {
	f.sent = true
	return f.sendErr
}

type fakeTxManager struct{}

func (m *fakeTxManager) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func testConfig() *config.Configuration {
	return &config.Configuration{
		JWTSecret:         "test-secret",
		JWTExpiry:         15,
		RefreshExpiry:     10080,
		FailedLoginWindow: 5,
		MaxFailedLogins:   4,
		URLs: config.URLConfig{
			FrontendBaseURL: "http://localhost:5173",
		},
	}
}

func hashedPassword(plain string) string {
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.MinCost)
	if err != nil {
		panic(err)
	}
	return string(h)
}

func activeIdentity() *model.Identity {
	return &model.Identity{
		ID:           1,
		Email:        "john@example.com",
		Username:     "johndoe",
		PasswordHash: hashedPassword("Password12"),
		Type:         auth.IdentityEmployee,
		Active:       true,
	}
}

func activeEmployee() *model.Employee {
	return &model.Employee{
		EmployeeID: 1,
		IdentityID: 1,
		FirstName:  "John",
		LastName:   "Doe",
		Identity:   *activeIdentity(),
		Permissions: []model.EmployeePermission{
			{EmployeeID: 1, Permission: permission.EmployeeView},
		},
	}
}

func activeSupervisor() *model.Employee {
	employee := activeEmployee()
	employee.EmployeeID = 2
	employee.IdentityID = 2
	employee.ActuaryInfo = &model.ActuaryInfo{
		EmployeeID:   2,
		IsSupervisor: true,
	}
	return employee
}

func activeAgent() *model.Employee {
	employee := activeEmployee()
	employee.ActuaryInfo = &model.ActuaryInfo{
		EmployeeID:   employee.EmployeeID,
		IsAgent:      true,
		Limit:        100000,
		UsedLimit:    15000,
		NeedApproval: true,
	}
	return employee
}

func adminEmployee() *model.Employee {
	employee := activeSupervisor()
	employee.EmployeeID = 3
	employee.IdentityID = 3
	employee.Permissions = mapPermissions(employee.EmployeeID, permission.All)
	employee.ActuaryInfo.EmployeeID = employee.EmployeeID
	return employee
}

func activeClientIdentity() *model.Identity {
	return &model.Identity{
		ID:           2,
		Email:        "client@example.com",
		Username:     "clientuser",
		PasswordHash: hashedPassword("Password12"),
		Type:         auth.IdentityClient,
		Active:       true,
	}
}

func activeClient() *model.Client {
	return &model.Client{
		ClientID:   1,
		IdentityID: 2,
		FirstName:  "Jane",
		LastName:   "Client",
		Identity:   *activeClientIdentity(),
	}
}

type fakeAuditRepo struct {
	saveErr error
}

func (f *fakeAuditRepo) Save(_ context.Context, _ *audit.AuditLog) error {
	return f.saveErr
}

func (f *fakeAuditRepo) GetAll(_ context.Context, _ string, _ *uint, _, _ *time.Time, _, _ int) ([]audit.AuditLog, int64, error) {
	return nil, 0, nil
}

func fakeAuditService(saveErr error) *audit.Service {
	return audit.NewService(&fakeAuditRepo{saveErr: saveErr})
}

func withAuth(ctx context.Context, identityID uint, identityType auth.IdentityType) context.Context {
	employeeID := identityID
	return auth.SetAuthOnContext(ctx, &auth.AuthContext{
		IdentityID:   identityID,
		IdentityType: identityType,
		EmployeeID:   &employeeID,
		Permissions:  []permission.Permission{permission.EmployeeView},
	})
}
