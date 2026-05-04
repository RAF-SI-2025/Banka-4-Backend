package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/repository"
)

type fakeCardServiceAccountRepo struct {
	accounts   map[string]*model.Account
	accountArr []model.Account
	account    *model.Account
	nameExists bool
}

func (r *fakeCardServiceAccountRepo) Create(_ context.Context, _ *model.Account) error {
	return nil
}

func (r *fakeCardServiceAccountRepo) AccountNumberExists(_ context.Context, _ string) (bool, error) {
	return false, nil
}
func (r *fakeCardServiceAccountRepo) FindByAccountType(ctx context.Context, accountType model.AccountType) (*model.Account, error) {
	return nil, nil
}
func (r *fakeCardServiceAccountRepo) FindByAccountNumber(_ context.Context, accountNumber string) (*model.Account, error) {
	account, ok := r.accounts[accountNumber]
	if !ok {
		return nil, nil
	}

	return account, nil
}

func (f *fakeCardServiceAccountRepo) UpdateBalance(ctx context.Context, account *model.Account) error {
	f.accounts[account.AccountNumber] = account
	return nil
}

func (r *fakeCardServiceAccountRepo) FindByClientID(_ context.Context, _ uint) ([]model.Account, error) {
	return nil, nil
}

func (f *fakeCardServiceAccountRepo) FindAllByClientID(_ context.Context, _ uint) ([]model.Account, error) {
	return f.accountArr, nil
}

func (f *fakeCardServiceAccountRepo) FindByAccountNumberAndClientID(_ context.Context, _ string, _ uint) (*model.Account, error) {
	return f.account, nil
}

func (f *fakeCardServiceAccountRepo) NameExistsForClient(_ context.Context, _ uint, _ string, _ string) (bool, error) {
	return f.nameExists, nil
}

func (f *fakeCardServiceAccountRepo) UpdateName(_ context.Context, _ string, _ string) error {
	return nil
}

func (f *fakeCardServiceAccountRepo) UpdateLimits(_ context.Context, _ string, _ float64, _ float64) error {
	return nil
}

func (f *fakeCardServiceAccountRepo) GetByAccountNumber(ctx context.Context, accountNumber string) (*model.Account, error) {
	return f.FindByAccountNumber(ctx, accountNumber)
}

func (f *fakeCardServiceAccountRepo) Update(_ context.Context, _ *model.Account) error {
	return nil
}

func (f *fakeCardServiceAccountRepo) FindAll(_ context.Context, _ *dto.ListAccountsQuery) ([]*model.Account, int64, error) {
	return nil, 0, nil
}

type fakeCardServiceCardRepo struct {
	cards        map[uint]*model.Card
	nextID       uint
	existingPANs map[string]bool
}

func (r *fakeCardServiceCardRepo) Create(_ context.Context, card *model.Card) error {
	r.nextID++
	card.CardID = r.nextID
	cloned := *card
	r.cards[card.CardID] = &cloned
	r.existingPANs[card.CardNumber] = true
	return nil
}

func (r *fakeCardServiceCardRepo) FindByID(_ context.Context, id uint) (*model.Card, error) {
	card, ok := r.cards[id]
	if !ok {
		return nil, nil
	}

	cloned := *card
	return &cloned, nil
}

func (r *fakeCardServiceCardRepo) ListByAccountNumber(_ context.Context, accountNumber string) ([]model.Card, error) {
	result := make([]model.Card, 0)
	for _, card := range r.cards {
		if card.AccountNumber == accountNumber {
			result = append(result, *card)
		}
	}
	return result, nil
}

func (r *fakeCardServiceCardRepo) CountByAccountNumber(_ context.Context, accountNumber string) (int64, error) {
	var count int64
	for _, card := range r.cards {
		if card.AccountNumber == accountNumber {
			count++
		}
	}
	return count, nil
}

