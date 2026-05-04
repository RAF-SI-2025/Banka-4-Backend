package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
)

// ── Error-injecting fakes ─────────────────────────────────────────────────────

type errCardRepo struct {
	fakeCardServiceCardRepo
	createErr                error
	findByIDErr              error
	listErr                  error
	countNonDeactErr         error
	countNonDeactByPersonErr error
	cardNumberExistsErr      error
	updateErr                error
}

func (r *errCardRepo) Create(ctx context.Context, card *model.Card) error {
	if r.createErr != nil {
		return r.createErr
	}
	return r.fakeCardServiceCardRepo.Create(ctx, card)
}

func (r *errCardRepo) FindByID(ctx context.Context, id uint) (*model.Card, error) {
	if r.findByIDErr != nil {
		return nil, r.findByIDErr
	}
	return r.fakeCardServiceCardRepo.FindByID(ctx, id)
}

func (r *errCardRepo) ListByAccountNumber(ctx context.Context, accountNumber string) ([]model.Card, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.fakeCardServiceCardRepo.ListByAccountNumber(ctx, accountNumber)
}

func (r *errCardRepo) CountNonDeactivatedByAccountNumber(ctx context.Context, accountNumber string) (int64, error) {
	if r.countNonDeactErr != nil {
		return 0, r.countNonDeactErr
	}
	return r.fakeCardServiceCardRepo.CountNonDeactivatedByAccountNumber(ctx, accountNumber)
}

func (r *errCardRepo) CountNonDeactivatedByAccountNumberAndAuthorizedPersonID(ctx context.Context, accountNumber string, authorizedPersonID *uint) (int64, error) {
	if r.countNonDeactByPersonErr != nil {
		return 0, r.countNonDeactByPersonErr
	}
	return r.fakeCardServiceCardRepo.CountNonDeactivatedByAccountNumberAndAuthorizedPersonID(ctx, accountNumber, authorizedPersonID)
}

func (r *errCardRepo) CardNumberExists(ctx context.Context, cardNumber string) (bool, error) {
	if r.cardNumberExistsErr != nil {
		return false, r.cardNumberExistsErr
	}
	return r.fakeCardServiceCardRepo.CardNumberExists(ctx, cardNumber)
}

func (r *errCardRepo) Update(ctx context.Context, card *model.Card) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	return r.fakeCardServiceCardRepo.Update(ctx, card)
}

func (r *errCardRepo) CountByAccountNumber(ctx context.Context, accountNumber string) (int64, error) {
	return r.fakeCardServiceCardRepo.CountByAccountNumber(ctx, accountNumber)
}

func (r *errCardRepo) CountByAccountNumberAndAuthorizedPersonID(ctx context.Context, accountNumber string, authorizedPersonID *uint) (int64, error) {
	return r.fakeCardServiceCardRepo.CountByAccountNumberAndAuthorizedPersonID(ctx, accountNumber, authorizedPersonID)
}

type errAccountRepo struct {
	fakeCardServiceAccountRepo
	findErr error
}

func (r *errAccountRepo) FindByAccountNumber(ctx context.Context, accountNumber string) (*model.Account, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	return r.fakeCardServiceAccountRepo.FindByAccountNumber(ctx, accountNumber)
}

type errCardRequestRepo struct {
	fakeCardServiceCardRequestRepo
	createErr      error
	findPendingErr error
	findByCodeErr  error
	updateErr      error
	pendingResult  *model.CardRequest
}

func (r *errCardRequestRepo) Create(ctx context.Context, request *model.CardRequest) error {
	if r.createErr != nil {
		return r.createErr
	}
	return r.fakeCardServiceCardRequestRepo.Create(ctx, request)
}

func (r *errCardRequestRepo) FindLatestPendingByAccountNumber(ctx context.Context, accountNumber string) (*model.CardRequest, error) {
	if r.findPendingErr != nil {
		return nil, r.findPendingErr
	}
	if r.pendingResult != nil {
		return r.pendingResult, nil
	}
	return r.fakeCardServiceCardRequestRepo.FindLatestPendingByAccountNumber(ctx, accountNumber)
}

func (r *errCardRequestRepo) FindByAccountNumberAndCode(ctx context.Context, accountNumber string, code string) (*model.CardRequest, error) {
	if r.findByCodeErr != nil {
		return nil, r.findByCodeErr
	}
	return r.fakeCardServiceCardRequestRepo.FindByAccountNumberAndCode(ctx, accountNumber, code)
}

func (r *errCardRequestRepo) Update(ctx context.Context, request *model.CardRequest) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	return r.fakeCardServiceCardRequestRepo.Update(ctx, request)
}

type errAuthorizedPersonRepo struct {
	fakeCardServiceAuthorizedPersonRepo
	createErr   error
	findByIDErr error
}

func (r *errAuthorizedPersonRepo) Create(ctx context.Context, person *model.AuthorizedPerson) error {
	if r.createErr != nil {
		return r.createErr
	}
	return r.fakeCardServiceAuthorizedPersonRepo.Create(ctx, person)
}

func (r *errAuthorizedPersonRepo) FindByID(ctx context.Context, id uint) (*model.AuthorizedPerson, error) {
	if r.findByIDErr != nil {
		return nil, r.findByIDErr
	}
	return r.fakeCardServiceAuthorizedPersonRepo.FindByID(ctx, id)
}

func (r *errAuthorizedPersonRepo) ListByAccountNumber(ctx context.Context, accountNumber string) ([]model.AuthorizedPerson, error) {
	return r.fakeCardServiceAuthorizedPersonRepo.ListByAccountNumber(ctx, accountNumber)
}

// ── NewCardService ─────────────────────────────────────────────────────────────

