package job

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/repository"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/service"
)

// ContractExpiryJob marks ACTIVE contracts and ONGOING negotiations whose
// settlement date has passed as expired, and releases any share reservations
// held on behalf of expired contracts.
type ContractExpiryJob struct {
	contracts    repository.PeerContractRepository
	negotiations repository.PeerNegotiationRepository
	trading      client.TradingClient
	pollEvery    time.Duration
	stop         chan struct{}
}

func NewContractExpiryJob(
	contracts repository.PeerContractRepository,
	negotiations repository.PeerNegotiationRepository,
	trading client.TradingClient,
) *ContractExpiryJob {
	return &ContractExpiryJob{
		contracts:    contracts,
		negotiations: negotiations,
		trading:      trading,
		pollEvery:    time.Hour,
		stop:         make(chan struct{}),
	}
}

func (j *ContractExpiryJob) Start() {
	go j.loop()
}

func (j *ContractExpiryJob) Stop() {
	close(j.stop)
}

func (j *ContractExpiryJob) loop() {
	ticker := time.NewTicker(j.pollEvery)
	defer ticker.Stop()
	for {
		select {
		case <-j.stop:
			return
		case <-ticker.C:
			j.run(context.Background())
		}
	}
}

func (j *ContractExpiryJob) run(ctx context.Context) {
	// Settlement dates are free-form ISO-8601 strings, so we fetch the live
	// (ACTIVE / ONGOING) rows and decide expiry in Go via SettlementPassed,
	// which correctly handles both date-only and timezoned datetime values.
	active, err := j.contracts.FindActive(ctx)
	if err != nil {
		zap.L().Error("contract_expiry: FindActive failed", zap.Error(err))
	} else {
		for i := range active {
			if service.SettlementPassed(active[i].SettlementDate) {
				j.expireContract(ctx, &active[i])
			}
		}
	}

	ongoing, err := j.negotiations.FindOngoing(ctx)
	if err != nil {
		zap.L().Error("contract_expiry: FindOngoing failed", zap.Error(err))
	} else {
		for i := range ongoing {
			if service.SettlementPassed(ongoing[i].SettlementDate) {
				j.expireNegotiation(ctx, &ongoing[i])
			}
		}
	}
}

func (j *ContractExpiryJob) expireContract(ctx context.Context, c *model.PeerContract) {
	contractKey := fmt.Sprintf("%d:%s", c.AuthorityRoutingNumber, c.ID)

	// Release the seller's share reservation. Idempotent — errors are logged
	// and the job retries on the next tick (contract is still ACTIVE until
	// both the release and the status update succeed).
	if _, err := j.trading.ReleasePeerOtcShares(ctx, contractKey); err != nil {
		zap.L().Error("contract_expiry: ReleasePeerOtcShares failed",
			zap.String("contract", contractKey), zap.Error(err))
		return
	}

	c.Status = model.PeerContractExpired
	if err := j.contracts.Update(ctx, c); err != nil {
		zap.L().Error("contract_expiry: failed to mark contract expired",
			zap.String("contract", contractKey), zap.Error(err))
	}
}

func (j *ContractExpiryJob) expireNegotiation(ctx context.Context, n *model.PeerNegotiation) {
	n.Status = model.PeerNegotiationExpired
	if err := j.negotiations.Update(ctx, n); err != nil {
		zap.L().Error("contract_expiry: failed to mark negotiation expired",
			zap.String("id", n.ID), zap.Error(err))
	}
}
