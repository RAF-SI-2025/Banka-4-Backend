package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	app_errors "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Mocks ---

// mockOtcHistoryService is a mock type for the OtcNegotiationHistoryService interface
type mockOtcHistoryServiceForHandler struct {
	mock.Mock
}

// CreateNegotiationHistory provides a mock function with given fields: ctx, offerID, oldOffer, newOffer, modifiedBy
func (_m *mockOtcHistoryServiceForHandler) CreateNegotiationHistory(ctx context.Context, offerID uint, oldOffer *model.OtcOffer, newOffer *model.OtcOffer, modifiedBy uint) error {
	ret := _m.Called(ctx, offerID, oldOffer, newOffer, modifiedBy)
	return ret.Error(0)
}

// GetNegotiationHistory provides a mock function with given fields: ctx, offerID, statusFilter, dateFrom, dateTo, counterpartyFilter
func (_m *mockOtcHistoryServiceForHandler) GetNegotiationHistory(ctx context.Context, offerID uint, statusFilter string, dateFrom *time.Time, dateTo *time.Time, counterpartyFilter uint) ([]*model.OtcNegotiationHistory, error) {
	ret := _m.Called(ctx, offerID, statusFilter, dateFrom, dateTo, counterpartyFilter)

	var r0 []*model.OtcNegotiationHistory
	if rf, ok := ret.Get(0).(func(context.Context, uint, string, *time.Time, *time.Time, uint) []*model.OtcNegotiationHistory); ok {
		r0 = rf(ctx, offerID, statusFilter, dateFrom, dateTo, counterpartyFilter)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*model.OtcNegotiationHistory)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, uint, string, *time.Time, *time.Time, uint) error); ok {
		r1 = rf(ctx, offerID, statusFilter, dateFrom, dateTo, counterpartyFilter)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// --- Test Setup ---

func setupHistoryHandlerTest() (*gin.Engine, *mockOtcHistoryServiceForHandler) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(app_errors.ErrorHandler())
	mockService := new(mockOtcHistoryServiceForHandler)
	var serviceInterface service.OtcNegotiationHistoryService = mockService
	historyHandler := NewOtcNegotiationHistoryHandler(serviceInterface)
	router.GET("/api/otc/offers/:id/history", historyHandler.GetNegotiationHistory)
	return router, mockService
}

// --- Tests ---

func TestGetNegotiationHistoryHandler_HappyPath(t *testing.T) {
	router, mockService := setupHistoryHandlerTest()

	expectedHistory := []*model.OtcNegotiationHistory{
		{OtcNegotiationHistoryID: 1, OtcOfferID: 1},
	}
	mockService.On("GetNegotiationHistory", mock.Anything, uint(1), "ACCEPTED", mock.AnythingOfType("*time.Time"), mock.AnythingOfType("*time.Time"), uint(123)).Return(expectedHistory, nil)

	req, _ := http.NewRequest("GET", "/api/otc/offers/1/history?status=ACCEPTED&counterparty=123", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var responseBody []*model.OtcNegotiationHistory
	err := json.Unmarshal(w.Body.Bytes(), &responseBody)
	assert.NoError(t, err)
	assert.Equal(t, expectedHistory, responseBody)
	mockService.AssertExpectations(t)
}

func TestGetNegotiationHistoryHandler_ServiceError(t *testing.T) {
	router, mockService := setupHistoryHandlerTest()

	expectedError := app_errors.NotFoundErr("offer not found")
	mockService.On("GetNegotiationHistory", mock.Anything, uint(1), "", mock.AnythingOfType("*time.Time"), mock.AnythingOfType("*time.Time"), uint(0)).Return(nil, expectedError)

	req, _ := http.NewRequest("GET", "/api/otc/offers/1/history", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var errorResponse app_errors.AppError
	err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
	assert.NoError(t, err)
	assert.Equal(t, "Not Found", errorResponse.Status)
	assert.Equal(t, "offer not found", errorResponse.Message)
	mockService.AssertExpectations(t)
}

func TestGetNegotiationHistoryHandler_InvalidUrlParams(t *testing.T) {
	tt := []struct {
		name        string
		url         string
		expectedMsg string
	}{
		{"Invalid offer ID", "/api/otc/offers/abc/history", "invalid offer id"},
		{"Invalid counterparty", "/api/otc/offers/1/history?counterparty=xyz", "invalid counterparty"},
		{"Invalid from date", "/api/otc/offers/1/history?from=not-a-date", "invalid from date"},
		{"Invalid to date", "/api/otc/offers/1/history?to=not-a-date", "invalid to date"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			router, _ := setupHistoryHandlerTest()

			req, _ := http.NewRequest("GET", tc.url, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			var errorResponse app_errors.AppError
			err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
			assert.NoError(t, err)
			assert.Equal(t, "Bad Request", errorResponse.Status)
			assert.Contains(t, errorResponse.Message, tc.expectedMsg)
		})
	}
}
