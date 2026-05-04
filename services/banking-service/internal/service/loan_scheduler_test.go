package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
)

// ── Fake Scheduler Loan Repository ──────────────────────────────────────────

type fakeSchedulerLoanRepo struct {
	loan                 *model.Loan
	loans                []model.Loan
	dueInstallments      []model.LoanInstallment
	retryInstallments    []model.LoanInstallment
	loanByRequestID      *model.Loan
	updatedInstallments  []*model.LoanInstallment
	updatedLoans         []*model.Loan
	findDueErr           error
	findRetryErr         error
	updateInstErr        error
	updateLoanErr        error
	findLoanByReqErr     error
	variableRateLoansErr error
}

func (f *fakeSchedulerLoanRepo) FindByClientID(_ context.Context, _ uint, _ bool) ([]model.Loan, error) {
	return f.loans, nil
}

func (f *fakeSchedulerLoanRepo) FindByIDAndClientID(_ context.Context, _ uint, _ uint) (*model.Loan, error) {
	return f.loan, nil
}

func (f *fakeSchedulerLoanRepo) HasActiveByClientID(_ context.Context, _ uint) (bool, error) {
	for _, loan := range f.loans {
		if loan.Status == model.LoanStatusActive {
			return true, nil
		}
	}

	return false, nil
}

func (f *fakeSchedulerLoanRepo) CreateLoan(_ context.Context, loan *model.Loan) error {
	loan.ID = 1
	return nil
}

func (f *fakeSchedulerLoanRepo) FindLoanByRequestID(_ context.Context, _ uint) (*model.Loan, error) {
	return f.loanByRequestID, f.findLoanByReqErr
}

func (f *fakeSchedulerLoanRepo) UpdateLoan(_ context.Context, loan *model.Loan) error {
	if f.updateLoanErr != nil {
		return f.updateLoanErr
	}
	loanCopy := *loan
	f.updatedLoans = append(f.updatedLoans, &loanCopy)
	return nil
}

func (f *fakeSchedulerLoanRepo) CreateInstallments(_ context.Context, _ []model.LoanInstallment) error {
	return nil
}

func (f *fakeSchedulerLoanRepo) FindDueInstallments(_ context.Context, _ time.Time) ([]model.LoanInstallment, error) {
	return f.dueInstallments, f.findDueErr
}

func (f *fakeSchedulerLoanRepo) FindRetryInstallments(_ context.Context, _ time.Time) ([]model.LoanInstallment, error) {
	return f.retryInstallments, f.findRetryErr
}

func (f *fakeSchedulerLoanRepo) UpdateInstallment(_ context.Context, inst *model.LoanInstallment) error {
	if f.updateInstErr != nil {
		return f.updateInstErr
	}
	instCopy := *inst
	f.updatedInstallments = append(f.updatedInstallments, &instCopy)
	return nil
}

func (f *fakeSchedulerLoanRepo) FindActiveVariableRateLoans(_ context.Context) ([]model.Loan, error) {
	return f.loans, f.variableRateLoansErr
}

// ── Fake Scheduler Mailer ───────────────────────────────────────────────────

type fakeSchedulerMailer struct {
	sentTo      []string
	sentSubject []string
	sendErr     error
}

func (f *fakeSchedulerMailer) Send(to, subject, body string) error {
	f.sentTo = append(f.sentTo, to)
	f.sentSubject = append(f.sentSubject, subject)
	return f.sendErr
}

// ── Helper ──────────────────────────────────────────────────────────────────

func newScheduler(
	loanRepo *fakeSchedulerLoanRepo,
	mailer *fakeSchedulerMailer,
	userClient *fakeUserClient,
) *LoanScheduler {
	if loanRepo == nil {
		loanRepo = &fakeSchedulerLoanRepo{}
	}
	if mailer == nil {
		mailer = &fakeSchedulerMailer{}
	}
	if userClient == nil {
		userClient = &fakeUserClient{}
	}

	accRepo := &fakeLoanAccountRepo{
		accounts: map[string]*model.Account{
			"client-account": {
				AccountNumber:    "client-account",
				AvailableBalance: 1_000_000,
				DailyLimit:       10_000_000,
				MonthlyLimit:     100_000_000,
				Currency:         model.Currency{Code: model.RSD},
			},
			BankAccounts[model.RSD]: {
				AccountNumber:    BankAccounts[model.RSD],
				AvailableBalance: 1_000_000,
				DailyLimit:       10_000_000,
				MonthlyLimit:     100_000_000,
				Currency:         model.Currency{Code: model.RSD},
			},
		},
	}

	txRepo := &fakeLoanTransactionRepo{}
	txManager := &fakeBankingTxManager{}
	txProcessor := NewTransactionProcessor(accRepo, txRepo, txManager)
	loanSvc := NewLoanService(accRepo, nil, nil, loanRepo, txProcessor, txManager, userClient, mailer)

	return NewLoanScheduler(loanRepo, accRepo, txRepo, txProcessor, txManager, mailer, userClient, loanSvc)
}

