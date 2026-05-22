package service

import (
	"context"
	stdErrors "errors"
	"fmt"

	commonauth "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/audit"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/repository"
	"gorm.io/gorm"
)

type ActuaryService struct {
	actuaryRepo   repository.ActuaryRepository
	employeeRepo  repository.EmployeeRepository
	tradingClient client.TradingClient
	auditRepo     audit.Repository
}

func NewActuaryService(
	actuaryRepo repository.ActuaryRepository,
	employeeRepo repository.EmployeeRepository,
	tradingClient client.TradingClient,
	auditRepo audit.Repository,
) *ActuaryService {
	return &ActuaryService{
		actuaryRepo:   actuaryRepo,
		employeeRepo:  employeeRepo,
		tradingClient: tradingClient,
		auditRepo:     auditRepo,
	}
}
func (s *ActuaryService) GetAllActuaries(ctx context.Context, query *dto.ListActuariesQuery) (*dto.ListActuariesResponse, error) {
	employees, total, err := s.actuaryRepo.GetAll(
		ctx,
		query.Email,
		query.FirstName,
		query.LastName,
		query.Position,
		query.Department,
		query.Type,
		query.Active,
		query.NeedApproval,
		query.Page,
		query.PageSize,
	)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	return dto.ToActuaryResponseList(employees, total, query.Page, query.PageSize), nil
}

func (s *ActuaryService) UpdateActuarySettings(ctx context.Context, employeeID uint, callerID uint, req *dto.UpdateActuarySettingsRequest) (*dto.ActuaryResponse, error) {
	employee, err := s.employeeRepo.FindByID(ctx, employeeID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if employee == nil {
		return nil, errors.NotFoundErr("employee not found")
	}

	wasSupervisor := employee.IsSupervisor()

	actuary := employee.ActuaryInfo

	if req.Limit != nil || req.NeedApproval != nil {
		if !employee.IsAgent() {
			return nil, errors.BadRequestErr("only agents have configurable limits")
		}
	}

	if actuary == nil {
		return nil, errors.BadRequestErr("employee has no actuary info")
	}

	if req.Limit != nil {
		actuary.Limit = *req.Limit
	}
	if req.NeedApproval != nil {
		actuary.NeedApproval = *req.NeedApproval
	}
	if req.IsAgent != nil {
		actuary.IsAgent = *req.IsAgent
	}
	if req.IsSupervisor != nil {
		actuary.IsSupervisor = *req.IsSupervisor
	}

	if err := s.actuaryRepo.Save(ctx, actuary); err != nil {
		return nil, errors.InternalErr(err)
	}
	employee.ActuaryInfo = actuary

	if req.Limit != nil {
		if err := s.auditRepo.Save(ctx, &audit.AuditLog{
			ActionType:    audit.ActionActuaryLimitChanged,
			PerformedByID: callerID,
			Details:       fmt.Sprintf("agent_id=%d new_limit=%.2f", employeeID, *req.Limit),
		}); err != nil {
			return nil, errors.InternalErr(err)
		}
	}

	if wasSupervisor && !employee.IsSupervisor() {
		if _, err := s.tradingClient.TransferFunds(ctx, employeeID, callerID); err != nil {
			return nil, errors.InternalErr(err)
		}
	}

	return dto.ToActuaryResponse(employee), nil
}

func (s *ActuaryService) IncrementUsedLimit(ctx context.Context, employeeID uint, amount float64) (float64, error) {
	if amount <= 0 {
		return 0, errors.BadRequestErr("amount must be positive")
	}

	employee, err := s.employeeRepo.FindByID(ctx, employeeID)
	if err != nil {
		return 0, errors.InternalErr(err)
	}
	if employee == nil {
		return 0, errors.NotFoundErr("employee not found")
	}
	if !employee.IsAgent() {
		return 0, errors.BadRequestErr("only agents have used limits")
	}

	actuary, err := s.actuaryRepo.IncrementUsedLimit(ctx, employeeID, amount)
	if err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return 0, errors.NotFoundErr("actuary info not found")
		}
		return 0, errors.InternalErr(err)
	}
	if actuary == nil {
		return 0, errors.NotFoundErr("actuary info not found")
	}

	return actuary.UsedLimit, nil
}

func (s *ActuaryService) ResetUsedLimit(ctx context.Context, employeeID uint) (*dto.ActuaryResponse, error) {
	employee, err := s.employeeRepo.FindByID(ctx, employeeID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if employee == nil {
		return nil, errors.NotFoundErr("employee not found")
	}
	if !employee.IsAgent() {
		return nil, errors.BadRequestErr("only agents have used limits")
	}

	if err := s.actuaryRepo.ResetUsedLimit(ctx, employeeID); err != nil {
		return nil, errors.InternalErr(err)
	}

	if authCtx := commonauth.GetAuthFromContext(ctx); authCtx != nil && authCtx.EmployeeID != nil {
		if err := s.auditRepo.Save(ctx, &audit.AuditLog{
			ActionType:    audit.ActionActuaryLimitReset,
			PerformedByID: *authCtx.EmployeeID,
			Details:       fmt.Sprintf("agent_id=%d", employeeID),
		}); err != nil {
			return nil, errors.InternalErr(err)
		}
	}

	employee.ActuaryInfo.UsedLimit = 0
	return dto.ToActuaryResponse(employee), nil
}

func (s *ActuaryService) ResetAllUsedLimits(ctx context.Context) error {
	if err := s.actuaryRepo.ResetAllUsedLimits(ctx); err != nil {
		return errors.InternalErr(err)
	}

	return nil
}
