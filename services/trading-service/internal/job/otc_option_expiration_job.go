package job

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/service"
)

type OtcOptionExpirationJob struct {
	contractRepo repository.OtcOptionContractRepository
	userClient   client.UserServiceClient
	mailer       service.Mailer
	now          func() time.Time
}

func NewOtcOptionExpirationJob(
	contractRepo repository.OtcOptionContractRepository,
	userClient client.UserServiceClient,
	mailer service.Mailer,
) *OtcOptionExpirationJob {
	return &OtcOptionExpirationJob{
		contractRepo: contractRepo,
		userClient:   userClient,
		mailer:       mailer,
		now:          time.Now,
	}
}

func (j *OtcOptionExpirationJob) Run(ctx context.Context) error {
	log.Println("Starting OTC Option Expiration Job")

	targetDate := j.now().AddDate(0, 0, 3)

	contracts, err := j.contractRepo.FindExpiringContracts(ctx, targetDate)
	if err != nil {
		log.Printf("error fetching expiring contracts: %v", err)
		return err
	}

	for _, c := range contracts {
		users, err := j.userClient.GetClientsByIds(ctx, []uint64{
			uint64(c.BuyerID),
			uint64(c.SellerID),
		})
		if err != nil {
			log.Printf("error fetching users for contract %d: %v", c.OtcOptionContractID, err)
			continue
		}

		for _, u := range users.Clients {
			_ = j.mailer.Send(
				u.Email,
				"OTC Contract Expiring Soon",
				fmt.Sprintf("Contract #%d expires in 3 days.", c.OtcOptionContractID),
			)
		}
	}

	log.Println("OTC Option Expiration Job completed")
	return nil
}
