package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/permission"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/model"
)

func setupUserRepoDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	database, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(
		&model.Identity{},
		&model.Client{},
		&model.ClientPermission{},
		&model.Position{},
		&model.Employee{},
		&model.EmployeePermission{},
		&model.ActuaryInfo{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return database
}

func TestIdentityRepositoryCreateFindUpdateAndExists(t *testing.T) {
	t.Parallel()

	repo := NewIdentityRepository(setupUserRepoDB(t))
	ctx := context.Background()

	identity := &model.Identity{
		Email:        "client@example.com",
		Username:     "client1",
		PasswordHash: "hash",
		Type:         auth.IdentityClient,
		Active:       false,
	}
	if err := repo.Create(ctx, identity); err != nil {
		t.Fatalf("create identity: %v", err)
	}

	byID, err := repo.FindByID(ctx, identity.ID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if byID == nil || byID.Email != identity.Email {
		t.Fatalf("unexpected identity by id %#v", byID)
	}
	byEmail, err := repo.FindByEmail(ctx, "client@example.com")
	if err != nil {
		t.Fatalf("find by email: %v", err)
	}
	if byEmail == nil || byEmail.Username != "client1" {
		t.Fatalf("unexpected identity by email %#v", byEmail)
	}
	byUsername, err := repo.FindByUsername(ctx, "client1")
	if err != nil {
		t.Fatalf("find by username: %v", err)
	}
	if byUsername == nil || byUsername.ID != identity.ID {
		t.Fatalf("unexpected identity by username %#v", byUsername)
	}

	byID.Active = true
	if err := repo.Update(ctx, byID); err != nil {
		t.Fatalf("update identity: %v", err)
	}
	exists, err := repo.EmailExists(ctx, "client@example.com")
	if err != nil {
		t.Fatalf("email exists: %v", err)
	}
	if !exists {
		t.Fatal("expected email to exist")
	}
	exists, err = repo.UsernameExists(ctx, "missing")
	if err != nil {
		t.Fatalf("username exists: %v", err)
	}
	if exists {
		t.Fatal("did not expect missing username to exist")
	}

	missing, err := repo.FindByID(ctx, 9999)
	if err != nil {
		t.Fatalf("find missing: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected missing identity nil, got %#v", missing)
	}
}

func TestClientRepositoryCreateFindUpdateAndList(t *testing.T) {
	t.Parallel()

	database := setupUserRepoDB(t)
	identityRepo := NewIdentityRepository(database)
	clientRepo := NewClientRepository(database)
	ctx := context.Background()

	identity := &model.Identity{Email: "client@example.com", Username: "client1", Type: auth.IdentityClient, Active: true}
	if err := identityRepo.Create(ctx, identity); err != nil {
		t.Fatalf("create identity: %v", err)
	}
	client := &model.Client{
		IdentityID: identity.ID,
		FirstName:  "Ana",
		LastName:   "Client",
		Permissions: []model.ClientPermission{
			{Permission: permission.ClientView},
		},
	}
	if err := clientRepo.Create(ctx, client); err != nil {
		t.Fatalf("create client: %v", err)
	}

	byIdentity, err := clientRepo.FindByIdentityID(ctx, identity.ID)
	if err != nil {
		t.Fatalf("find by identity: %v", err)
	}
	if byIdentity == nil || byIdentity.Identity.Email != identity.Email || !byIdentity.HasPermission(permission.ClientView) {
		t.Fatalf("unexpected client by identity %#v", byIdentity)
	}
	byID, err := clientRepo.FindByID(ctx, client.ClientID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if byID == nil || byID.FirstName != "Ana" {
		t.Fatalf("unexpected client by id %#v", byID)
	}
	byIDs, err := clientRepo.FindByIDs(ctx, []uint{client.ClientID, 9999})
	if err != nil {
		t.Fatalf("find by ids: %v", err)
	}
	if len(byIDs) != 1 || byIDs[0].ClientID != client.ClientID {
		t.Fatalf("unexpected clients by ids %#v", byIDs)
	}

	byID.LastName = "Updated"
	byID.Permissions = []model.ClientPermission{{ClientID: byID.ClientID, Permission: permission.ClientUpdate}}
	if err := clientRepo.Update(ctx, byID); err != nil {
		t.Fatalf("update client: %v", err)
	}
	updated, err := clientRepo.FindByID(ctx, client.ClientID)
	if err != nil {
		t.Fatalf("find updated: %v", err)
	}
	if updated.LastName != "Updated" || !updated.HasPermission(permission.ClientUpdate) || updated.HasPermission(permission.ClientView) {
		t.Fatalf("unexpected updated client %#v", updated)
	}

	all, total, err := clientRepo.FindAll(ctx, &dto.ListClientsQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("find all clients: %v", err)
	}
	if total != 1 || len(all) != 1 {
		t.Fatalf("unexpected client list total=%d rows=%#v", total, all)
	}

	empty, err := clientRepo.FindByIDs(ctx, nil)
	if err != nil {
		t.Fatalf("find empty ids: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected empty id lookup, got %#v", empty)
	}
}

func TestEmployeeRepositoryCreateFindUpdateAndList(t *testing.T) {
	t.Parallel()

	database := setupUserRepoDB(t)
	identityRepo := NewIdentityRepository(database)
	employeeRepo := NewEmployeeRepository(database)
	ctx := context.Background()

	position := model.Position{Title: "Trader"}
	if err := database.Create(&position).Error; err != nil {
		t.Fatalf("create position: %v", err)
	}
	identity := &model.Identity{Email: "employee@example.com", Username: "employee1", Type: auth.IdentityEmployee, Active: true}
	if err := identityRepo.Create(ctx, identity); err != nil {
		t.Fatalf("create identity: %v", err)
	}
	employee := &model.Employee{
		IdentityID:  identity.ID,
		FirstName:   "Marko",
		LastName:    "Employee",
		Department:  "Trading",
		PositionID:  position.PositionID,
		Permissions: []model.EmployeePermission{{Permission: permission.EmployeeView}},
		ActuaryInfo: &model.ActuaryInfo{IsAgent: true, Limit: 1000},
	}
	if err := employeeRepo.Create(ctx, employee); err != nil {
		t.Fatalf("create employee: %v", err)
	}

	byID, err := employeeRepo.FindByID(ctx, employee.EmployeeID)
	if err != nil {
		t.Fatalf("find employee by id: %v", err)
	}
	if byID == nil || byID.ActuaryInfo == nil || !byID.IsAgent() || !byID.HasPermission(permission.EmployeeView) {
		t.Fatalf("unexpected employee by id %#v", byID)
	}
	byIdentity, err := employeeRepo.FindByIdentityID(ctx, identity.ID)
	if err != nil {
		t.Fatalf("find employee by identity: %v", err)
	}
	if byIdentity == nil || byIdentity.Identity.Email != identity.Email {
		t.Fatalf("unexpected employee by identity %#v", byIdentity)
	}

	byID.FirstName = "Milan"
	byID.Permissions = []model.EmployeePermission{{EmployeeID: byID.EmployeeID, Permission: permission.EmployeeUpdate}}
	byID.ActuaryInfo = &model.ActuaryInfo{EmployeeID: byID.EmployeeID, IsSupervisor: true, Limit: 2000}
	if err := employeeRepo.Update(ctx, byID); err != nil {
		t.Fatalf("update employee: %v", err)
	}
	updated, err := employeeRepo.FindByID(ctx, employee.EmployeeID)
	if err != nil {
		t.Fatalf("find updated employee: %v", err)
	}
	if updated.FirstName != "Milan" || !updated.IsSupervisor() || !updated.HasPermission(permission.EmployeeUpdate) || updated.HasPermission(permission.EmployeeView) {
		t.Fatalf("unexpected updated employee %#v", updated)
	}

	rows, total, err := employeeRepo.GetAll(ctx, "", "", "", "", 1, 10)
	if err != nil {
		t.Fatalf("get all employees: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].Position.Title != "Trader" {
		t.Fatalf("unexpected employee list total=%d rows=%#v", total, rows)
	}

	missing, err := employeeRepo.FindByID(ctx, 9999)
	if err != nil {
		t.Fatalf("find missing employee: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected missing employee nil, got %#v", missing)
	}
}