func TestNewCardService(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{accounts: map[string]*model.Account{}}
	cardRepo := &fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}}
	personRepo := &fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}}
	requestRepo := &fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}}
	userClient := &fakeCardServiceUserClient{}
	mailer := &fakeCardServiceMailer{}
	txMgr := &fakeBankingTxManager{}

	svc := NewCardService(accountRepo, cardRepo, personRepo, requestRepo, userClient, mailer, txMgr)
	require.NotNil(t, svc)
	require.Equal(t, accountRepo, svc.accountRepo)
	require.Equal(t, cardRepo, svc.cardRepo)
	require.Equal(t, personRepo, svc.authorizedPersonRepo)
	require.Equal(t, requestRepo, svc.cardRequestRepo)
	require.Equal(t, userClient, svc.userClient)
	require.Equal(t, mailer, svc.mailer)
	require.Equal(t, txMgr, svc.txManager)
}

// ── RequestCard ────────────────────────────────────────────────────────────────

func TestRequestCard_NilInput(t *testing.T) {
	svc := newCardServiceForTests(
		&fakeCardServiceAccountRepo{accounts: map[string]*model.Account{}},
		&fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}},
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.RequestCard(clientContext(1), nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "request body is required")
}

func TestRequestCard_EmptyAccountNumber(t *testing.T) {
	svc := newCardServiceForTests(
		&fakeCardServiceAccountRepo{accounts: map[string]*model.Account{}},
		&fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}},
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.RequestCard(clientContext(1), &RequestCardInput{AccountNumber: "  "})
	require.Error(t, err)
	require.Contains(t, err.Error(), "account number is required")
}

func TestRequestCard_AccountRepoError(t *testing.T) {
	accountRepo := &errAccountRepo{
		fakeCardServiceAccountRepo: fakeCardServiceAccountRepo{accounts: map[string]*model.Account{}},
		findErr:                    fmt.Errorf("db error"),
	}
	svc := newCardServiceForTests(
		accountRepo,
		&fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}},
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.RequestCard(clientContext(1), &RequestCardInput{AccountNumber: "ACC-1"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "db error")
}

func TestRequestCard_AccountNotFound(t *testing.T) {
	svc := newCardServiceForTests(
		&fakeCardServiceAccountRepo{accounts: map[string]*model.Account{}},
		&fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}},
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.RequestCard(clientContext(1), &RequestCardInput{AccountNumber: "MISSING"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "account not found")
}

func TestRequestCard_ForbiddenClientMismatch(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 2, AccountType: model.AccountTypePersonal},
		},
	}
	svc := newCardServiceForTests(
		accountRepo,
		&fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}},
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.RequestCard(clientContext(1), &RequestCardInput{AccountNumber: "ACC-1"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "account does not belong to authenticated client")
}

func TestRequestCard_PendingRequestExists(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 1, AccountType: model.AccountTypePersonal},
		},
	}
	requestRepo := &errCardRequestRepo{
		fakeCardServiceCardRequestRepo: fakeCardServiceCardRequestRepo{
			requests: map[uint]*model.CardRequest{},
			nextID:   0,
		},
		pendingResult: &model.CardRequest{
			CardRequestID:    1,
			AccountNumber:    "ACC-1",
			ConfirmationCode: "999999",
			ExpiresAt:        time.Now().Add(10 * time.Minute),
		},
	}
	svc := newCardServiceForTests(
		accountRepo,
		&fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}},
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		requestRepo,
		nil, nil,
	)

	_, err := svc.RequestCard(clientContext(1), &RequestCardInput{AccountNumber: "ACC-1"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "pending card request already exists")
}

func TestRequestCard_FindPendingError(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 1, AccountType: model.AccountTypePersonal},
		},
	}
	requestRepo := &errCardRequestRepo{
		fakeCardServiceCardRequestRepo: fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		findPendingErr:                 fmt.Errorf("db error"),
	}
	svc := newCardServiceForTests(
		accountRepo,
		&fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}},
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		requestRepo,
		nil, nil,
	)

	_, err := svc.RequestCard(clientContext(1), &RequestCardInput{AccountNumber: "ACC-1"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "db error")
}

