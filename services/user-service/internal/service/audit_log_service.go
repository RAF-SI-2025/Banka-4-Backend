package service

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/audit"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/dto"
)

type AuditLogService struct {
	repo audit.Repository
}

func NewAuditLogService(repo audit.Repository) *AuditLogService {
	return &AuditLogService{repo: repo}
}

func (s *AuditLogService) GetAll(ctx context.Context, query *dto.ListAuditLogsQuery) (*dto.ListAuditLogsResponse, error) {
	entries, total, err := s.repo.GetAll(
		ctx,
		query.ActionType,
		query.PerformedByID,
		query.DateFrom,
		query.DateTo,
		query.Page,
		query.PageSize,
	)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	result := dto.ToAuditLogResponseList(entries, total, query.Page, query.PageSize)
	return &result, nil
}
