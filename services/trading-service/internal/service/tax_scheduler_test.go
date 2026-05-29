package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/config"
)

// ── nextFirstOfMonth Tests ─────────────────────────────────────────

func TestNextFirstOfMonth_IsFirstOfNextMonth(t *testing.T) {
	result := nextFirstOfMonth()
	now := time.Now()

	// result must be in the future
	require.True(t, result.After(now), "nextFirstOfMonth should be after now")

	// result must be the 1st day of the month
	require.Equal(t, 1, result.Day())

	// result must be in the next calendar month relative to now
	expectedMonth := now.Month() + 1
	expectedYear := now.Year()
	if expectedMonth > 12 {
		expectedMonth = 1
		expectedYear++
	}
	require.Equal(t, expectedMonth, result.Month())
	require.Equal(t, expectedYear, result.Year())

	// hour should be 1
	require.Equal(t, 1, result.Hour())
}

// ── NewTaxScheduler Tests ──────────────────────────────────────────

func TestNewTaxScheduler_ReturnsNonNil(t *testing.T) {
	taxSvc := newTestTaxService(&fakeTaxRepo{}, &fakeBankingClient{})
	scheduler := NewTaxScheduler(taxSvc)
	require.NotNil(t, scheduler)
}

// ── Start / Stop lifecycle Tests ───────────────────────────────────

func TestTaxScheduler_StartStop(t *testing.T) {
	taxSvc := newTestTaxService(&fakeTaxRepo{}, &fakeBankingClient{})
	scheduler := NewTaxScheduler(taxSvc)

	// Start should not panic
	scheduler.Start()

	// Double-start should be a no-op (idempotent)
	scheduler.Start()

	// Stop should cancel the context
	scheduler.Stop()

	// Double-stop should also be safe
	scheduler.Stop()
}

func TestTaxScheduler_StopWithoutStart(t *testing.T) {
	taxSvc := newTestTaxService(&fakeTaxRepo{}, &fakeBankingClient{})
	scheduler := NewTaxScheduler(taxSvc)

	// Stopping without starting should not panic
	scheduler.Stop()
}

func TestTaxScheduler_StartSetsCancel(t *testing.T) {
	taxSvc := newTestTaxService(&fakeTaxRepo{}, &fakeBankingClient{})
	scheduler := NewTaxScheduler(taxSvc)

	scheduler.Start()
	// After start, cancel should be set (we can verify by calling Stop which reads it)
	scheduler.Stop()

	// After stop, cancel should be nil; a second stop is safe
	scheduler.Stop()
}

// ── TaxScheduler with a real TaxService constructor ────────────────

func TestNewTaxScheduler_WithRealTaxService(t *testing.T) {
	repo := &fakeTaxRepo{}
	banking := &fakeBankingClient{}
	taxSvc := NewTaxService(repo, banking, &config.Configuration{
		TaxAccountNumber: "444000000000000000",
	}, fakeAuditService(nil))
	scheduler := NewTaxScheduler(taxSvc)
	require.NotNil(t, scheduler)
	require.NotNil(t, scheduler.taxService)
}