func TestRequestCard_PersonalWithAuthorizedPerson(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 1, AccountType: model.AccountTypePersonal},
		},
	}
	svc := newCardServiceForTests(
		accountRepo,
		&fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}},
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.RequestCard(clientContext(1), &RequestCardInput{
		AccountNumber:    "ACC-1",
		AuthorizedPerson: &AuthorizedPersonInput{FirstName: "Test"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "personal accounts cannot create cards for authorized persons")
}

func TestRequestCard_PersonalMaxCardsReached(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 1, AccountType: model.AccountTypePersonal},
		},
	}
	// Two existing active cards
	cardRepo := &fakeCardServiceCardRepo{
		cards: map[uint]*model.Card{
			1: {CardID: 1, AccountNumber: "ACC-1", Status: model.CardStatusActive},
			2: {CardID: 2, AccountNumber: "ACC-1", Status: model.CardStatusBlocked},
		},
		existingPANs: map[string]bool{},
	}
	svc := newCardServiceForTests(
		accountRepo, cardRepo,
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.RequestCard(clientContext(1), &RequestCardInput{AccountNumber: "ACC-1"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "maximum number of cards reached")
}

func TestRequestCard_PersonalCountError(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 1, AccountType: model.AccountTypePersonal},
		},
	}
	cardRepo := &errCardRepo{
		fakeCardServiceCardRepo: fakeCardServiceCardRepo{
			cards: map[uint]*model.Card{}, existingPANs: map[string]bool{},
		},
		countNonDeactErr: fmt.Errorf("count error"),
	}
	svc := newCardServiceForTests(
		accountRepo, cardRepo,
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.RequestCard(clientContext(1), &RequestCardInput{AccountNumber: "ACC-1"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "count error")
}

func TestRequestCard_BusinessOwnerCountError(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"BUS-1": {AccountNumber: "BUS-1", ClientID: 1, AccountType: model.AccountTypeBusiness},
		},
	}
	cardRepo := &errCardRepo{
		fakeCardServiceCardRepo: fakeCardServiceCardRepo{
			cards: map[uint]*model.Card{}, existingPANs: map[string]bool{},
		},
		countNonDeactByPersonErr: fmt.Errorf("count error"),
	}
	svc := newCardServiceForTests(
		accountRepo, cardRepo,
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.RequestCard(clientContext(1), &RequestCardInput{AccountNumber: "BUS-1"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "count error")
}

func TestRequestCard_BusinessAuthorizedPersonMissingFirstName(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"BUS-1": {AccountNumber: "BUS-1", ClientID: 1, AccountType: model.AccountTypeBusiness},
		},
	}
	svc := newCardServiceForTests(
		accountRepo,
		&fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}},
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.RequestCard(clientContext(1), &RequestCardInput{
		AccountNumber: "BUS-1",
		AuthorizedPerson: &AuthorizedPersonInput{
			FirstName: "",
			LastName:  "Petrovic",
			Email:     "test@example.com",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "authorized person first name is required")
}

func TestRequestCard_BusinessAuthorizedPersonMissingLastName(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"BUS-1": {AccountNumber: "BUS-1", ClientID: 1, AccountType: model.AccountTypeBusiness},
		},
	}
	svc := newCardServiceForTests(
		accountRepo,
		&fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}},
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.RequestCard(clientContext(1), &RequestCardInput{
		AccountNumber: "BUS-1",
		AuthorizedPerson: &AuthorizedPersonInput{
			FirstName: "Ana",
			LastName:  "",
			Email:     "test@example.com",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "authorized person last name is required")
}

func TestRequestCard_BusinessAuthorizedPersonMissingEmail(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"BUS-1": {AccountNumber: "BUS-1", ClientID: 1, AccountType: model.AccountTypeBusiness},
		},
	}
	svc := newCardServiceForTests(
		accountRepo,
		&fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}},
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.RequestCard(clientContext(1), &RequestCardInput{
		AccountNumber: "BUS-1",
		AuthorizedPerson: &AuthorizedPersonInput{
			FirstName: "Ana",
			LastName:  "Petrovic",
			Email:     "",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "authorized person email is required")
}

func TestRequestCard_CardRequestCreateError(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 1, AccountType: model.AccountTypePersonal},
		},
	}
	requestRepo := &errCardRequestRepo{
		fakeCardServiceCardRequestRepo: fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		createErr:                      fmt.Errorf("create error"),
	}
	svc := newCardServiceForTests(
		accountRepo,
		&fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}},
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		requestRepo,
		nil, nil,
	)

	_, err := svc.RequestCard(clientContext(1), &RequestCardInput{AccountNumber: "ACC-1"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "create error")
}

func TestRequestCard_UserClientError(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 1, AccountType: model.AccountTypePersonal},
		},
	}
	userClient := &fakeCardServiceUserClient{
		clientErr: fmt.Errorf("user service unavailable"),
	}
	svc := newCardServiceForTests(
		accountRepo,
		&fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}},
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		userClient, nil,
	)

	_, err := svc.RequestCard(clientContext(1), &RequestCardInput{AccountNumber: "ACC-1"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "user service unavailable")
}

func TestRequestCard_BusinessAuthorizedPersonSuccess(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"BUS-1": {AccountNumber: "BUS-1", ClientID: 1, AccountType: model.AccountTypeBusiness},
		},
	}
	userClient := &fakeCardServiceUserClient{
		clientResp: &pb.GetClientByIdResponse{
			Id:       1,
			Email:    "owner@example.com",
			FullName: "Owner",
		},
	}
	mailer := &fakeCardServiceMailer{}
	svc := newCardServiceForTests(
		accountRepo,
		&fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}},
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		userClient, mailer,
	)

	req, err := svc.RequestCard(clientContext(1), &RequestCardInput{
		AccountNumber: "BUS-1",
		AuthorizedPerson: &AuthorizedPersonInput{
			FirstName: "Ana",
			LastName:  "Petrovic",
			Email:     "ana@example.com",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, req)
	require.True(t, req.ForAuthorizedPerson)
	require.Equal(t, "Ana", *req.AuthorizedPersonFirstName)
	require.Equal(t, "Petrovic", *req.AuthorizedPersonLastName)
	require.Equal(t, "ana@example.com", *req.AuthorizedPersonEmail)
}

// ── ConfirmCardRequest ─────────────────────────────────────────────────────────

func TestConfirmCardRequest_AccountRepoError(t *testing.T) {
	accountRepo := &errAccountRepo{
		fakeCardServiceAccountRepo: fakeCardServiceAccountRepo{accounts: map[string]*model.Account{}},
		findErr:                    fmt.Errorf("db error"),
	}
	svc := newCardServiceForTests(
		accountRepo,
		&fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}},
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.ConfirmCardRequest(clientContext(1), "ACC-1", "123456")
	require.Error(t, err)
	require.Contains(t, err.Error(), "db error")
}

func TestConfirmCardRequest_AccountNotFound(t *testing.T) {
	svc := newCardServiceForTests(
		&fakeCardServiceAccountRepo{accounts: map[string]*model.Account{}},
		&fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}},
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.ConfirmCardRequest(clientContext(1), "MISSING", "123456")
	require.Error(t, err)
	require.Contains(t, err.Error(), "account not found")
}

func TestConfirmCardRequest_ForbiddenClientMismatch(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 2, AccountType: model.AccountTypePersonal},
		},
	}
	svc := newCardServiceForTests(
		accountRepo,
		&fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}},
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.ConfirmCardRequest(clientContext(1), "ACC-1", "123456")
	require.Error(t, err)
	require.Contains(t, err.Error(), "account does not belong to authenticated client")
}

