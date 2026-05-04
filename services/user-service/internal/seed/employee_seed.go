package seed

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/permission"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/model"
)

var positions = []string{
	"Manager",
	"Developer",
	"HR",
	"Accountant",
	"QA",
}

var employees = []struct {
	FirstName   string
	LastName    string
	Gender      string
	DateOfBirth string
	Email       string
	PhoneNumber string
	Address     string
	Username    string
	Password    string
	Active      bool
	Department  string
	Position    string
}{
	{"Dimitrije", "Mijailovic", "M", "1985-05-01", "dimitrije@raf.rs", "123456789", "Street 1", "dimitrije", "pass123", true, "IT", "Developer"},
	{"Petar", "Petrovic", "M", "1990-08-12", "petar@raf.rs", "987654321", "Street 2", "petar", "pass123", true, "HR", "HR"},
	{"Admin", "Admin", "M", "1980-01-01", "admin@raf.rs", "000000000", "RAF", "admin", "admin123", true, "IT", "Manager"},
	{"Marko", "Markovic", "M", "1992-03-15", "marko@raf.rs", "111222333", "Street 3", "marko", "pass123", true, "IT", "Developer"},
	{"Jelena", "Jovanovic", "F", "1988-07-22", "jelena@raf.rs", "444555666", "Street 4", "jelena", "pass123", true, "Finance", "Accountant"},
	{"Nikola", "Nikolic", "M", "1995-11-30", "nikola@raf.rs", "777888999", "Street 5", "nikola", "pass123", true, "IT", "QA"},
	{"Admin", "Novi", "M", "1980-01-01", "adminnovi@raf.rs", "000000001", "RAF", "adminnovi", "admin123", true, "IT", "Manager"},
}

var activatableClients = []struct {
	FirstName   string
	LastName    string
	Gender      string
	DateOfBirth string
	Email       string
	Username    string
	PhoneNumber string
	Address     string
	Password    string
}{
	{"Test", "Client", "M", "2000-01-01", "testclient@example.com", "testclient", "+381600000001", "Test Address 1, Beograd", "test123"},
}

var clients = []struct {
	FirstName                string
	LastName                 string
	Gender                   string
	DateOfBirth              string
	Email                    string
	Username                 string
	PhoneNumber              string
	Address                  string
	Password                 string
	MobileVerificationSecret string
}{
	{"Banka", "Četiri", "M", "1992-03-15", "banka4@raf.rs", "banka4", "+381600000000", "Bankarska ulica 1, Beograd", "Banka123", "AAAAAAAAAAAAAAAAAAAA"},
	{"Marko", "Markovic", "M", "1992-03-15", "marko.markovic@example.com", "marko.markovic", "+381601234567", "Knez Mihailova 10, Beograd", "password123", "AAAAAAAAAAAAAAAAAAAA"},
	{"Ana", "Anic", "F", "1995-07-22", "ana.anic@example.com", "ana.anic", "+381609876543", "Bulevar Oslobodjenja 20, Novi Sad", "password123", "AAAAAAAAAAAAAAAAAAAA"},
	{"Stefan", "Stefanovic", "M", "1988-11-30", "stefan.stefanovic@example.com", "stefan.stefanovic", "+381611112222", "Trg Republike 5, Beograd", "password123", "AAAAAAAAAAAAAAAAAAAA"},
	{"Mirko", "Mirkovic", "F", "1995-07-22", "mirko.mirkovic@example.com", "mirko.mirkovic", "+381609876543", "Bulevar Oslobodjenja 20, Novi Sad", "password123", "AAAAAAAAAAAAAAAAAAAA"},
	{"Sekretar", "Drzavne Kase", "M", "1995-07-22", "sekretar@gov.rs", "drzavna.kasa", "+381604555888", "Beograd", "kasa123", "AAAAAAAAAAAAAAAAAAAA"},
}

