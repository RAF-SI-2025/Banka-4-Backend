package job

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/service"
)

type FundRedemptionJob struct {
	fundService *service.InvestmentFundService
}

func NewFundRedemptionJob(fundService *service.InvestmentFundService) *FundRedemptionJob {
	return &FundRedemptionJob{fundService: fundService}
}

func (j *FundRedemptionJob) Run(ctx context.Context) error {
	return j.fundService.ProcessPendingRedemptions(ctx)
}
