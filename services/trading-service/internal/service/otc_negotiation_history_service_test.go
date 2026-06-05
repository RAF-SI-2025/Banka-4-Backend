package service

import (
	"context"
	"errors"
	"testing"
	"time"

	app_errors "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// --- Fakes ---

type fakeOtcNegotiationHistoryRepo struct {
	history []*model.OtcNegotiationHistory
	err     error
}

func (r *fakeOtcNegotiationHistoryRepo) Create(_ context.Context, h *model.OtcNegotiationHistory) error {
	if r.err != nil {
		return r.err
	}
	r.history = append(r.history, h)
	return nil
}

func (r *fakeOtcNegotiationHistoryRepo) GetByOfferID(_ context.Context, offerID uint) ([]*model.OtcNegotiationHistory, error) {
	if r.err != nil {
		return nil, r.err
	}
	var results []*model.OtcNegotiationHistory
	for _, h := range r.history {
		if h.OtcOfferID == offerID {
			results = append(results, h)
		}
	}
	return results, nil
}

type fakeOtcOfferRepoWithErrors struct {
	*fakeOtcOfferRepo
	findByIDErr error
}

func (r *fakeOtcOfferRepoWithErrors) FindByID(_ context.Context, id uint) (*model.OtcOffer, error) {
	if r.findByIDErr != nil {
		return nil, r.findByIDErr
	}
	return r.fakeOtcOfferRepo.FindByID(context.Background(), id)
}

// --- Test Setup ---

func setupHistoryTest() (OtcNegotiationHistoryService, *fakeOtcOfferRepoWithErrors, *fakeOtcNegotiationHistoryRepo) {
	offerRepo := &fakeOtcOfferRepoWithErrors{fakeOtcOfferRepo: newFakeOtcOfferRepo()}
	historyRepo := &fakeOtcNegotiationHistoryRepo{}
	service := NewOtcNegotiationHistoryService(offerRepo, historyRepo)
	return service, offerRepo, historyRepo
}

// --- Tests ---

func TestGetNegotiationHistory_HappyPath(t *testing.T) {
	service, offerRepo, historyRepo := setupHistoryTest()

	// Seed data
	offer := &model.OtcOffer{
		OtcOfferID: 1,
		Status:     model.OtcOfferStatusAccepted,
	}
	_ = offerRepo.Create(context.Background(), offer)
	_ = historyRepo.Create(context.Background(), &model.OtcNegotiationHistory{OtcOfferID: 1})

	history, err := service.GetNegotiationHistory(context.Background(), 1, "", nil, nil, 0)

	require.NoError(t, err)
	assert.Len(t, history, 1)
}

func TestGetNegotiationHistory_OfferNotFound(t *testing.T) {
	tt := []struct {
		name          string
		repoErr       error
		expectedError error
	}{
		{
			name:          "GORM record not found",
			repoErr:       gorm.ErrRecordNotFound,
			expectedError: app_errors.NotFoundErr("offer not found"),
		},
		{
			name:          "Other DB error",
			repoErr:       errors.New("some other db error"),
			expectedError: app_errors.InternalErr(errors.New("some other db error")),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			service, offerRepo, _ := setupHistoryTest()
			offerRepo.findByIDErr = tc.repoErr

			_, err := service.GetNegotiationHistory(context.Background(), 1, "", nil, nil, 0)

			require.Error(t, err)
			assert.Equal(t, tc.expectedError.Error(), err.Error())
		})
	}
}

func TestGetNegotiationHistory_OfferIsActive(t *testing.T) {
	service, offerRepo, _ := setupHistoryTest()

	offer := &model.OtcOffer{
		OtcOfferID: 1,
		Status:     model.OtcOfferStatusActive,
	}
	_ = offerRepo.Create(context.Background(), offer)

	_, err := service.GetNegotiationHistory(context.Background(), 1, "", nil, nil, 0)

	require.Error(t, err)
	assert.Equal(t, app_errors.BadRequestErr("cannot view history of an active negotiation").Error(), err.Error())
}

func TestCreateNegotiationHistory(t *testing.T) {
	service, _, historyRepo := setupHistoryTest()

	oldOffer := &model.OtcOffer{Amount: 100}
	newOffer := &model.OtcOffer{Amount: 120}

	err := service.CreateNegotiationHistory(context.Background(), 1, oldOffer, newOffer, 123)

	require.NoError(t, err)
	require.Len(t, historyRepo.history, 1)
	h := historyRepo.history[0]
	assert.Equal(t, uint(1), h.OtcOfferID)
	assert.Equal(t, 100, h.OldAmount)
	assert.Equal(t, 120, h.NewAmount)
	assert.Equal(t, uint(123), h.ModifiedBy)
	assert.WithinDuration(t, time.Now(), h.Timestamp, time.Second)
}
