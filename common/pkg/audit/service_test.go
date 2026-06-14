package audit

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeAuditRepo struct {
	savedEntry *AuditLog
	saveErr    error
	getEntries []AuditLog
	getTotal   int64
	getErr     error
}

func (f *fakeAuditRepo) Save(_ context.Context, entry *AuditLog) error {
	f.savedEntry = entry
	return f.saveErr
}

func (f *fakeAuditRepo) GetAll(_ context.Context, _ string, _ *uint, _, _ *time.Time, _, _ int) ([]AuditLog, int64, error) {
	return f.getEntries, f.getTotal, f.getErr
}

func TestServiceLogBuildsAuditEntry(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditRepo{}
	svc := NewService(repo)

	if err := svc.Log(context.Background(), ActionOrderApproved, 42, "order 7"); err != nil {
		t.Fatalf("log audit entry: %v", err)
	}

	if repo.savedEntry == nil {
		t.Fatal("expected repository Save to be called")
	}
	if repo.savedEntry.ActionType != ActionOrderApproved {
		t.Fatalf("unexpected action type %q", repo.savedEntry.ActionType)
	}
	if repo.savedEntry.PerformedByEmployeeID != 42 {
		t.Fatalf("unexpected employee id %d", repo.savedEntry.PerformedByEmployeeID)
	}
	if repo.savedEntry.Details != "order 7" {
		t.Fatalf("unexpected details %q", repo.savedEntry.Details)
	}
}

func TestServiceLogPropagatesRepositoryError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("write failed")
	svc := NewService(&fakeAuditRepo{saveErr: wantErr})

	if err := svc.Log(context.Background(), ActionOrderDeclined, 9, "order 8"); !errors.Is(err, wantErr) {
		t.Fatalf("expected repository error, got %v", err)
	}
}

func TestServiceGetAllDelegatesToRepository(t *testing.T) {
	t.Parallel()

	entries := []AuditLog{{ID: 1, ActionType: ActionTaxCollectionTriggered}}
	svc := NewService(&fakeAuditRepo{getEntries: entries, getTotal: 3})

	got, total, err := svc.GetAll(context.Background(), "", nil, nil, nil, 1, 10)
	if err != nil {
		t.Fatalf("get audit logs: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total 3, got %d", total)
	}
	if len(got) != 1 || got[0].ActionType != ActionTaxCollectionTriggered {
		t.Fatalf("unexpected entries %#v", got)
	}
}
