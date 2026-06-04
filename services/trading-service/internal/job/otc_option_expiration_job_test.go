package job

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/stretchr/testify/require"
)

//
// ─────────────────────────────
// FAKE REPO
// ─────────────────────────────
//

type fakeContractRepo struct {
	contracts []model.OtcOptionContract
	err       error
}

func (f *fakeContractRepo) FindExpiringContracts(ctx context.Context, before time.Time) ([]model.OtcOptionContract, error) {
	return f.contracts, f.err
}

// no-op methods (implementacija interfejsa)
func (f *fakeContractRepo) Create(ctx context.Context, c *model.OtcOptionContract) error { return nil }
func (f *fakeContractRepo) Save(ctx context.Context, c *model.OtcOptionContract) error   { return nil }

func (f *fakeContractRepo) FindByID(ctx context.Context, id uint) (*model.OtcOptionContract, error) {
	return nil, nil
}

func (f *fakeContractRepo) FindByIDForUpdate(ctx context.Context, id uint) (*model.OtcOptionContract, error) {
	return nil, nil
}

func (f *fakeContractRepo) FindByOfferID(ctx context.Context, offerID uint) (*model.OtcOptionContract, error) {
	return nil, nil
}

func (f *fakeContractRepo) FindForUser(ctx context.Context, userID uint) ([]model.OtcOptionContract, error) {
	return nil, nil
}

func (f *fakeContractRepo) FindActiveBySellerAndStock(ctx context.Context, sellerID, stockAssetID uint, now time.Time) ([]model.OtcOptionContract, error) {
	return nil, nil
}

func (f *fakeContractRepo) FindExpiredActive(ctx context.Context, before time.Time, limit int) ([]model.OtcOptionContract, error) {
	return nil, nil
}

//
// ─────────────────────────────
// FAKE USER CLIENT
// ─────────────────────────────
//

type fakeUserClient struct {
	called bool
}

func (f *fakeUserClient) GetClientById(ctx context.Context, id uint64) (*pb.GetClientByIdResponse, error) {
	return &pb.GetClientByIdResponse{
		Id:    id,
		Email: "user@example.com",
	}, nil
}

func (f *fakeUserClient) GetClientByIdentityId(ctx context.Context, identityId uint64) (*pb.GetClientByIdResponse, error) {
	return &pb.GetClientByIdResponse{
		Id:    identityId,
		Email: "user@example.com",
	}, nil
}

func (f *fakeUserClient) GetEmployeeById(ctx context.Context, id uint64) (*pb.GetEmployeeByIdResponse, error) {
	return &pb.GetEmployeeByIdResponse{
		Id:    id,
		Email: "employee@example.com",
	}, nil
}

func (f *fakeUserClient) GetEmployeeByIdentityId(ctx context.Context, identityId uint64) (*pb.GetEmployeeByIdResponse, error) {
	return &pb.GetEmployeeByIdResponse{
		Id:    identityId,
		Email: "employee@example.com",
	}, nil
}

func (f *fakeUserClient) GetAllClients(ctx context.Context, page, pageSize int32, firstName, lastName string) (*pb.GetAllClientsResponse, error) {
	return &pb.GetAllClientsResponse{}, nil
}

func (f *fakeUserClient) GetAllActuaries(ctx context.Context, page, pageSize int32, firstName, lastName string) (*pb.GetAllActuariesResponse, error) {
	return &pb.GetAllActuariesResponse{}, nil
}

func (f *fakeUserClient) GetIdentityByUserId(ctx context.Context, userID uint64, userType string) (*pb.GetIdentityByUserIdResponse, error) {
	return &pb.GetIdentityByUserIdResponse{}, nil
}

func (f *fakeUserClient) IncrementUsedLimit(ctx context.Context, employeeID uint64, amount float64) (*pb.IncrementUsedLimitResponse, error) {
	return &pb.IncrementUsedLimitResponse{}, nil
}

func (f *fakeUserClient) GetClientsByIds(ctx context.Context, ids []uint64) (*pb.GetClientsByIdsResponse, error) {
	f.called = true

	clients := make([]*pb.GetClientByIdResponse, 0, len(ids))
	for _, id := range ids {
		clients = append(clients, &pb.GetClientByIdResponse{
			Id:    id,
			Email: fmt.Sprintf("user-%d@test.com", id),
		})
	}

	return &pb.GetClientsByIdsResponse{
		Clients: clients,
	}, nil
}

//
// ─────────────────────────────
// FAKE MAILER
// ─────────────────────────────
//

type fakeEmailSender struct {
	sent []string
}

func (f *fakeEmailSender) Send(to, subject, body string) error {
	f.sent = append(f.sent, to+"|"+subject+"|"+body)
	return nil
}

//
// ─────────────────────────────
// TEST
// ─────────────────────────────
//

func TestOtcOptionExpirationJob_Run_Success(t *testing.T) {
	ctx := context.Background()

	repo := &fakeContractRepo{
		contracts: []model.OtcOptionContract{
			{
				OtcOptionContractID: 1,
				BuyerID:             1,
				SellerID:            2,
			},
		},
	}

	userClient := &fakeUserClient{}
	mailer := &fakeEmailSender{}

	job := &OtcOptionExpirationJob{
		contractRepo: repo,
		userClient:   userClient,
		mailer:       mailer,
		now:          func() time.Time { return time.Now() },
	}

	err := job.Run(ctx)

	require.NoError(t, err)

	// user client mora biti pozvan
	require.True(t, userClient.called)

	// 2 emaila (buyer + seller)
	require.Len(t, mailer.sent, 2)

	// proveri sadržaj emailova
	for _, s := range mailer.sent {
		require.Contains(t, s, "OTC Contract Expiring Soon")
		require.Contains(t, s, "Contract #1 expires in 3 days.")
	}
}
