package job

import (
	"context"
	"log"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/service"
)

type FundHistoryJob struct {
	fundService *service.InvestmentFundService
}

func NewFundHistoryJob(fundService *service.InvestmentFundService) *FundHistoryJob {
	return &FundHistoryJob{
		fundService: fundService,
	}
}

func (j *FundHistoryJob) Run(ctx context.Context) error {
	log.Println("Starting Fund History Job")

	err := j.fundService.CalculateAndSaveDailyHistory(ctx)
	if err != nil {
		log.Printf("Error calculating fund history: %v", err)
		return err
	}

	log.Println("Fund History Job completed successfully")
	return nil
}
