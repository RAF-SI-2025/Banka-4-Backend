package job

import (
	"context"
	"log"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/service"
)

// DividendPayoutJob runs on every weekday and checks whether today is the last
// business day of a dividend quarter (March, June, September, December).
// If it is, it delegates to DividendPayoutService.ProcessDividends.
type DividendPayoutJob struct {
	dividendService *service.DividendPayoutService
}

func NewDividendPayoutJob(dividendService *service.DividendPayoutService) *DividendPayoutJob {
	return &DividendPayoutJob{dividendService: dividendService}
}

func (j *DividendPayoutJob) Run(ctx context.Context) error {
	now := time.Now()

	if !isLastBusinessDayOfDividendQuarter(now) {
		return nil
	}

	log.Printf("[DividendPayoutJob] Today (%s) is the last business day of a dividend quarter — processing dividends", now.Format("2006-01-02"))

	if err := j.dividendService.ProcessDividends(ctx); err != nil {
		log.Printf("[DividendPayoutJob] error: %v", err)
		return err
	}

	log.Println("[DividendPayoutJob] Dividend payout completed successfully")
	return nil
}

// dividendQuarterEndMonths are the months whose last business day triggers payout.
var dividendQuarterEndMonths = map[time.Month]bool{
	time.March:     true,
	time.June:      true,
	time.September: true,
	time.December:  true,
}

// isLastBusinessDayOfDividendQuarter returns true when t falls on the last
// Monday–Friday of a dividend-quarter-end month.
func isLastBusinessDayOfDividendQuarter(t time.Time) bool {
	if !dividendQuarterEndMonths[t.Month()] {
		return false
	}

	// Weekends are not business days
	if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		return false
	}

	// Check if there are any more weekdays left in the month
	next := t.AddDate(0, 0, 1)
	for next.Month() == t.Month() {
		if next.Weekday() != time.Saturday && next.Weekday() != time.Sunday {
			// There is still at least one more business day in this month
			return false
		}
		next = next.AddDate(0, 0, 1)
	}

	return true
}
