package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
)

type fakePayeeRepo struct {
	payees    map[uint]*model.Payee
	createErr error
	updateErr error
	deleteErr error
	nextID    uint
}

func newFakePayeeRepo(payees ...*model.Payee) *fakePayeeRepo {
	m := make(map[uint]*model.Payee)
	var maxID uint
	for _, p := range payees {
		m[p.PayeeID] = p
		if p.PayeeID > maxID {
			maxID = p.PayeeID
		}
	}
	return &fakePayeeRepo{payees: m, nextID: maxID + 1}
}

func (f *fakePayeeRepo) FindAllByClientID(ctx context.Context, clientID uint) ([]model.Payee, error) {
	var result []model.Payee
	for _, p := range f.payees {
		if p.ClientID == clientID {
			result = append(result, *p)
		}
	}
	return result, nil
}

func (f *fakePayeeRepo) FindByID(ctx context.Context, id uint) (*model.Payee, error) {
	p, ok := f.payees[id]
	if !ok {
		return nil, nil
	}
	return p, nil
}

func (f *fakePayeeRepo) Create(ctx context.Context, payee *model.Payee) error {
	if f.createErr != nil {
		return f.createErr
	}
	payee.PayeeID = f.nextID
	f.nextID++
	f.payees[payee.PayeeID] = payee
	return nil
}

func (f *fakePayeeRepo) Update(ctx context.Context, payee *model.Payee) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.payees[payee.PayeeID] = payee
	return nil
}

func (f *fakePayeeRepo) Delete(ctx context.Context, id uint) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.payees, id)
	return nil
}

// fakePayeeAccountRepo is a local stub for AccountRepository used only in payee service tests.
type fakePayeeAccountRepo struct {
	accounts map[string]*model.Account
}

func newFakePayeeAccountRepo(accounts ...*model.Account) *fakePayeeAccountRepo {
	m := make(map[string]*model.Account)
	for _, a := range accounts {
		m[a.AccountNumber] = a
	}
	return &fakePayeeAccountRepo{accounts: m}
}

func (f *fakePayeeAccountRepo) FindByAccountNumber(_ context.Context, accountNumber string) (*model.Account, error) {
	a, ok := f.accounts[accountNumber]
	if !ok {
		return nil, nil
	}
	return a, nil
}

func (f *fakePayeeAccountRepo) AccountNumberExists(_ context.Context, accountNumber string) (bool, error) {
	_, ok := f.accounts[accountNumber]
	return ok, nil
}

func (f *fakePayeeAccountRepo) Create(_ context.Context, _ *model.Account) error {
	return nil
}

func (f *fakePayeeAccountRepo) Update(_ context.Context, _ *model.Account) error {
	return nil
}

func (f *fakePayeeAccountRepo) UpdateBalance(_ context.Context, _ *model.Account) error {
	return nil
}

func (f *fakePayeeAccountRepo) FindAllByClientID(_ context.Context, _ uint) ([]model.Account, error) {
	return nil, nil
}

func (f *fakePayeeAccountRepo) FindByClientID(_ context.Context, _ uint) ([]model.Account, error) {
	return nil, nil
}

func (f *fakePayeeAccountRepo) FindByAccountNumberAndClientID(_ context.Context, accountNumber string, clientID uint) (*model.Account, error) {
	a, ok := f.accounts[accountNumber]
	if !ok || a.ClientID != clientID {
		return nil, nil
	}
	return a, nil
}

func (f *fakePayeeAccountRepo) UpdateName(_ context.Context, _ string, _ string) error {
	return nil
}

func (f *fakePayeeAccountRepo) UpdateLimits(_ context.Context, _ string, _ float64, _ float64) error {
	return nil
}

func (f *fakePayeeAccountRepo) NameExistsForClient(_ context.Context, _ uint, _ string, _ string) (bool, error) {
	return false, nil
}

func (f *fakePayeeAccountRepo) FindAll(_ context.Context, _ *dto.ListAccountsQuery) ([]*model.Account, int64, error) {
	return nil, 0, nil
}

func (f *fakePayeeAccountRepo) FindByAccountType(_ context.Context, _ model.AccountType) (*model.Account, error) {
	return nil, nil
}

func (f *fakePayeeAccountRepo) GetByAccountNumber(_ context.Context, accountNumber string) (*model.Account, error) {
	return f.FindByAccountNumber(context.Background(), accountNumber)
}

func ctxWithClient(clientID uint) context.Context {
	id := clientID
	return auth.SetAuthOnContext(context.Background(), &auth.AuthContext{
		ClientID: &id,
	})
}

func TestGetAll_Success(t *testing.T) {
	repo := newFakePayeeRepo(
		&model.Payee{PayeeID: 1, ClientID: 1, Name: "Ana", AccountNumber: "111"},
		&model.Payee{PayeeID: 2, ClientID: 1, Name: "Marko", AccountNumber: "222"},
		&model.Payee{PayeeID: 3, ClientID: 2, Name: "Drugi klijent", AccountNumber: "333"},
	)
	svc := NewPayeeService(repo, newFakePayeeAccountRepo())

	payees, err := svc.GetAll(ctxWithClient(1))
	require.NoError(t, err)
	require.Len(t, payees, 2)
}