func (r *fakeCardServiceCardRepo) CountByAccountNumberAndAuthorizedPersonID(_ context.Context, accountNumber string, authorizedPersonID *uint) (int64, error) {
	var count int64
	for _, card := range r.cards {
		if card.AccountNumber != accountNumber {
			continue
		}
		if authorizedPersonID == nil && card.AuthorizedPersonID == nil {
			count++
			continue
		}
		if authorizedPersonID != nil && card.AuthorizedPersonID != nil && *authorizedPersonID == *card.AuthorizedPersonID {
			count++
		}
	}
	return count, nil
}

func (r *fakeCardServiceCardRepo) CountNonDeactivatedByAccountNumber(_ context.Context, accountNumber string) (int64, error) {
	var count int64
	for _, card := range r.cards {
		if card.AccountNumber == accountNumber && card.Status != model.CardStatusDeactivated {
			count++
		}
	}
	return count, nil
}

func (r *fakeCardServiceCardRepo) CountNonDeactivatedByAccountNumberAndAuthorizedPersonID(_ context.Context, accountNumber string, authorizedPersonID *uint) (int64, error) {
	var count int64
	for _, card := range r.cards {
		if card.AccountNumber != accountNumber || card.Status == model.CardStatusDeactivated {
			continue
		}
		if authorizedPersonID == nil && card.AuthorizedPersonID == nil {
			count++
			continue
		}
		if authorizedPersonID != nil && card.AuthorizedPersonID != nil && *authorizedPersonID == *card.AuthorizedPersonID {
			count++
		}
	}
	return count, nil
}

func (r *fakeCardServiceCardRepo) CardNumberExists(_ context.Context, cardNumber string) (bool, error) {
	return r.existingPANs[cardNumber], nil
}

func (r *fakeCardServiceCardRepo) Update(_ context.Context, card *model.Card) error {
	cloned := *card
	r.cards[card.CardID] = &cloned
	return nil
}

type fakeCardServiceAuthorizedPersonRepo struct {
	people map[uint]*model.AuthorizedPerson
	nextID uint
}

func (r *fakeCardServiceAuthorizedPersonRepo) Create(_ context.Context, person *model.AuthorizedPerson) error {
	r.nextID++
	person.AuthorizedPersonID = r.nextID
	cloned := *person
	r.people[person.AuthorizedPersonID] = &cloned
	return nil
}

func (r *fakeCardServiceAuthorizedPersonRepo) FindByID(_ context.Context, id uint) (*model.AuthorizedPerson, error) {
	person, ok := r.people[id]
	if !ok {
		return nil, nil
	}
	cloned := *person
	return &cloned, nil
}

func (r *fakeCardServiceAuthorizedPersonRepo) ListByAccountNumber(_ context.Context, accountNumber string) ([]model.AuthorizedPerson, error) {
	result := make([]model.AuthorizedPerson, 0)
	for _, person := range r.people {
		if person.AccountNumber == accountNumber {
			result = append(result, *person)
		}
	}
	return result, nil
}

type fakeCardServiceCardRequestRepo struct {
	requests map[uint]*model.CardRequest
	nextID   uint
}

func (r *fakeCardServiceCardRequestRepo) Create(_ context.Context, request *model.CardRequest) error {
	r.nextID++
	request.CardRequestID = r.nextID
	cloned := *request
	r.requests[request.CardRequestID] = &cloned
	return nil
}

func (r *fakeCardServiceCardRequestRepo) FindByAccountNumberAndCode(_ context.Context, accountNumber string, code string) (*model.CardRequest, error) {
	for _, request := range r.requests {
		if request.AccountNumber == accountNumber && request.ConfirmationCode == code {
			cloned := *request
			return &cloned, nil
		}
	}
	return nil, nil
}

func (r *fakeCardServiceCardRequestRepo) FindLatestPendingByAccountNumber(_ context.Context, accountNumber string) (*model.CardRequest, error) {
	var latest *model.CardRequest
	for _, request := range r.requests {
		if request.AccountNumber != accountNumber || request.Used || !request.ExpiresAt.After(time.Now()) {
			continue
		}
		if latest == nil || request.CardRequestID > latest.CardRequestID {
			cloned := *request
			latest = &cloned
		}
	}
	return latest, nil
}