func TestConfirmCardRequest_FindByCodeError(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 1, AccountType: model.AccountTypePersonal},
		},
	}
	requestRepo := &errCardRequestRepo{
		fakeCardServiceCardRequestRepo: fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		findByCodeErr:                  fmt.Errorf("db error"),
	}
	svc := newCardServiceForTests(
		accountRepo,
		&fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}},
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		requestRepo,
		nil, nil,
	)

	_, err := svc.ConfirmCardRequest(clientContext(1), "ACC-1", "123456")
	require.Error(t, err)
	require.Contains(t, err.Error(), "db error")
}

func TestConfirmCardRequest_InvalidCode(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 1, AccountType: model.AccountTypePersonal},
		},
	}
	svc := newCardServiceForTests(
		accountRepo,
		&fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}},
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.ConfirmCardRequest(clientContext(1), "ACC-1", "WRONG")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid or expired confirmation code")
}

func TestConfirmCardRequest_CodeUsed(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 1, AccountType: model.AccountTypePersonal},
		},
	}
	requestRepo := &fakeCardServiceCardRequestRepo{
		requests: map[uint]*model.CardRequest{
			1: {
				CardRequestID:    1,
				AccountNumber:    "ACC-1",
				ConfirmationCode: "111111",
				ExpiresAt:        time.Now().Add(10 * time.Minute),
				Used:             true,
			},
		},
		nextID: 1,
	}
	svc := newCardServiceForTests(
		accountRepo,
		&fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}},
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		requestRepo,
		nil, nil,
	)

	_, err := svc.ConfirmCardRequest(clientContext(1), "ACC-1", "111111")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid or expired confirmation code")
}

func TestConfirmCardRequest_CodeExpired(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 1, AccountType: model.AccountTypePersonal},
		},
	}
	requestRepo := &fakeCardServiceCardRequestRepo{
		requests: map[uint]*model.CardRequest{
			1: {
				CardRequestID:    1,
				AccountNumber:    "ACC-1",
				ConfirmationCode: "222222",
				ExpiresAt:        time.Now().Add(-10 * time.Minute), // expired
				Used:             false,
			},
		},
		nextID: 1,
	}
	svc := newCardServiceForTests(
		accountRepo,
		&fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}},
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		requestRepo,
		nil, nil,
	)

	_, err := svc.ConfirmCardRequest(clientContext(1), "ACC-1", "222222")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid or expired confirmation code")
}

func TestConfirmCardRequest_PersonalMaxCardsReached(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 1, AccountType: model.AccountTypePersonal},
		},
	}
	cardRepo := &fakeCardServiceCardRepo{
		cards: map[uint]*model.Card{
			1: {CardID: 1, AccountNumber: "ACC-1", Status: model.CardStatusActive},
			2: {CardID: 2, AccountNumber: "ACC-1", Status: model.CardStatusBlocked},
		},
		existingPANs: map[string]bool{},
	}
	requestRepo := &fakeCardServiceCardRequestRepo{
		requests: map[uint]*model.CardRequest{
			1: {
				CardRequestID:    1,
				AccountNumber:    "ACC-1",
				ConfirmationCode: "333333",
				ExpiresAt:        time.Now().Add(10 * time.Minute),
				Used:             false,
			},
		},
		nextID: 1,
	}
	svc := newCardServiceForTests(
		accountRepo, cardRepo,
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		requestRepo,
		nil, nil,
	)

	_, err := svc.ConfirmCardRequest(clientContext(1), "ACC-1", "333333")
	require.Error(t, err)
	require.Contains(t, err.Error(), "maximum number of cards reached")
}

func TestConfirmCardRequest_PersonalSuccess(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 1, AccountType: model.AccountTypePersonal, MonthlyLimit: 1000000},
		},
	}
	requestRepo := &fakeCardServiceCardRequestRepo{
		requests: map[uint]*model.CardRequest{
			1: {
				CardRequestID:    1,
				AccountNumber:    "ACC-1",
				ConfirmationCode: "444444",
				ExpiresAt:        time.Now().Add(10 * time.Minute),
				Used:             false,
			},
		},
		nextID: 1,
	}
	mailer := &fakeCardServiceMailer{}
	svc := newCardServiceForTests(
		accountRepo,
		&fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}},
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		requestRepo,
		nil, mailer,
	)

	card, err := svc.ConfirmCardRequest(clientContext(1), "ACC-1", "444444")
	require.NoError(t, err)
	require.NotNil(t, card)
	require.Equal(t, model.CardStatusActive, card.Status)
}

func TestConfirmCardRequest_BusinessOwnerMaxCardsReached(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"BUS-1": {AccountNumber: "BUS-1", ClientID: 1, AccountType: model.AccountTypeBusiness},
		},
	}
	cardRepo := &fakeCardServiceCardRepo{
		cards: map[uint]*model.Card{
			1: {CardID: 1, AccountNumber: "BUS-1", Status: model.CardStatusActive, AuthorizedPersonID: nil},
		},
		existingPANs: map[string]bool{},
	}
	requestRepo := &fakeCardServiceCardRequestRepo{
		requests: map[uint]*model.CardRequest{
			1: {
				CardRequestID:       1,
				AccountNumber:       "BUS-1",
				ConfirmationCode:    "555555",
				ExpiresAt:           time.Now().Add(10 * time.Minute),
				Used:                false,
				ForAuthorizedPerson: false,
			},
		},
		nextID: 1,
	}
	svc := newCardServiceForTests(
		accountRepo, cardRepo,
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		requestRepo,
		nil, nil,
	)

	_, err := svc.ConfirmCardRequest(clientContext(1), "BUS-1", "555555")
	require.Error(t, err)
	require.Contains(t, err.Error(), "business account owner already has a card")
}