// ── nextMidnight Tests ──────────────────────────────────────────────────────

func TestNextMidnight_ReturnsTimeInFuture(t *testing.T) {
	t.Parallel()
	result := nextMidnight()
	require.True(t, result.After(time.Now()))
}

func TestNextMidnight_ReturnsTimeAtMidnight(t *testing.T) {
	t.Parallel()
	result := nextMidnight()
	require.Equal(t, 0, result.Hour())
	require.Equal(t, 0, result.Minute())
	require.Equal(t, 0, result.Second())
	require.Equal(t, 0, result.Nanosecond())
}

func TestNextMidnight_WithinNext24Hours(t *testing.T) {
	t.Parallel()
	result := nextMidnight()
	diff := time.Until(result)
	require.True(t, diff > 0)
	require.True(t, diff <= 24*time.Hour)
}

// ── nextFirstOfMonth Tests ──────────────────────────────────────────────────

func TestNextFirstOfMonth_ReturnsTimeInFuture(t *testing.T) {
	t.Parallel()
	result := nextFirstOfMonth()
	require.True(t, result.After(time.Now()))
}

func TestNextFirstOfMonth_ReturnsDayOneAtOneAM(t *testing.T) {
	t.Parallel()
	result := nextFirstOfMonth()
	require.Equal(t, 1, result.Day())
	require.Equal(t, 1, result.Hour())
	require.Equal(t, 0, result.Minute())
	require.Equal(t, 0, result.Second())
}

func TestNextFirstOfMonth_WithinNext32Days(t *testing.T) {
	t.Parallel()
	result := nextFirstOfMonth()
	diff := time.Until(result)
	require.True(t, diff > 0)
	require.True(t, diff <= 32*24*time.Hour)
}

// ── onInstallmentPaid Tests ─────────────────────────────────────────────────

func TestOnInstallmentPaid_SetsInstallmentFields(t *testing.T) {
	t.Parallel()

	loanRepo := &fakeSchedulerLoanRepo{}
	sched := newScheduler(loanRepo, nil, nil)

	loan := &model.Loan{
		ID:                  1,
		RemainingDebt:       50000,
		PaidInstallments:    2,
		RepaymentPeriod:     12,
		NextInstallmentDate: time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
		Status:              model.LoanStatusActive,
	}
	installment := &model.LoanInstallment{
		ID:     1,
		Amount: 5000,
		Status: model.InstallmentStatusPending,
	}

	err := sched.onInstallmentPaid(context.Background(), installment, loan, 42)

	require.NoError(t, err)
	require.Equal(t, model.InstallmentStatusPaid, installment.Status)
	require.NotNil(t, installment.PaidAt)
	require.NotNil(t, installment.TransactionID)
	require.Equal(t, uint(42), *installment.TransactionID)
	require.Nil(t, installment.RetryAt)
}

func TestOnInstallmentPaid_DecreasesRemainingDebt(t *testing.T) {
	t.Parallel()

	loanRepo := &fakeSchedulerLoanRepo{}
	sched := newScheduler(loanRepo, nil, nil)

	loan := &model.Loan{
		ID:                  1,
		RemainingDebt:       50000,
		PaidInstallments:    2,
		RepaymentPeriod:     12,
		NextInstallmentDate: time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
		Status:              model.LoanStatusActive,
	}
	installment := &model.LoanInstallment{
		ID:     1,
		Amount: 5000,
		Status: model.InstallmentStatusPending,
	}

	err := sched.onInstallmentPaid(context.Background(), installment, loan, 1)

	require.NoError(t, err)
	require.InDelta(t, 45000, loan.RemainingDebt, 0.01)
	require.Equal(t, 3, loan.PaidInstallments)
}

func TestOnInstallmentPaid_RemainingDebtDoesNotGoNegative(t *testing.T) {
	t.Parallel()

	loanRepo := &fakeSchedulerLoanRepo{}
	sched := newScheduler(loanRepo, nil, nil)

	loan := &model.Loan{
		ID:                  1,
		RemainingDebt:       3000,
		PaidInstallments:    11,
		RepaymentPeriod:     12,
		NextInstallmentDate: time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
		Status:              model.LoanStatusActive,
	}
	installment := &model.LoanInstallment{
		ID:     12,
		Amount: 5000,
		Status: model.InstallmentStatusPending,
	}

	err := sched.onInstallmentPaid(context.Background(), installment, loan, 1)

	require.NoError(t, err)
	require.Equal(t, 0.0, loan.RemainingDebt)
}