func Run(db *gorm.DB) error {
	// seed positions
	positionMap := make(map[string]uint)
	for _, title := range positions {
		var pos model.Position
		err := db.Where("title = ?", title).First(&pos).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			pos = model.Position{Title: title}
			if err := db.Create(&pos).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		positionMap[title] = pos.PositionID
	}

	// seed employees
	for _, e := range employees {
		var existingIdentity model.Identity
		if err := db.Where("email = ?", e.Email).First(&existingIdentity).Error; err == nil {
			continue
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(e.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}

		dob, err := time.Parse("2006-01-02", e.DateOfBirth)
		if err != nil {
			return err
		}

		identity := model.Identity{
			Email:        e.Email,
			Username:     e.Username,
			PasswordHash: string(hash),
			Type:         auth.IdentityEmployee,
			Active:       e.Active,
		}
		if err := db.Create(&identity).Error; err != nil {
			return err
		}

		employee := model.Employee{
			IdentityID:  identity.ID,
			FirstName:   e.FirstName,
			LastName:    e.LastName,
			Gender:      e.Gender,
			DateOfBirth: dob,
			PhoneNumber: e.PhoneNumber,
			Address:     e.Address,
			Department:  e.Department,
			PositionID:  positionMap[e.Position],
		}
		if err := db.Create(&employee).Error; err != nil {
			return err
		}
	}
	// seed clients
	for _, c := range clients {
		var existingIdentity model.Identity
		if err := db.Where("email = ?", c.Email).First(&existingIdentity).Error; err == nil {
			continue
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(c.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}

		dob, err := time.Parse("2006-01-02", c.DateOfBirth)
		if err != nil {
			return err
		}

		identity := model.Identity{
			Email:        c.Email,
			Username:     c.Username,
			PasswordHash: string(hash),
			Type:         auth.IdentityClient,
			Active:       true,
		}
		if err := db.Create(&identity).Error; err != nil {
			return err
		}

		client := model.Client{
			IdentityID:               identity.ID,
			FirstName:                c.FirstName,
			LastName:                 c.LastName,
			Gender:                   c.Gender,
			DateOfBirth:              dob,
			PhoneNumber:              c.PhoneNumber,
			Address:                  c.Address,
			MobileVerificationSecret: c.MobileVerificationSecret,
		}
		if err := db.Create(&client).Error; err != nil {
			return err
		}
	}
	// seed activatable clients, activated in place
	for _, c := range activatableClients {
		var existingIdentity model.Identity
		if err := db.Where("email = ?", c.Email).First(&existingIdentity).Error; err == nil {
			continue // already seeded
		}

		dob, err := time.Parse("2006-01-02", c.DateOfBirth)
		if err != nil {
			return err
		}

		identity := model.Identity{
			Email:    c.Email,
			Username: c.Username,
			Type:     auth.IdentityClient,
			Active:   false,
		}
		if err := db.Create(&identity).Error; err != nil {
			return err
		}

		client := model.Client{
			IdentityID:  identity.ID,
			FirstName:   c.FirstName,
			LastName:    c.LastName,
			Gender:      c.Gender,
			DateOfBirth: dob,
			PhoneNumber: c.PhoneNumber,
			Address:     c.Address,
		}
		if err := db.Create(&client).Error; err != nil {
			return err
		}

		// generate and insert an activation token
		rawBytes := make([]byte, 16)
		if _, err := rand.Read(rawBytes); err != nil {
			return err
		}
		tokenStr := hex.EncodeToString(rawBytes)

		activationToken := model.ActivationToken{
			IdentityID: identity.ID,
			Token:      tokenStr,
			ExpiresAt:  time.Now().Add(24 * time.Hour),
		}
		if err := db.Create(&activationToken).Error; err != nil {
			return err
		}

		// activate in-place
		hash, err := bcrypt.GenerateFromPassword([]byte(c.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		if err := db.Model(&identity).Updates(map[string]any{
			"password_hash": string(hash),
			"active":        true,
		}).Error; err != nil {
			return err
		}
		if err := db.Delete(&activationToken).Error; err != nil {
			return err
		}
	}

	traderClientEmails := []string{
		"marko.markovic@example.com",
		"ana.anic@example.com",
		"stefan.stefanovic@example.com",
	}

	for _, email := range traderClientEmails {
		var clientIdentity model.Identity
		if err := db.Where("email = ?", email).First(&clientIdentity).Error; err != nil {
			return err
		}

		var traderClient model.Client
		if err := db.Where("identity_id = ?", clientIdentity.ID).First(&traderClient).Error; err != nil {
			return err
		}

		var existingPerms model.ClientPermission
		if err := db.Where("client_id = ?", traderClient.ClientID).First(&existingPerms).Error; err == nil {
			continue // already seeded
		}

		perm := model.ClientPermission{
			ClientID:   traderClient.ClientID,
			Permission: permission.Trading,
		}
		if err := db.FirstOrCreate(&perm, perm).Error; err != nil {
			return err
		}
	}

	marginClientEmails := []string{
		"marko.markovic@example.com",
	}

	for _, email := range marginClientEmails {
		var clientIdentity model.Identity
		if err := db.Where("email = ?", email).First(&clientIdentity).Error; err != nil {
			return err
		}

		var traderClient model.Client
		if err := db.Where("identity_id = ?", clientIdentity.ID).First(&traderClient).Error; err != nil {
			return err
		}

		var existingPerms model.ClientPermission
		if err := db.Where("client_id = ?", traderClient.ClientID).First(&existingPerms).Error; err == nil {
			continue // already seeded
		}

		perm := model.ClientPermission{
			ClientID:   traderClient.ClientID,
			Permission: permission.TradingMargin,
		}
		if err := db.FirstOrCreate(&perm, perm).Error; err != nil {
			return err
		}
	}

	adminEmails := []string{
		"admin@raf.rs",
		"adminnovi@raf.rs",
	}

	for _, email := range adminEmails {
		var adminIdentity model.Identity
		if err := db.Where("email = ?", email).First(&adminIdentity).Error; err != nil {
			return err
		}

		var admin model.Employee
		if err := db.Where("identity_id = ?", adminIdentity.ID).First(&admin).Error; err != nil {
			return err
		}

		for _, p := range permission.All {
			var existing model.EmployeePermission
			err := db.Where("employee_id = ? AND permission = ?", admin.EmployeeID, string(p)).
				First(&existing).Error

			if errors.Is(err, gorm.ErrRecordNotFound) {
				perm := model.EmployeePermission{
					EmployeeID: admin.EmployeeID,
					Permission: p,
				}
				if err := db.Create(&perm).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			}
		}

		var adminActuary model.ActuaryInfo
		err := db.Where("employee_id = ?", admin.EmployeeID).First(&adminActuary).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			adminActuary = model.ActuaryInfo{
				EmployeeID:   admin.EmployeeID,
				IsSupervisor: true,
			}
			if err := db.Create(&adminActuary).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if !adminActuary.IsSupervisor || adminActuary.IsAgent || adminActuary.NeedApproval || adminActuary.Limit != 0 || adminActuary.UsedLimit != 0 {
			adminActuary.IsAgent = false
			adminActuary.IsSupervisor = true
			adminActuary.Limit = 0
			adminActuary.UsedLimit = 0
			adminActuary.NeedApproval = false
			if err := db.Save(&adminActuary).Error; err != nil {
				return err
			}
		}

	}

	agentEmails := []string{
		"marko@raf.rs",
		"jelena@raf.rs",
		"nikola@raf.rs",
	}

	agentPermissions := []permission.Permission{
		permission.ClientView,
		permission.ClientUpdate,
		permission.Trading,
		permission.TradingMargin,
	}

	for _, email := range agentEmails {
		var identity model.Identity
		if err := db.Where("email = ?", email).First(&identity).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return err
		}

		var employee model.Employee
		err := db.Where("identity_id = ?", identity.ID).First(&employee).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		} else if err != nil {
			return err
		}

		var existing model.ActuaryInfo
		err = db.Where("employee_id = ?", employee.EmployeeID).First(&existing).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			actuary := model.ActuaryInfo{
				EmployeeID:   employee.EmployeeID,
				IsAgent:      true,
				IsSupervisor: false,
				Limit:        100000,
			}
			if err := db.Create(&actuary).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			existing.IsAgent = true
			existing.IsSupervisor = false
			if err := db.Save(&existing).Error; err != nil {
				return err
			}
		}

		for _, perm := range agentPermissions {
			var existingPerm model.EmployeePermission
			err := db.Where("employee_id = ? AND permission = ?", employee.EmployeeID, string(perm)).
				First(&existingPerm).Error

			if errors.Is(err, gorm.ErrRecordNotFound) {
				newPerm := model.EmployeePermission{
					EmployeeID: employee.EmployeeID,
					Permission: perm,
				}
				if err := db.Create(&newPerm).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			}
		}
	}
	return nil
}