func TestConfirmCardRequest_AuthorizedPersonIncompleteData(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"BUS-1": {AccountNumber: "BUS-1", ClientID: 1, AccountType: model.AccountTypeBusiness},
		},
	}
	// ForAuthorizedPerson=true but missing email (empty string pointer)
	emptyStr := ""
	requestRepo := &fakeCardServiceCardRequestRepo{
		requests: map[uint]*model.CardRequest{
			1: {
				CardRequestID:             1,
				AccountNumber:             "BUS-1",
				ConfirmationCode:          "666666",
				ExpiresAt:                 time.Now().Add(10 * time.Minute),
				Used:                      false,
				ForAuthorizedPerson:       true,
				AuthorizedPersonFirstName: stringPointer("Ana"),
				AuthorizedPersonLastName:  stringPointer("Petrovic"),
				AuthorizedPersonEmail:     &emptyStr, // empty
			},
		},
		nextID: 1,
	}
	svc := newCardServiceForTests(
		accountRepo,
		&fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}},
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		requestRepo,
		nil, nil,
	)

	_, err := svc.ConfirmCardRequest(clientContext(1), "BUS-1", "666666")
	require.Error(t, err)
	require.Contains(t, err.Error(), "authorized person data is incomplete")
}

// ── ListCardsForAccount ────────────────────────────────────────────────────────

func TestListCardsForAccount_AccountNotFound(t *testing.T) {
	svc := newCardServiceForTests(
		&fakeCardServiceAccountRepo{accounts: map[string]*model.Account{}},
		&fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}},
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.ListCardsForAccount(clientContext(1), "MISSING")
	require.Error(t, err)
	require.Contains(t, err.Error(), "account not found")
}

func TestListCardsForAccount_AccountRepoError(t *testing.T) {
	accountRepo := &errAccountRepo{
		fakeCardServiceAccountRepo: fakeCardServiceAccountRepo{accounts: map[string]*model.Account{}},
		findErr:                    fmt.Errorf("db error"),
	}
	svc := newCardServiceForTests(
		accountRepo,
		&fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}},
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.ListCardsForAccount(clientContext(1), "ACC-1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "db error")
}

func TestListCardsForAccount_EmployeeSuccess(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 99, AccountType: model.AccountTypePersonal},
		},
	}
	cardRepo := &fakeCardServiceCardRepo{
		cards: map[uint]*model.Card{
			1: {CardID: 1, AccountNumber: "ACC-1", Status: model.CardStatusActive},
		},
		existingPANs: map[string]bool{},
	}
	svc := newCardServiceForTests(
		accountRepo, cardRepo,
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	result, err := svc.ListCardsForAccount(employeeContext(5), "ACC-1")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Cards, 1)
}

func TestListCardsForAccount_ClientSuccess(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 1, AccountType: model.AccountTypePersonal},
		},
	}
	cardRepo := &fakeCardServiceCardRepo{
		cards: map[uint]*model.Card{
			1: {CardID: 1, AccountNumber: "ACC-1", Status: model.CardStatusActive},
			2: {CardID: 2, AccountNumber: "ACC-1", Status: model.CardStatusBlocked},
		},
		existingPANs: map[string]bool{},
	}
	svc := newCardServiceForTests(
		accountRepo, cardRepo,
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	result, err := svc.ListCardsForAccount(clientContext(1), "ACC-1")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Cards, 2)
}

func TestListCardsForAccount_CardRepoError(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 1, AccountType: model.AccountTypePersonal},
		},
	}
	cardRepo := &errCardRepo{
		fakeCardServiceCardRepo: fakeCardServiceCardRepo{
			cards: map[uint]*model.Card{}, existingPANs: map[string]bool{},
		},
		listErr: fmt.Errorf("list error"),
	}
	svc := newCardServiceForTests(
		accountRepo, cardRepo,
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.ListCardsForAccount(clientContext(1), "ACC-1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "list error")
}

func TestListCardsForAccount_UnsupportedIdentityType(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 1, AccountType: model.AccountTypePersonal},
		},
	}
	svc := newCardServiceForTests(
		accountRepo,
		&fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}},
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	ctx := auth.SetAuthOnContext(context.Background(), &auth.AuthContext{
		IdentityType: auth.IdentityType("partner"),
	})
	_, err := svc.ListCardsForAccount(ctx, "ACC-1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported identity type")
}

func TestListCardsForAccount_ClientNoClientID(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 1, AccountType: model.AccountTypePersonal},
		},
	}
	svc := newCardServiceForTests(
		accountRepo,
		&fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}},
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	ctx := auth.SetAuthOnContext(context.Background(), &auth.AuthContext{
		IdentityType: auth.IdentityClient,
		ClientID:     nil,
	})
	_, err := svc.ListCardsForAccount(ctx, "ACC-1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not authenticated")
}

// ── BlockCard ──────────────────────────────────────────────────────────────────

func TestBlockCard_CardNotFound(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{accounts: map[string]*model.Account{}}
	cardRepo := &fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}}
	svc := newCardServiceForTests(accountRepo, cardRepo,
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.BlockCard(clientContext(1), 999)
	require.Error(t, err)
	require.Contains(t, err.Error(), "card not found")
}

