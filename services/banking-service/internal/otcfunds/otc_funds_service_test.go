package otcfunds_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/service"
)

type fakeAccountRepo struct {
	accounts map[string]*model.Account
}

func newFakeAccountRepo(accounts ...*model.Account) *fakeAccountRepo {
	m := make(map[string]*model.Account, len(accounts))
	for _, account := range accounts {
		copy := *account
		m[account.AccountNumber] = &copy
	}
	return &fakeAccountRepo{accounts: m}
}

func (r *fakeAccountRepo) Create(_ context.Context, _ *model.Account) error { return nil }
func (r *fakeAccountRepo) AccountNumberExists(_ context.Context, accountNumber string) (bool, error) {
	_, ok := r.accounts[accountNumber]
	return ok, nil
}
func (r *fakeAccountRepo) GetByAccountNumber(ctx context.Context, accountNumber string) (*model.Account, error) {
	return r.FindByAccountNumber(ctx, accountNumber)
}
func (r *fakeAccountRepo) Update(_ context.Context, account *model.Account) error {
	r.accounts[account.AccountNumber] = account
	return nil
}
func (r *fakeAccountRepo) FindAllByClientID(_ context.Context, _ uint) ([]model.Account, error) {
	return nil, nil
}
func (r *fakeAccountRepo) FindByAccountNumberAndClientID(_ context.Context, _ string, _ uint) (*model.Account, error) {
	return nil, nil
}
func (r *fakeAccountRepo) UpdateName(_ context.Context, _ string, _ string) error { return nil }
func (r *fakeAccountRepo) UpdateLimits(_ context.Context, _ string, _ float64, _ float64) error {
	return nil
}
func (r *fakeAccountRepo) NameExistsForClient(_ context.Context, _ uint, _ string, _ string) (bool, error) {
	return false, nil
}
func (r *fakeAccountRepo) FindByAccountNumber(_ context.Context, accountNumber string) (*model.Account, error) {
	account, ok := r.accounts[accountNumber]
	if !ok {
		return nil, nil
	}
	return account, nil
}
func (r *fakeAccountRepo) UpdateBalance(_ context.Context, account *model.Account) error {
	r.accounts[account.AccountNumber] = account
	return nil
}
func (r *fakeAccountRepo) FindAll(_ context.Context, _ *dto.ListAccountsQuery) ([]*model.Account, int64, error) {
	return nil, 0, nil
}
func (r *fakeAccountRepo) FindByClientID(_ context.Context, _ uint) ([]model.Account, error) {
	return nil, nil
}

type fakeReservationRepo struct {
	reservations map[string]*model.OtcFundsReservation
}

func newFakeReservationRepo() *fakeReservationRepo {
	return &fakeReservationRepo{reservations: map[string]*model.OtcFundsReservation{}}
}

func (r *fakeReservationRepo) Create(_ context.Context, reservation *model.OtcFundsReservation) error {
	copy := *reservation
	r.reservations[reservation.ExecutionID] = &copy
	return nil
}

func (r *fakeReservationRepo) FindByExecutionID(_ context.Context, executionID string) (*model.OtcFundsReservation, error) {
	reservation, ok := r.reservations[executionID]
	if !ok {
		return nil, nil
	}
	copy := *reservation
	return &copy, nil
}

func (r *fakeReservationRepo) Save(_ context.Context, reservation *model.OtcFundsReservation) error {
	copy := *reservation
	r.reservations[reservation.ExecutionID] = &copy
	return nil
}

type fakeExchangeRateRepo struct{}

func (r *fakeExchangeRateRepo) UpsertAll(_ context.Context, _ []model.ExchangeRate) error { return nil }
func (r *fakeExchangeRateRepo) GetAll(_ context.Context) ([]model.ExchangeRate, error) {
	return nil, nil
}

type fakeExchangeClient struct{}

func (c *fakeExchangeClient) FetchRates(_ context.Context) (*client.ExchangeRateAPIResponse, error) {
	return nil, errors.New("not used")
}

type fakeTxManager struct{}

func (m *fakeTxManager) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func testAccount(number string, balance float64) *model.Account {
	return &model.Account{
		AccountNumber:    number,
		Balance:          balance,
		AvailableBalance: balance,
		Currency:         model.Currency{Code: model.RSD},
	}
}

func newServiceForTest(accounts ...*model.Account) (*service.OtcFundsService, *fakeAccountRepo, *fakeReservationRepo) {
	accountRepo := newFakeAccountRepo(accounts...)
	reservationRepo := newFakeReservationRepo()
	exchangeSvc := service.NewExchangeService(&fakeExchangeRateRepo{}, &fakeExchangeClient{})
	return service.NewOtcFundsService(accountRepo, reservationRepo, &fakeTxManager{}, exchangeSvc), accountRepo, reservationRepo
}

func TestReserveOnlyReducesAvailableBalance(t *testing.T) {
	svc, accountRepo, _ := newServiceForTest(testAccount("BUYER", 1000), testAccount("SELLER", 250))

	reservation, err := svc.Reserve(context.Background(), "exec-1", "BUYER", "SELLER", 300, model.RSD)
	require.NoError(t, err)
	require.Equal(t, model.OtcFundsReservationStatusReserved, reservation.Status)
	require.Equal(t, 1000.0, accountRepo.accounts["BUYER"].Balance)
	require.Equal(t, 700.0, accountRepo.accounts["BUYER"].AvailableBalance)
	require.Equal(t, 250.0, accountRepo.accounts["SELLER"].Balance)
	require.Equal(t, 250.0, accountRepo.accounts["SELLER"].AvailableBalance)
}

