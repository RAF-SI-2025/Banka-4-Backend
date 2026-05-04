package service

import (
	"context"
	"log"
	"sync"
	"time"
)

type TaxScheduler struct {
	taxService *TaxService
	mu         sync.Mutex
	cancel     context.CancelFunc
}

func NewTaxScheduler(taxService *TaxService) *TaxScheduler {
	return &TaxScheduler{
		taxService: taxService,
	}
}

func (s *TaxScheduler) Start() {
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.mu.Unlock()

	go s.runMonthlyTaxCollectionJob(ctx)
}

func (s *TaxScheduler) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

func nextFirstOfMonth() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month()+1, 1, 1, 0, 0, 0, now.Location())
}

func (s *TaxScheduler) runMonthlyTaxCollectionJob(ctx context.Context) {
	for {
		timer := time.NewTimer(time.Until(nextFirstOfMonth()))

		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			log.Printf("[TaxScheduler] collecting taxes")
			if err := s.taxService.CollectTaxes(ctx); err != nil {
				log.Printf("[TaxScheduler] CollectTaxes error: %v", err)
			}
		}
	}
}
