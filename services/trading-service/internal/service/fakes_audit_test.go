package service

import (
	"context"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/audit"
)

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
