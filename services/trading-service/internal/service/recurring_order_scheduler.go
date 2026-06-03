package service

import (
	"context"
	"log"
	"math"
	"sync"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
)

const recurringOrderPollInterval = time.Minute

type RecurringOrderScheduler struct {
	recurringOrderRepo repository.RecurringOrderRepository
	orderService       *OrderService
	userClient         client.UserServiceClient
	mailer             Mailer

	mu     sync.Mutex
	cancel context.CancelFunc
}

func NewRecurringOrderScheduler(
	recurringOrderRepo repository.RecurringOrderRepository,
	orderService *OrderService,
	userClient client.UserServiceClient,
	mailer Mailer,
) *RecurringOrderScheduler {
	return &RecurringOrderScheduler{
		recurringOrderRepo: recurringOrderRepo,
		orderService:       orderService,
		userClient:         userClient,
		mailer:             mailer,
	}
}

func (s *RecurringOrderScheduler) Start() {
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.mu.Unlock()

	ticker := time.NewTicker(recurringOrderPollInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.processDueRecurringOrders(ctx)
			}
		}
	}()
}

func (s *RecurringOrderScheduler) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

func (s *RecurringOrderScheduler) processDueRecurringOrders(ctx context.Context) {
	now := time.Now()
	log.Printf("[RecurringOrderScheduler] processDueRecurringOrders started at %s", now.Format(time.RFC3339))

	orders, err := s.recurringOrderRepo.FindDue(ctx, now)
	if err != nil {
		log.Printf("[RecurringOrderScheduler] FindDue error: %v", err)
		return
	}

	for i := range orders {
		s.processRecurringOrder(ctx, &orders[i])
	}

	log.Printf("[RecurringOrderScheduler] processDueRecurringOrders done, processed %d orders", len(orders))
}

func (s *RecurringOrderScheduler) processRecurringOrder(ctx context.Context, ro *model.RecurringOrder) {
	quantity, ok := s.resolveQuantity(ro)
	if !ok {
		log.Printf("[RecurringOrderScheduler] recurring order %d: cannot resolve quantity (price unavailable), will retry next tick", ro.RecurringOrderID)
		return
	}

	if quantity == 0 {
		log.Printf("[RecurringOrderScheduler] recurring order %d: quantity is 0, skipping", ro.RecurringOrderID)
		s.sendSkippedNotification(ctx, ro)
		s.advanceNextRun(ctx, ro)
		return
	}

	_, err := s.orderService.CreateSystemOrder(ctx, SystemOrderParams{
		AccountNumber: ro.AccountNumber,
		ListingID:     ro.ListingID,
		Direction:     ro.Direction,
		Quantity:      quantity,
		OwnerUserID:   ro.UserID,
		OwnerType:     ro.OwnerType,
	})
	if err != nil {
		log.Printf("[RecurringOrderScheduler] recurring order %d: CreateSystemOrder failed: %v", ro.RecurringOrderID, err)
		s.sendSkippedNotification(ctx, ro)
		s.advanceNextRun(ctx, ro)
		return
	}

	log.Printf("[RecurringOrderScheduler] recurring order %d: market order created, quantity=%d", ro.RecurringOrderID, quantity)
	s.advanceNextRun(ctx, ro)
}

func (s *RecurringOrderScheduler) resolveQuantity(ro *model.RecurringOrder) (uint, bool) {
	if ro.Mode == model.RecurringOrderModeByQuantity {
		return uint(math.Round(ro.Value)), true
	}

	if ro.Listing.ListingID == 0 {
		return 0, false
	}

	askPrice := ro.Listing.Ask
	if askPrice <= 0 {
		return 0, false
	}

	quantity := uint(math.Floor(ro.Value / askPrice))
	return quantity, true
}

func (s *RecurringOrderScheduler) advanceNextRun(ctx context.Context, ro *model.RecurringOrder) {
	ro.NextRun = nextRunTime(ro.Cadence, ro.NextRun)
	ro.UpdatedAt = time.Now()

	if err := s.recurringOrderRepo.Save(ctx, ro); err != nil {
		log.Printf("[RecurringOrderScheduler] recurring order %d: failed to advance NextRun: %v", ro.RecurringOrderID, err)
	}
}

func (s *RecurringOrderScheduler) sendSkippedNotification(ctx context.Context, ro *model.RecurringOrder) {
	email, err := s.resolveUserEmail(ctx, ro)
	if err != nil {
		log.Printf("[RecurringOrderScheduler] recurring order %d: failed to resolve user email: %v", ro.RecurringOrderID, err)
		return
	}

	subject := "Trajni nalog nije izvršen"
	body := "Vaš trajni nalog nije mogao biti izvršen zbog nedovoljnih sredstava na računu ili nedostupnosti hartije. " +
		"Sledeći pokušaj će biti izvršen u skladu sa zadatim intervalom."

	if err := s.mailer.Send(email, subject, body); err != nil {
		log.Printf("[RecurringOrderScheduler] recurring order %d: failed to send notification: %v", ro.RecurringOrderID, err)
	}
}

func (s *RecurringOrderScheduler) resolveUserEmail(ctx context.Context, ro *model.RecurringOrder) (string, error) {
	if ro.OwnerType == model.OwnerTypeClient {
		resp, err := s.userClient.GetClientById(ctx, uint64(ro.UserID))
		if err != nil {
			return "", err
		}
		return resp.Email, nil
	}

	resp, err := s.userClient.GetEmployeeById(ctx, uint64(ro.UserID))
	if err != nil {
		return "", err
	}
	return resp.Email, nil
}