func TestOnInstallmentPaid_AllInstallmentsPaid_CompletesLoan(t *testing.T) {
	t.Parallel()

	loanRepo := &fakeSchedulerLoanRepo{}
	sched := newScheduler(loanRepo, nil, nil)

	loan := &model.Loan{
		ID:                  1,
		RemainingDebt:       5000,
		PaidInstallments:    11,
		RepaymentPeriod:     12,
		NextInstallmentDate: time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
		Status:              model.LoanStatusActive,
	}
	installment := &model.LoanInstallment{
		ID:     12,
		Amount: 5000,
		Status: model.InstallmentStatusPending,
	}

	err := sched.onInstallmentPaid(context.Background(), installment, loan, 1)

	require.NoError(t, err)
	require.Equal(t, model.LoanStatusCompleted, loan.Status)
	require.True(t, loan.NextInstallmentDate.IsZero())
}

func TestOnInstallmentPaid_NotAllPaid_AdvancesNextInstallmentDate(t *testing.T) {
	t.Parallel()

	loanRepo := &fakeSchedulerLoanRepo{}
	sched := newScheduler(loanRepo, nil, nil)

	originalDate := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	loan := &model.Loan{
		ID:                  1,
		RemainingDebt:       50000,
		PaidInstallments:    2,
		RepaymentPeriod:     12,
		NextInstallmentDate: originalDate,
		Status:              model.LoanStatusActive,
	}
	installment := &model.LoanInstallment{
		ID:     3,
		Amount: 5000,
		Status: model.InstallmentStatusPending,
	}

	err := sched.onInstallmentPaid(context.Background(), installment, loan, 1)

	require.NoError(t, err)
	require.Equal(t, model.LoanStatusActive, loan.Status)
	expectedDate := time.Date(2025, 7, 15, 0, 0, 0, 0, time.UTC)
	require.Equal(t, expectedDate, loan.NextInstallmentDate)
}

func TestOnInstallmentPaid_UpdateInstallmentError(t *testing.T) {
	t.Parallel()

	loanRepo := &fakeSchedulerLoanRepo{updateInstErr: fmt.Errorf("db error")}
	sched := newScheduler(loanRepo, nil, nil)

	loan := &model.Loan{
		ID:                  1,
		RemainingDebt:       50000,
		PaidInstallments:    2,
		RepaymentPeriod:     12,
		NextInstallmentDate: time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
		Status:              model.LoanStatusActive,
	}
	installment := &model.LoanInstallment{ID: 1, Amount: 5000, Status: model.InstallmentStatusPending}

	err := sched.onInstallmentPaid(context.Background(), installment, loan, 1)
	require.Error(t, err)
}

func TestOnInstallmentPaid_UpdateLoanError(t *testing.T) {
	t.Parallel()

	loanRepo := &fakeSchedulerLoanRepo{updateLoanErr: fmt.Errorf("db error")}
	sched := newScheduler(loanRepo, nil, nil)

	loan := &model.Loan{
		ID:                  1,
		RemainingDebt:       50000,
		PaidInstallments:    2,
		RepaymentPeriod:     12,
		NextInstallmentDate: time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
		Status:              model.LoanStatusActive,
	}
	installment := &model.LoanInstallment{ID: 1, Amount: 5000, Status: model.InstallmentStatusPending}

	err := sched.onInstallmentPaid(context.Background(), installment, loan, 1)
	require.Error(t, err)
}

// ── onInstallmentFailed Tests ───────────────────────────────────────────────

func TestOnInstallmentFailed_FirstFailure_SetsRetrying(t *testing.T) {
	t.Parallel()

	loanRepo := &fakeSchedulerLoanRepo{}
	mailer := &fakeSchedulerMailer{}
	sched := newScheduler(loanRepo, mailer, nil)

	loan := &model.Loan{
		ID:     1,
		Status: model.LoanStatusActive,
		LoanRequest: model.LoanRequest{
			ClientID: 1,
		},
	}
	installment := &model.LoanInstallment{
		ID:     1,
		Amount: 5000,
		Status: model.InstallmentStatusPending,
	}

	sched.onInstallmentFailed(context.Background(), installment, loan)

	require.Equal(t, model.InstallmentStatusRetrying, installment.Status)
	require.NotNil(t, installment.RetryAt)
	// RetryAt should be approximately 72 hours from now
	expectedRetryAt := time.Now().Add(retryAfter)
	require.InDelta(t, expectedRetryAt.Unix(), installment.RetryAt.Unix(), 5)
}

