package auth

import (
	"github.com/gin-gonic/gin"

	commonauth "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/repository"
)

// RequireSupervisor checks that the authenticated employee is a supervisor.
// Must run after common auth middleware.
func RequireSupervisor(employeeRepo repository.EmployeeRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		employee, err := currentEmployee(c, employeeRepo)
		if err != nil {
			abortWithError(c, err)
			return
		}

		if !employee.IsSupervisor() {
			abortWithError(c, errors.ForbiddenErr("only supervisors can manage actuaries"))
			return
		}

		c.Next()
	}
}

func currentEmployee(c *gin.Context, employeeRepo repository.EmployeeRepository) (*model.Employee, error) {
	authCtx := commonauth.GetAuth(c)
	if authCtx == nil {
		return nil, errors.UnauthorizedErr("not authenticated")
	}

	var (
		employee *model.Employee
		err      error
	)

	if authCtx.EmployeeID != nil {
		employee, err = employeeRepo.FindByID(c.Request.Context(), *authCtx.EmployeeID)
	} else {
		employee, err = employeeRepo.FindByIdentityID(c.Request.Context(), authCtx.IdentityID)
	}

	if err != nil {
		return nil, errors.InternalErr(err)
	}

	if employee == nil {
		return nil, errors.NotFoundErr("employee not found")
	}

	return employee, nil
}

func abortWithError(c *gin.Context, err error) {
	_ = c.Error(err)
	c.Abort()
}