func TestBlockCard_CardRepoFindError(t *testing.T) {
	cardRepo := &errCardRepo{
		fakeCardServiceCardRepo: fakeCardServiceCardRepo{
			cards: map[uint]*model.Card{}, existingPANs: map[string]bool{},
		},
		findByIDErr: fmt.Errorf("db error"),
	}
	svc := newCardServiceForTests(
		&fakeCardServiceAccountRepo{accounts: map[string]*model.Account{}},
		cardRepo,
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.BlockCard(clientContext(1), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "db error")
}

func TestBlockCard_DeactivatedCard(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 1},
		},
	}
	cardRepo := &fakeCardServiceCardRepo{
		cards: map[uint]*model.Card{
			1: {CardID: 1, AccountNumber: "ACC-1", Status: model.CardStatusDeactivated},
		},
		existingPANs: map[string]bool{},
	}
	svc := newCardServiceForTests(accountRepo, cardRepo,
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.BlockCard(clientContext(1), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "deactivated cards cannot be blocked")
}

func TestBlockCard_AlreadyBlocked(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 1},
		},
	}
	cardRepo := &fakeCardServiceCardRepo{
		cards: map[uint]*model.Card{
			1: {CardID: 1, AccountNumber: "ACC-1", Status: model.CardStatusBlocked},
		},
		existingPANs: map[string]bool{},
	}
	svc := newCardServiceForTests(accountRepo, cardRepo,
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.BlockCard(clientContext(1), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "card is already blocked")
}

func TestBlockCard_WrongClient(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 2},
		},
	}
	cardRepo := &fakeCardServiceCardRepo{
		cards: map[uint]*model.Card{
			1: {CardID: 1, AccountNumber: "ACC-1", Status: model.CardStatusActive},
		},
		existingPANs: map[string]bool{},
	}
	svc := newCardServiceForTests(accountRepo, cardRepo,
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.BlockCard(clientContext(1), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "card does not belong to authenticated client")
}

func TestBlockCard_EmployeeSuccess(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 99, AccountType: model.AccountTypePersonal},
		},
	}
	cardRepo := &fakeCardServiceCardRepo{
		cards: map[uint]*model.Card{
			1: {CardID: 1, AccountNumber: "ACC-1", Status: model.CardStatusActive},
		},
		existingPANs: map[string]bool{},
	}
	mailer := &fakeCardServiceMailer{}
	svc := newCardServiceForTests(accountRepo, cardRepo,
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, mailer,
	)

	card, err := svc.BlockCard(employeeContext(5), 1)
	require.NoError(t, err)
	require.Equal(t, model.CardStatusBlocked, card.Status)
}

func TestBlockCard_UnsupportedIdentityType(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 1},
		},
	}
	cardRepo := &fakeCardServiceCardRepo{
		cards: map[uint]*model.Card{
			1: {CardID: 1, AccountNumber: "ACC-1", Status: model.CardStatusActive},
		},
		existingPANs: map[string]bool{},
	}
	svc := newCardServiceForTests(accountRepo, cardRepo,
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	ctx := auth.SetAuthOnContext(context.Background(), &auth.AuthContext{
		IdentityType: auth.IdentityType("unknown"),
	})
	_, err := svc.BlockCard(ctx, 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported identity type")
}

func TestBlockCard_CardUpdateError(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 1, AccountType: model.AccountTypePersonal},
		},
	}
	cardRepo := &errCardRepo{
		fakeCardServiceCardRepo: fakeCardServiceCardRepo{
			cards: map[uint]*model.Card{
				1: {CardID: 1, AccountNumber: "ACC-1", Status: model.CardStatusActive},
			},
			existingPANs: map[string]bool{},
		},
		updateErr: fmt.Errorf("update error"),
	}
	svc := newCardServiceForTests(accountRepo, cardRepo,
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.BlockCard(clientContext(1), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "update error")
}

// ── UnblockCard ────────────────────────────────────────────────────────────────

func TestUnblockCard_CardNotFound(t *testing.T) {
	svc := newCardServiceForTests(
		&fakeCardServiceAccountRepo{accounts: map[string]*model.Account{}},
		&fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}},
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.UnblockCard(employeeContext(1), 999)
	require.Error(t, err)
	require.Contains(t, err.Error(), "card not found")
}

func TestUnblockCard_DeactivatedCard(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 1},
		},
	}
	cardRepo := &fakeCardServiceCardRepo{
		cards: map[uint]*model.Card{
			1: {CardID: 1, AccountNumber: "ACC-1", Status: model.CardStatusDeactivated},
		},
		existingPANs: map[string]bool{},
	}
	svc := newCardServiceForTests(accountRepo, cardRepo,
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.UnblockCard(employeeContext(1), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "deactivated cards cannot be unblocked")
}

func TestUnblockCard_NotBlocked(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 1},
		},
	}
	cardRepo := &fakeCardServiceCardRepo{
		cards: map[uint]*model.Card{
			1: {CardID: 1, AccountNumber: "ACC-1", Status: model.CardStatusActive},
		},
		existingPANs: map[string]bool{},
	}
	svc := newCardServiceForTests(accountRepo, cardRepo,
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.UnblockCard(employeeContext(1), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "only blocked cards can be unblocked")
}

func TestUnblockCard_UpdateError(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 1, AccountType: model.AccountTypePersonal},
		},
	}
	cardRepo := &errCardRepo{
		fakeCardServiceCardRepo: fakeCardServiceCardRepo{
			cards: map[uint]*model.Card{
				1: {CardID: 1, AccountNumber: "ACC-1", Status: model.CardStatusBlocked},
			},
			existingPANs: map[string]bool{},
		},
		updateErr: fmt.Errorf("update error"),
	}
	svc := newCardServiceForTests(accountRepo, cardRepo,
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.UnblockCard(employeeContext(1), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "update error")
}

// ── DeactivateCard ─────────────────────────────────────────────────────────────

func TestDeactivateCard_AlreadyDeactivated(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 1},
		},
	}
	cardRepo := &fakeCardServiceCardRepo{
		cards: map[uint]*model.Card{
			1: {CardID: 1, AccountNumber: "ACC-1", Status: model.CardStatusDeactivated},
		},
		existingPANs: map[string]bool{},
	}
	svc := newCardServiceForTests(accountRepo, cardRepo,
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.DeactivateCard(employeeContext(1), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "card is already deactivated")
}