func TestOnInstallmentFailed_SecondFailure_SetsUnpaid(t *testing.T) {
	t.Parallel()

	loanRepo := &fakeSchedulerLoanRepo{}
	mailer := &fakeSchedulerMailer{}
	sched := newScheduler(loanRepo, mailer, nil)

	retryAt := time.Now().Add(-1 * time.Hour)
	loan := &model.Loan{
		ID:     1,
		Status: model.LoanStatusActive,
		LoanRequest: model.LoanRequest{
			ClientID: 1,
		},
	}
	installment := &model.LoanInstallment{
		ID:      1,
		Amount:  5000,
		Status:  model.InstallmentStatusRetrying,
		RetryAt: &retryAt,
	}

	sched.onInstallmentFailed(context.Background(), installment, loan)

	require.Equal(t, model.InstallmentStatusUnpaid, installment.Status)
	require.Nil(t, installment.RetryAt)
}

func TestOnInstallmentFailed_SendsNotification(t *testing.T) {
	t.Parallel()

	loanRepo := &fakeSchedulerLoanRepo{}
	mailer := &fakeSchedulerMailer{}
	userClient := &fakeUserClient{}
	sched := newScheduler(loanRepo, mailer, userClient)

	loan := &model.Loan{
		ID:     1,
		Status: model.LoanStatusActive,
		LoanRequest: model.LoanRequest{
			ClientID: 1,
		},
	}
	installment := &model.LoanInstallment{
		ID:     1,
		Amount: 5000,
		Status: model.InstallmentStatusPending,
	}

	sched.onInstallmentFailed(context.Background(), installment, loan)

	// Notification should have been sent
	require.Len(t, mailer.sentTo, 1)
}

func TestOnInstallmentFailed_UpdateInstallmentError_DoesNotPanic(t *testing.T) {
	t.Parallel()

	loanRepo := &fakeSchedulerLoanRepo{updateInstErr: fmt.Errorf("db error")}
	mailer := &fakeSchedulerMailer{}
	sched := newScheduler(loanRepo, mailer, nil)

	loan := &model.Loan{
		ID:     1,
		Status: model.LoanStatusActive,
		LoanRequest: model.LoanRequest{
			ClientID: 1,
		},
	}
	installment := &model.LoanInstallment{
		ID:     1,
		Amount: 5000,
		Status: model.InstallmentStatusPending,
	}

	// Should not panic, just logs
	sched.onInstallmentFailed(context.Background(), installment, loan)

	// Notification should NOT be sent when UpdateInstallment fails (early return)
	require.Len(t, mailer.sentTo, 0)
}

// ── sendFailureNotification Tests ───────────────────────────────────────────

func TestSendFailureNotification_RetryingStatus(t *testing.T) {
	t.Parallel()

	mailer := &fakeSchedulerMailer{}
	sched := newScheduler(nil, mailer, &fakeUserClient{})

	loan := &model.Loan{
		ID:          1,
		LoanRequest: model.LoanRequest{ClientID: 1},
	}
	installment := &model.LoanInstallment{
		Status: model.InstallmentStatusRetrying,
	}

	sched.sendFailureNotification(context.Background(), loan, installment)

	require.Len(t, mailer.sentTo, 1)
	// For retrying status, the subject should be the first-failure notification
	require.Contains(t, mailer.sentSubject[0], "Neuspesna naplata")
}

func TestSendFailureNotification_UnpaidStatus(t *testing.T) {
	t.Parallel()

	mailer := &fakeSchedulerMailer{}
	sched := newScheduler(nil, mailer, &fakeUserClient{})

	loan := &model.Loan{
		ID:          1,
		LoanRequest: model.LoanRequest{ClientID: 1},
	}
	installment := &model.LoanInstallment{
		Status: model.InstallmentStatusUnpaid,
	}

	sched.sendFailureNotification(context.Background(), loan, installment)

	require.Len(t, mailer.sentTo, 1)
	require.Contains(t, mailer.sentSubject[0], "nije naplaćena")
}