func TestGetAll_Unauthorized(t *testing.T) {
	svc := NewPayeeService(newFakePayeeRepo(), newFakePayeeAccountRepo())

	payees, err := svc.GetAll(context.Background())
	require.Nil(t, payees)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not authenticated as client")
}

func TestCreate_Success(t *testing.T) {
	svc := NewPayeeService(newFakePayeeRepo(), newFakePayeeAccountRepo(
		&model.Account{AccountNumber: "444000112345678913"},
	))

	payee, err := svc.Create(ctxWithClient(1), dto.CreatePayeeRequest{
		Name:          "Stefan",
		AccountNumber: "444000112345678913",
	})
	require.NoError(t, err)
	require.Equal(t, "Stefan", payee.Name)
	require.Equal(t, uint(1), payee.ClientID)
}

func TestCreate_Unauthorized(t *testing.T) {
	svc := NewPayeeService(newFakePayeeRepo(), newFakePayeeAccountRepo())

	payee, err := svc.Create(context.Background(), dto.CreatePayeeRequest{
		Name:          "Stefan",
		AccountNumber: "444000112345678913",
	})
	require.Nil(t, payee)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not authenticated as client")
}

func TestCreate_RepoError(t *testing.T) {
	repo := newFakePayeeRepo()
	repo.createErr = errors.New("db error")
	svc := NewPayeeService(repo, newFakePayeeAccountRepo(
		&model.Account{AccountNumber: "444000112345678913"},
	))

	payee, err := svc.Create(ctxWithClient(1), dto.CreatePayeeRequest{
		Name:          "Stefan",
		AccountNumber: "444000112345678913",
	})
	require.Nil(t, payee)
	require.Error(t, err)
}

func TestUpdate_Success(t *testing.T) {
	repo := newFakePayeeRepo(
		&model.Payee{PayeeID: 1, ClientID: 1, Name: "Staro ime", AccountNumber: "111"},
	)
	svc := NewPayeeService(repo, newFakePayeeAccountRepo())

	payee, err := svc.Update(ctxWithClient(1), 1, dto.UpdatePayeeRequest{Name: "Novo ime"})
	require.NoError(t, err)
	require.Equal(t, "Novo ime", payee.Name)
	require.Equal(t, "111", payee.AccountNumber)
}

func TestUpdate_NotFound(t *testing.T) {
	svc := NewPayeeService(newFakePayeeRepo(), newFakePayeeAccountRepo())

	payee, err := svc.Update(ctxWithClient(1), 99, dto.UpdatePayeeRequest{Name: "Novo ime"})
	require.Nil(t, payee)
	require.Error(t, err)
	require.Contains(t, err.Error(), "payee not found")
}

func TestUpdate_Forbidden(t *testing.T) {
	repo := newFakePayeeRepo(
		&model.Payee{PayeeID: 1, ClientID: 2, Name: "Tudji", AccountNumber: "111"},
	)
	svc := NewPayeeService(repo, newFakePayeeAccountRepo())

	payee, err := svc.Update(ctxWithClient(1), 1, dto.UpdatePayeeRequest{Name: "Pokusaj"})
	require.Nil(t, payee)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not your payee")
}

func TestUpdate_Unauthorized(t *testing.T) {
	svc := NewPayeeService(newFakePayeeRepo(), newFakePayeeAccountRepo())

	payee, err := svc.Update(context.Background(), 1, dto.UpdatePayeeRequest{Name: "Novo"})
	require.Nil(t, payee)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not authenticated as client")
}

func TestDelete_Success(t *testing.T) {
	repo := newFakePayeeRepo(
		&model.Payee{PayeeID: 1, ClientID: 1, Name: "Ana", AccountNumber: "111"},
	)
	svc := NewPayeeService(repo, newFakePayeeAccountRepo())

	err := svc.Delete(ctxWithClient(1), 1)
	require.NoError(t, err)
}

func TestDelete_NotFound(t *testing.T) {
	svc := NewPayeeService(newFakePayeeRepo(), newFakePayeeAccountRepo())

	err := svc.Delete(ctxWithClient(1), 99)
	require.Error(t, err)
	require.Contains(t, err.Error(), "payee not found")
}

func TestDelete_Forbidden(t *testing.T) {
	repo := newFakePayeeRepo(
		&model.Payee{PayeeID: 1, ClientID: 2, Name: "Tudji", AccountNumber: "111"},
	)
	svc := NewPayeeService(repo, newFakePayeeAccountRepo())

	err := svc.Delete(ctxWithClient(1), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not your payee")
}

func TestDelete_Unauthorized(t *testing.T) {
	svc := NewPayeeService(newFakePayeeRepo(), newFakePayeeAccountRepo())

	err := svc.Delete(context.Background(), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not authenticated as client")
}