func (r *fakeCardServiceCardRequestRepo) Update(_ context.Context, request *model.CardRequest) error {
	cloned := *request
	r.requests[request.CardRequestID] = &cloned
	return nil
}

type fakeCardServiceUserClient struct {
	clientResp   *pb.GetClientByIdResponse
	clientErr    error
	employeeResp *pb.GetEmployeeByIdResponse
	employeeErr  error
}

func (f *fakeCardServiceUserClient) GetClientByID(_ context.Context, _ uint) (*pb.GetClientByIdResponse, error) {
	return f.clientResp, f.clientErr
}

func (f *fakeCardServiceUserClient) GetEmployeeByID(_ context.Context, _ uint) (*pb.GetEmployeeByIdResponse, error) {
	return f.employeeResp, f.employeeErr
}

type sentEmail struct {
	to      string
	subject string
	body    string
}

type fakeCardServiceMailer struct {
	sendErr error
	sent    []sentEmail
}

func (f *fakeCardServiceMailer) Send(to, subject, body string) error {
	if f.sendErr != nil {
		return f.sendErr
	}

	f.sent = append(f.sent, sentEmail{
		to:      to,
		subject: subject,
		body:    body,
	})
	return nil
}

func newCardServiceForTests(
	accountRepo repository.AccountRepository,
	cardRepo repository.CardRepository,
	authorizedPersonRepo repository.AuthorizedPersonRepository,
	cardRequestRepo repository.CardRequestRepository,
	userClient client.UserClient,
	mailer Mailer,
) *CardService {
	if userClient == nil {
		userClient = &fakeCardServiceUserClient{
			clientResp: &pb.GetClientByIdResponse{
				Id:       1,
				Email:    "owner@example.com",
				FullName: "Owner Client",
			},
		}
	}
	if mailer == nil {
		mailer = &fakeCardServiceMailer{}
	}

	return &CardService{
		accountRepo:          accountRepo,
		cardRepo:             cardRepo,
		authorizedPersonRepo: authorizedPersonRepo,
		cardRequestRepo:      cardRequestRepo,
		userClient:           userClient,
		mailer:               mailer,
		txManager:            &fakeBankingTxManager{},
	}
}
func TestRequestCardPersonalSuccess(t *testing.T) {
	t.Parallel()

	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-1": {
				AccountNumber: "ACC-1",
				ClientID:      1,
				AccountType:   model.AccountTypePersonal,
			},
		},
	}
	cardRepo := &fakeCardServiceCardRepo{
		cards:        map[uint]*model.Card{},
		existingPANs: map[string]bool{},
	}
	personRepo := &fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}}
	requestRepo := &fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}}

	mailer := &fakeCardServiceMailer{}
	svc := newCardServiceForTests(accountRepo, cardRepo, personRepo, requestRepo, nil, mailer)
	ctx := clientContext(1)

	request, err := svc.RequestCard(ctx, &RequestCardInput{AccountNumber: "ACC-1"})

	require.NoError(t, err)
	require.NotNil(t, request)
	require.Equal(t, "ACC-1", request.AccountNumber)
	require.False(t, request.ForAuthorizedPerson)
	require.Len(t, request.ConfirmationCode, 6)
}

func TestRequestCardBusinessOwnerLimitReached(t *testing.T) {
	t.Parallel()

	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"BUS-1": {
				AccountNumber: "BUS-1",
				ClientID:      1,
				AccountType:   model.AccountTypeBusiness,
			},
		},
	}
	cardRepo := &fakeCardServiceCardRepo{
		cards: map[uint]*model.Card{
			1: {
				CardID:        1,
				AccountNumber: "BUS-1",
				Status:        model.CardStatusActive,
			},
		},
		existingPANs: map[string]bool{},
	}
	personRepo := &fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}}
	requestRepo := &fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}}

	mailer := &fakeCardServiceMailer{}
	svc := newCardServiceForTests(accountRepo, cardRepo, personRepo, requestRepo, nil, mailer)
	ctx := clientContext(1)

	_, err := svc.RequestCard(ctx, &RequestCardInput{AccountNumber: "BUS-1"})

	require.Error(t, err)
	require.Contains(t, err.Error(), "business account owner already has a card")
}