func TestSendFailureNotification_UserClientError_DoesNotPanic(t *testing.T) {
	t.Parallel()

	mailer := &fakeSchedulerMailer{}
	userClient := &fakeUserClient{clientErr: fmt.Errorf("user service unavailable")}
	sched := newScheduler(nil, mailer, userClient)

	loan := &model.Loan{
		ID:          1,
		LoanRequest: model.LoanRequest{ClientID: 1},
	}
	installment := &model.LoanInstallment{
		Status: model.InstallmentStatusRetrying,
	}

	// Should not panic, just logs
	sched.sendFailureNotification(context.Background(), loan, installment)
	require.Len(t, mailer.sentTo, 0)
}

// ── processDueInstallments Tests ────────────────────────────────────────────

func TestProcessDueInstallments_NoInstallments(t *testing.T) {
	t.Parallel()

	loanRepo := &fakeSchedulerLoanRepo{dueInstallments: []model.LoanInstallment{}}
	sched := newScheduler(loanRepo, nil, nil)

	// Should not panic
	sched.processDueInstallments(context.Background())
}

func TestProcessDueInstallments_FindDueError(t *testing.T) {
	t.Parallel()

	loanRepo := &fakeSchedulerLoanRepo{findDueErr: fmt.Errorf("db error")}
	sched := newScheduler(loanRepo, nil, nil)

	// Should not panic, just logs
	sched.processDueInstallments(context.Background())
}

// ── processRetryInstallments Tests ──────────────────────────────────────────

func TestProcessRetryInstallments_NoInstallments(t *testing.T) {
	t.Parallel()

	loanRepo := &fakeSchedulerLoanRepo{retryInstallments: []model.LoanInstallment{}}
	sched := newScheduler(loanRepo, nil, nil)

	// Should not panic
	sched.processRetryInstallments(context.Background())
}

func TestProcessRetryInstallments_FindRetryError(t *testing.T) {
	t.Parallel()

	loanRepo := &fakeSchedulerLoanRepo{findRetryErr: fmt.Errorf("db error")}
	sched := newScheduler(loanRepo, nil, nil)

	// Should not panic, just logs
	sched.processRetryInstallments(context.Background())
}

// ── processInstallment Tests ────────────────────────────────────────────────

func TestProcessInstallment_AccountNotFound(t *testing.T) {
	t.Parallel()

	loanRepo := &fakeSchedulerLoanRepo{}
	accRepo := &fakeLoanAccountRepo{account: nil}
	mailer := &fakeSchedulerMailer{}

	txRepo := &fakeLoanTransactionRepo{}
	txManager := &fakeBankingTxManager{}
	txProcessor := NewTransactionProcessor(accRepo, txRepo, txManager)
	loanSvc := NewLoanService(accRepo, nil, nil, loanRepo, txProcessor, txManager, &fakeUserClient{}, mailer)

	sched := NewLoanScheduler(loanRepo, accRepo, txRepo, txProcessor, txManager, mailer, &fakeUserClient{}, loanSvc)

	installment := &model.LoanInstallment{
		ID:     1,
		Amount: 5000,
		Status: model.InstallmentStatusPending,
		Loan: model.Loan{
			ID: 1,
			LoanRequest: model.LoanRequest{
				AccountNumber: "MISSING-ACC",
				ClientID:      1,
			},
		},
	}

	// Should not panic, just logs
	sched.processInstallment(context.Background(), installment)
}

func TestProcessInstallment_InsufficientFunds_TriggersFailure(t *testing.T) {
	t.Parallel()

	loanRepo := &fakeSchedulerLoanRepo{}
	accRepo := &fakeLoanAccountRepo{
		account: &model.Account{
			AccountNumber:    "client-account",
			AvailableBalance: 100, // not enough for 5000
			Currency:         model.Currency{Code: model.RSD},
		},
	}
	mailer := &fakeSchedulerMailer{}

	txRepo := &fakeLoanTransactionRepo{}
	txManager := &fakeBankingTxManager{}
	txProcessor := NewTransactionProcessor(accRepo, txRepo, txManager)
	loanSvc := NewLoanService(accRepo, nil, nil, loanRepo, txProcessor, txManager, &fakeUserClient{}, mailer)

	sched := NewLoanScheduler(loanRepo, accRepo, txRepo, txProcessor, txManager, mailer, &fakeUserClient{}, loanSvc)

	installment := &model.LoanInstallment{
		ID:     1,
		Amount: 5000,
		Status: model.InstallmentStatusPending,
		Loan: model.Loan{
			ID: 1,
			LoanRequest: model.LoanRequest{
				AccountNumber: "client-account",
				ClientID:      1,
			},
		},
	}

	sched.processInstallment(context.Background(), installment)

	// Installment should be set to retrying
	require.Equal(t, model.InstallmentStatusRetrying, installment.Status)
}