func TestDeactivateCard_CardNotFound(t *testing.T) {
	svc := newCardServiceForTests(
		&fakeCardServiceAccountRepo{accounts: map[string]*model.Account{}},
		&fakeCardServiceCardRepo{cards: map[uint]*model.Card{}, existingPANs: map[string]bool{}},
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.DeactivateCard(employeeContext(1), 999)
	require.Error(t, err)
	require.Contains(t, err.Error(), "card not found")
}

func TestDeactivateCard_UpdateError(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 1, AccountType: model.AccountTypePersonal},
		},
	}
	cardRepo := &errCardRepo{
		fakeCardServiceCardRepo: fakeCardServiceCardRepo{
			cards: map[uint]*model.Card{
				1: {CardID: 1, AccountNumber: "ACC-1", Status: model.CardStatusActive},
			},
			existingPANs: map[string]bool{},
		},
		updateErr: fmt.Errorf("update error"),
	}
	svc := newCardServiceForTests(accountRepo, cardRepo,
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.DeactivateCard(employeeContext(1), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "update error")
}

func TestDeactivateCard_FromBlocked(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 1, AccountType: model.AccountTypePersonal},
		},
	}
	cardRepo := &fakeCardServiceCardRepo{
		cards: map[uint]*model.Card{
			1: {CardID: 1, AccountNumber: "ACC-1", Status: model.CardStatusBlocked},
		},
		existingPANs: map[string]bool{},
	}
	mailer := &fakeCardServiceMailer{}
	svc := newCardServiceForTests(accountRepo, cardRepo,
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, mailer,
	)

	card, err := svc.DeactivateCard(employeeContext(1), 1)
	require.NoError(t, err)
	require.Equal(t, model.CardStatusDeactivated, card.Status)
}

// ── getCardAndAccount – account not found ──────────────────────────────────────

func TestGetCardAndAccount_AccountNotFound(t *testing.T) {
	// Card exists but account does not
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{}, // no accounts
	}
	cardRepo := &fakeCardServiceCardRepo{
		cards: map[uint]*model.Card{
			1: {CardID: 1, AccountNumber: "MISSING-ACCT", Status: model.CardStatusActive},
		},
		existingPANs: map[string]bool{},
	}
	svc := newCardServiceForTests(accountRepo, cardRepo,
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		nil, nil,
	)

	_, err := svc.BlockCard(employeeContext(1), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "account not found")
}

// ── Email notification tests ───────────────────────────────────────────────────

func TestBlockCard_EmailSendError(t *testing.T) {
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {AccountNumber: "ACC-1", ClientID: 1, AccountType: model.AccountTypePersonal},
		},
	}
	cardRepo := &fakeCardServiceCardRepo{
		cards: map[uint]*model.Card{
			1: {CardID: 1, AccountNumber: "ACC-1", Status: model.CardStatusActive},
		},
		existingPANs: map[string]bool{},
	}
	userClient := &fakeCardServiceUserClient{
		clientResp: &pb.GetClientByIdResponse{
			Id: 1, Email: "owner@example.com", FullName: "Owner",
		},
	}
	mailer := &fakeCardServiceMailer{sendErr: fmt.Errorf("smtp error")}
	svc := newCardServiceForTests(accountRepo, cardRepo,
		&fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		userClient, mailer,
	)

	_, err := svc.BlockCard(clientContext(1), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "smtp error")
}

func TestDeactivateCard_BusinessCardWithAuthorizedPerson_EmailSent(t *testing.T) {
	authorizedPersonID := uint(5)
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"BUS-1": {AccountNumber: "BUS-1", ClientID: 1, AccountType: model.AccountTypeBusiness},
		},
	}
	cardRepo := &fakeCardServiceCardRepo{
		cards: map[uint]*model.Card{
			1: {CardID: 1, AccountNumber: "BUS-1", Status: model.CardStatusActive, AuthorizedPersonID: &authorizedPersonID},
		},
		existingPANs: map[string]bool{},
	}
	personRepo := &fakeCardServiceAuthorizedPersonRepo{
		people: map[uint]*model.AuthorizedPerson{
			5: {AuthorizedPersonID: 5, AccountNumber: "BUS-1", Email: "person@example.com"},
		},
	}
	userClient := &fakeCardServiceUserClient{
		clientResp: &pb.GetClientByIdResponse{
			Id: 1, Email: "owner@example.com", FullName: "Owner",
		},
	}
	mailer := &fakeCardServiceMailer{}
	svc := newCardServiceForTests(accountRepo, cardRepo, personRepo,
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		userClient, mailer,
	)

	card, err := svc.DeactivateCard(employeeContext(1), 1)
	require.NoError(t, err)
	require.Equal(t, model.CardStatusDeactivated, card.Status)
	// Should have sent to both owner and authorized person
	require.Len(t, mailer.sent, 2)
}