func TestConfirmCardRequestCreatesAuthorizedPersonCard(t *testing.T) {
	t.Parallel()

	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"BUS-2": {
				AccountNumber: "BUS-2",
				ClientID:      1,
				AccountType:   model.AccountTypeBusiness,
				MonthlyLimit:  1000000,
			},
		},
	}
	cardRepo := &fakeCardServiceCardRepo{
		cards:        map[uint]*model.Card{},
		existingPANs: map[string]bool{},
	}
	personRepo := &fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}}
	requestRepo := &fakeCardServiceCardRequestRepo{
		requests: map[uint]*model.CardRequest{
			1: {
				CardRequestID:             1,
				AccountNumber:             "BUS-2",
				ConfirmationCode:          "123456",
				ExpiresAt:                 time.Now().Add(10 * time.Minute),
				ForAuthorizedPerson:       true,
				AuthorizedPersonFirstName: stringPointer("Ana"),
				AuthorizedPersonLastName:  stringPointer("Petrovic"),
				AuthorizedPersonEmail:     stringPointer("ana@example.com"),
			},
		},
		nextID: 1,
	}

	mailer := &fakeCardServiceMailer{}
	svc := newCardServiceForTests(accountRepo, cardRepo, personRepo, requestRepo, nil, mailer)
	ctx := clientContext(1)

	card, err := svc.ConfirmCardRequest(ctx, "BUS-2", "123456")

	require.NoError(t, err)
	require.NotNil(t, card)
	require.NotNil(t, card.AuthorizedPersonID)
	require.Equal(t, model.CardStatusActive, card.Status)
	require.Len(t, card.CardNumber, 16)
	require.True(t, isValidLuhn(card.CardNumber))

	updatedRequest := requestRepo.requests[1]
	require.True(t, updatedRequest.Used)
	require.Len(t, personRepo.people, 1)
}

func TestListCardsForAccountForbiddenForForeignClient(t *testing.T) {
	t.Parallel()

	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-2": {
				AccountNumber: "ACC-2",
				ClientID:      2,
				AccountType:   model.AccountTypePersonal,
			},
		},
	}
	cardRepo := &fakeCardServiceCardRepo{
		cards:        map[uint]*model.Card{},
		existingPANs: map[string]bool{},
	}
	personRepo := &fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}}
	requestRepo := &fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}}

	mailer := &fakeCardServiceMailer{}
	svc := newCardServiceForTests(accountRepo, cardRepo, personRepo, requestRepo, nil, mailer)
	ctx := clientContext(1)

	_, err := svc.ListCardsForAccount(ctx, "ACC-2")

	require.Error(t, err)
	require.Contains(t, err.Error(), "account does not belong to authenticated client")
}

func TestBlockCardClientOwnCardSuccess(t *testing.T) {
	t.Parallel()

	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-3": {
				AccountNumber: "ACC-3",
				ClientID:      1,
				AccountType:   model.AccountTypePersonal,
			},
		},
	}
	cardRepo := &fakeCardServiceCardRepo{
		cards: map[uint]*model.Card{
			1: {
				CardID:        1,
				AccountNumber: "ACC-3",
				Status:        model.CardStatusActive,
			},
		},
		existingPANs: map[string]bool{},
	}
	personRepo := &fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}}
	requestRepo := &fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}}

	mailer := &fakeCardServiceMailer{}
	svc := newCardServiceForTests(accountRepo, cardRepo, personRepo, requestRepo, nil, mailer)
	ctx := clientContext(1)

	card, err := svc.BlockCard(ctx, 1)

	require.NoError(t, err)
	require.Equal(t, model.CardStatusBlocked, card.Status)
}

