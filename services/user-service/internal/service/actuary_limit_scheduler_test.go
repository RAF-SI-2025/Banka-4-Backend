package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestActuaryLimitScheduler_RunDailyReset(t *testing.T) {
	t.Parallel()

	repo := &fakeActuaryRepo{}
	svc := NewActuaryService(repo, &fakeEmployeeRepo{}, nil, fakeAuditService(nil))
	scheduler := NewActuaryLimitScheduler(svc)

	// Use a context we cancel quickly to exercise the ctx.Done() path in runDailyReset.
	ctx, cancel := context.WithCancel(context.Background())
	go scheduler.runDailyReset(ctx)
	cancel() // triggers <-ctx.Done() branch
}

func TestActuaryLimitScheduler_StartStop(t *testing.T) {
	t.Parallel()

	repo := &fakeActuaryRepo{}
	svc := NewActuaryService(repo, &fakeEmployeeRepo{}, nil, fakeAuditService(nil))
	scheduler := NewActuaryLimitScheduler(svc)

	// Start the scheduler
	scheduler.Start()

	// Verify cancel is set (scheduler is running)
	scheduler.mu.Lock()
	require.NotNil(t, scheduler.cancel)
	scheduler.mu.Unlock()

	// Calling Start again should be a no-op (idempotent)
	scheduler.Start()

	// Stop the scheduler
	scheduler.Stop()

	// Verify cancel is nil (scheduler is stopped)
	scheduler.mu.Lock()
	require.Nil(t, scheduler.cancel)
	scheduler.mu.Unlock()

	// Calling Stop again should be safe (no panic)
	scheduler.Stop()
}

func TestNextActuaryReset(t *testing.T) {
	t.Parallel()

	resetTime := nextActuaryReset()
	now := time.Now()

	// Reset time must be in the future
	require.True(t, resetTime.After(now))

	// Reset time must be at 23:59:00
	require.Equal(t, 23, resetTime.Hour())
	require.Equal(t, 59, resetTime.Minute())
	require.Equal(t, 0, resetTime.Second())

	// Reset time must be within the next 24 hours
	require.True(t, resetTime.Before(now.Add(24*time.Hour+time.Second)))
}