func TestReleaseRestoresAvailableBalance(t *testing.T) {
	svc, accountRepo, _ := newServiceForTest(testAccount("BUYER", 1000), testAccount("SELLER", 250))

	_, err := svc.Reserve(context.Background(), "exec-2", "BUYER", "SELLER", 300, model.RSD)
	require.NoError(t, err)

	reservation, err := svc.Release(context.Background(), "exec-2")
	require.NoError(t, err)
	require.Equal(t, model.OtcFundsReservationStatusReleased, reservation.Status)
	require.Equal(t, 1000.0, accountRepo.accounts["BUYER"].Balance)
	require.Equal(t, 1000.0, accountRepo.accounts["BUYER"].AvailableBalance)
}

func TestCommitMovesBalancesExactlyOnce(t *testing.T) {
	svc, accountRepo, _ := newServiceForTest(testAccount("BUYER", 1000), testAccount("SELLER", 250))

	_, err := svc.Reserve(context.Background(), "exec-3", "BUYER", "SELLER", 300, model.RSD)
	require.NoError(t, err)

	reservation, err := svc.Commit(context.Background(), "exec-3")
	require.NoError(t, err)
	require.Equal(t, model.OtcFundsReservationStatusCommitted, reservation.Status)
	require.Equal(t, 700.0, accountRepo.accounts["BUYER"].Balance)
	require.Equal(t, 700.0, accountRepo.accounts["BUYER"].AvailableBalance)
	require.Equal(t, 550.0, accountRepo.accounts["SELLER"].Balance)
	require.Equal(t, 550.0, accountRepo.accounts["SELLER"].AvailableBalance)

	reservation, err = svc.Commit(context.Background(), "exec-3")
	require.NoError(t, err)
	require.Equal(t, 700.0, accountRepo.accounts["BUYER"].Balance)
	require.Equal(t, 550.0, accountRepo.accounts["SELLER"].Balance)
}

func TestRefundReversesCommittedTransfer(t *testing.T) {
	svc, accountRepo, _ := newServiceForTest(testAccount("BUYER", 1000), testAccount("SELLER", 250))

	_, err := svc.Reserve(context.Background(), "exec-4", "BUYER", "SELLER", 300, model.RSD)
	require.NoError(t, err)
	_, err = svc.Commit(context.Background(), "exec-4")
	require.NoError(t, err)

	reservation, err := svc.Refund(context.Background(), "exec-4")
	require.NoError(t, err)
	require.Equal(t, model.OtcFundsReservationStatusRefunded, reservation.Status)
	require.Equal(t, 1000.0, accountRepo.accounts["BUYER"].Balance)
	require.Equal(t, 1000.0, accountRepo.accounts["BUYER"].AvailableBalance)
	require.Equal(t, 250.0, accountRepo.accounts["SELLER"].Balance)
	require.Equal(t, 250.0, accountRepo.accounts["SELLER"].AvailableBalance)
}

func TestReserveIsIdempotentForSameExecutionID(t *testing.T) {
	svc, accountRepo, reservationRepo := newServiceForTest(testAccount("BUYER", 1000), testAccount("SELLER", 250))

	first, err := svc.Reserve(context.Background(), "exec-5", "BUYER", "SELLER", 300, model.RSD)
	require.NoError(t, err)

	second, err := svc.Reserve(context.Background(), "exec-5", "BUYER", "SELLER", 300, model.RSD)
	require.NoError(t, err)
	require.Equal(t, first.ExecutionID, second.ExecutionID)
	require.Len(t, reservationRepo.reservations, 1)
	require.Equal(t, 700.0, accountRepo.accounts["BUYER"].AvailableBalance)
}

func TestRefundFailsWhenSellerNoLongerHasFunds(t *testing.T) {
	svc, accountRepo, _ := newServiceForTest(testAccount("BUYER", 1000), testAccount("SELLER", 250))

	_, err := svc.Reserve(context.Background(), "exec-6", "BUYER", "SELLER", 300, model.RSD)
	require.NoError(t, err)
	_, err = svc.Commit(context.Background(), "exec-6")
	require.NoError(t, err)

	accountRepo.accounts["SELLER"].Balance = 100
	accountRepo.accounts["SELLER"].AvailableBalance = 100

	_, err = svc.Refund(context.Background(), "exec-6")
	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient seller funds for refund")
}

func TestReservationTimestampsRoundTrip(t *testing.T) {
	svc, _, reservationRepo := newServiceForTest(testAccount("BUYER", 1000), testAccount("SELLER", 250))

	_, err := svc.Reserve(context.Background(), "exec-7", "BUYER", "SELLER", 10, model.RSD)
	require.NoError(t, err)

	stored := reservationRepo.reservations["exec-7"]
	stored.CreatedAt = time.Now()
	stored.UpdatedAt = stored.CreatedAt
	require.False(t, stored.CreatedAt.IsZero())
}