func TestBlockCardClientCannotBlockAuthorizedPersonCard(t *testing.T) {
	t.Parallel()

	authorizedPersonID := uint(7)

	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"BUS-3": {
				AccountNumber: "BUS-3",
				ClientID:      1,
				AccountType:   model.AccountTypeBusiness,
			},
		},
	}
	cardRepo := &fakeCardServiceCardRepo{
		cards: map[uint]*model.Card{
			1: {
				CardID:             1,
				AccountNumber:      "BUS-3",
				Status:             model.CardStatusActive,
				AuthorizedPersonID: &authorizedPersonID,
			},
		},
		existingPANs: map[string]bool{},
	}
	personRepo := &fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}}
	requestRepo := &fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}}

	mailer := &fakeCardServiceMailer{}
	svc := newCardServiceForTests(accountRepo, cardRepo, personRepo, requestRepo, nil, mailer)
	ctx := clientContext(1)

	_, err := svc.BlockCard(ctx, 1)

	require.Error(t, err)
	require.Contains(t, err.Error(), "client can block only their own card")
}

func TestUnblockCardEmployeeSuccess(t *testing.T) {
	t.Parallel()

	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-4": {
				AccountNumber: "ACC-4",
				ClientID:      1,
				AccountType:   model.AccountTypePersonal,
			},
		},
	}
	cardRepo := &fakeCardServiceCardRepo{
		cards: map[uint]*model.Card{
			1: {
				CardID:        1,
				AccountNumber: "ACC-4",
				Status:        model.CardStatusBlocked,
			},
		},
		existingPANs: map[string]bool{},
	}
	personRepo := &fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}}
	requestRepo := &fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}}

	mailer := &fakeCardServiceMailer{}
	svc := newCardServiceForTests(accountRepo, cardRepo, personRepo, requestRepo, nil, mailer)
	ctx := employeeContext(11)

	card, err := svc.UnblockCard(ctx, 1)

	require.NoError(t, err)
	require.Equal(t, model.CardStatusActive, card.Status)
}

func TestDeactivateCardEmployeeSuccess(t *testing.T) {
	t.Parallel()

	accountRepo := &fakeCardServiceAccountRepo{
		accounts: map[string]*model.Account{
			"ACC-5": {
				AccountNumber: "ACC-5",
				ClientID:      1,
				AccountType:   model.AccountTypePersonal,
			},
		},
	}
	cardRepo := &fakeCardServiceCardRepo{
		cards: map[uint]*model.Card{
			1: {
				CardID:        1,
				AccountNumber: "ACC-5",
				Status:        model.CardStatusActive,
			},
		},
		existingPANs: map[string]bool{},
	}
	personRepo := &fakeCardServiceAuthorizedPersonRepo{people: map[uint]*model.AuthorizedPerson{}}
	requestRepo := &fakeCardServiceCardRequestRepo{requests: map[uint]*model.CardRequest{}}

	mailer := &fakeCardServiceMailer{}
	svc := newCardServiceForTests(accountRepo, cardRepo, personRepo, requestRepo, nil, mailer)
	ctx := employeeContext(12)

	card, err := svc.DeactivateCard(ctx, 1)

	require.NoError(t, err)
	require.Equal(t, model.CardStatusDeactivated, card.Status)
}

func clientContext(clientID uint) context.Context {
	return auth.SetAuthOnContext(context.Background(), &auth.AuthContext{
		IdentityType: auth.IdentityClient,
		ClientID:     &clientID,
	})
}

func employeeContext(employeeID uint) context.Context {
	return auth.SetAuthOnContext(context.Background(), &auth.AuthContext{
		IdentityType: auth.IdentityEmployee,
		EmployeeID:   &employeeID,
	})
}

func stringPointer(value string) *string {
	return &value
}