func TestDeactivateCard_BusinessCardWithAuthorizedPersonSameEmail(t *testing.T) {
	// Test containsString deduplication: authorized person has same email as owner
	authorizedPersonID := uint(5)
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"BUS-1": {AccountNumber: "BUS-1", ClientID: 1, AccountType: model.AccountTypeBusiness},
		},
	}
	cardRepo := &fakeCardServiceCardRepo{
		cards: map[uint]*model.Card{
			1: {CardID: 1, AccountNumber: "BUS-1", Status: model.CardStatusActive, AuthorizedPersonID: &authorizedPersonID},
		},
		existingPANs: map[string]bool{},
	}
	personRepo := &fakeCardServiceAuthorizedPersonRepo{
		people: map[uint]*model.AuthorizedPerson{
			5: {AuthorizedPersonID: 5, AccountNumber: "BUS-1", Email: "owner@example.com"}, // same as owner
		},
	}
	userClient := &fakeCardServiceUserClient{
		clientResp: &pb.GetClientByIdResponse{
			Id: 1, Email: "owner@example.com", FullName: "Owner",
		},
	}
	mailer := &fakeCardServiceMailer{}
	svc := newCardServiceForTests(accountRepo, cardRepo, personRepo,
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		userClient, mailer,
	)

	card, err := svc.DeactivateCard(employeeContext(1), 1)
	require.NoError(t, err)
	require.Equal(t, model.CardStatusDeactivated, card.Status)
	// Only one email since both are the same
	require.Len(t, mailer.sent, 1)
}

// ── validateAuthorizedPersonInput ─────────────────────────────────────────────

func TestValidateAuthorizedPersonInput_NilPerson(t *testing.T) {
	err := validateAuthorizedPersonInput(nil)
	require.NoError(t, err)
}

func TestValidateAuthorizedPersonInput_AllValid(t *testing.T) {
	err := validateAuthorizedPersonInput(&AuthorizedPersonInput{
		FirstName: "Ana",
		LastName:  "Petrovic",
		Email:     "ana@example.com",
	})
	require.NoError(t, err)
}

func TestValidateAuthorizedPersonInput_EmptyFirstName(t *testing.T) {
	err := validateAuthorizedPersonInput(&AuthorizedPersonInput{
		FirstName: "  ",
		LastName:  "Petrovic",
		Email:     "ana@example.com",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "authorized person first name is required")
}

func TestValidateAuthorizedPersonInput_EmptyLastName(t *testing.T) {
	err := validateAuthorizedPersonInput(&AuthorizedPersonInput{
		FirstName: "Ana",
		LastName:  "  ",
		Email:     "ana@example.com",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "authorized person last name is required")
}

func TestValidateAuthorizedPersonInput_EmptyEmail(t *testing.T) {
	err := validateAuthorizedPersonInput(&AuthorizedPersonInput{
		FirstName: "Ana",
		LastName:  "Petrovic",
		Email:     "  ",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "authorized person email is required")
}

// ── containsString ─────────────────────────────────────────────────────────────

func TestContainsString(t *testing.T) {
	require.True(t, containsString([]string{"a@x.com", "b@x.com"}, "A@X.COM"))
	require.True(t, containsString([]string{"a@x.com"}, "a@x.com"))
	require.False(t, containsString([]string{"a@x.com"}, "b@x.com"))
	require.False(t, containsString([]string{}, "a@x.com"))
	require.True(t, containsString([]string{"  a@x.com  "}, "a@x.com"))
}

// ── stringPtr / timePtr ───────────────────────────────────────────────────────

func TestStringPtr(t *testing.T) {
	val := "hello"
	ptr := stringPtr(val)
	require.NotNil(t, ptr)
	require.Equal(t, val, *ptr)
}

func TestTimePtr(t *testing.T) {
	now := time.Now()
	ptr := timePtr(now)
	require.NotNil(t, ptr)
	require.Equal(t, now, *ptr)
}

// ── sendCardStatusChangedEmail – authorized person not found ──────────────────

func TestBlockCard_AuthorizedPersonNotFound(t *testing.T) {
	authorizedPersonID := uint(99) // does not exist in repo
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"BUS-1": {AccountNumber: "BUS-1", ClientID: 1, AccountType: model.AccountTypeBusiness},
		},
	}
	cardRepo := &fakeCardServiceCardRepo{
		cards: map[uint]*model.Card{
			1: {CardID: 1, AccountNumber: "BUS-1", Status: model.CardStatusActive, AuthorizedPersonID: &authorizedPersonID},
		},
		existingPANs: map[string]bool{},
	}
	personRepo := &fakeCardServiceAuthorizedPersonRepo{
		people: map[uint]*model.AuthorizedPerson{}, // empty
	}
	userClient := &fakeCardServiceUserClient{
		clientResp: &pb.GetClientByIdResponse{
			Id: 1, Email: "owner@example.com", FullName: "Owner",
		},
	}
	mailer := &fakeCardServiceMailer{}
	svc := newCardServiceForTests(accountRepo, cardRepo, personRepo,
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		userClient, mailer,
	)

	// Employee blocks so ownership check passes
	_, err := svc.BlockCard(employeeContext(1), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "authorized person not found")
}

// ── AuthorizedPersonRepo – find error during notification ────────────────────

func TestBlockCard_AuthorizedPersonFindError(t *testing.T) {
	authorizedPersonID := uint(5)
	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"BUS-1": {AccountNumber: "BUS-1", ClientID: 1, AccountType: model.AccountTypeBusiness},
		},
	}
	cardRepo := &fakeCardServiceCardRepo{
		cards: map[uint]*model.Card{
			1: {CardID: 1, AccountNumber: "BUS-1", Status: model.CardStatusActive, AuthorizedPersonID: &authorizedPersonID},
		},
		existingPANs: map[string]bool{},
	}
	personRepo := &errAuthorizedPersonRepo{
		fakeCardServiceAuthorizedPersonRepo: fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}},
		findByIDErr:                         fmt.Errorf("person db error"),
	}
	userClient := &fakeCardServiceUserClient{
		clientResp: &pb.GetClientByIdResponse{
			Id: 1, Email: "owner@example.com", FullName: "Owner",
		},
	}
	mailer := &fakeCardServiceMailer{}
	svc := newCardServiceForTests(accountRepo, cardRepo, personRepo,
		&fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}},
		userClient, mailer,
	)

	_, err := svc.BlockCard(employeeContext(1), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "person db error")
}
