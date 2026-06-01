package handler

import (
	"net/http"
	"strconv"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/service"
	"github.com/gin-gonic/gin"
)

type DividendHandler struct {
	dividendService *service.DividendPayoutService
}

func NewDividendHandler(dividendService *service.DividendPayoutService) *DividendHandler {
	return &DividendHandler{dividendService: dividendService}
}

// GetAllDividendPayouts godoc
// @Summary List all dividend payouts
// @Description Returns all dividend payout records. Restricted to supervisors.
// @Tags dividends
// @Produce json
// @Success 200 {object} dto.ListDividendPayoutsResponse
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Security BearerAuth
// @Router /api/dividends [get]
func (h *DividendHandler) GetAllDividendPayouts(c *gin.Context) {
	ctx := c.Request.Context()

	payouts, err := h.dividendService.GetAllPayouts(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, dto.ListDividendPayoutsResponse{
		Data: mapPayouts(payouts),
	})
}

// GetDividendPayoutsForAssetOwnership godoc
// @Summary List dividend payouts for a portfolio position
// @Description Returns dividend payout history for a specific asset ownership (position).
// @Tags dividends
// @Produce json
// @Param assetOwnershipId path int true "Asset Ownership ID"
// @Success 200 {object} dto.ListDividendPayoutsResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Security BearerAuth
// @Router /api/portfolio/assets/{assetOwnershipId}/dividends [get]
func (h *DividendHandler) GetDividendPayoutsForAssetOwnership(c *gin.Context) {
	ctx := c.Request.Context()

	idStr := c.Param("assetOwnershipId")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		_ = c.Error(errors.BadRequestErr("invalid assetOwnershipId"))
		return
	}

	payouts, err := h.dividendService.GetPayoutsForAssetOwnership(ctx, uint(id))
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, dto.ListDividendPayoutsResponse{
		Data: mapPayouts(payouts),
	})
}

// TriggerDividends godoc
// @Summary Manually trigger dividend payout
// @Description Forces immediate dividend processing. For internal use only.
// @Tags dividends
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 500 {object} errors.AppError
// @Security BearerAuth
// @Router /api/dividends/trigger [post]
func (h *DividendHandler) TriggerDividends(c *gin.Context) {
	if err := h.dividendService.ProcessDividends(c.Request.Context()); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "dividends processed"})
}

func mapPayouts(payouts []model.DividendPayout) []dto.DividendPayoutResponse {
	result := make([]dto.DividendPayoutResponse, 0, len(payouts))
	for _, p := range payouts {
		ticker := p.AssetOwnership.Asset.Ticker
		result = append(result, dto.DividendPayoutResponse{
			DividendPayoutID: p.DividendPayoutID,
			AssetOwnershipID: p.AssetOwnershipID,
			Stock:            ticker,
			Quantity:         p.Quantity,
			GrossAmount:      p.GrossAmount,
			TaxAmount:        p.TaxAmount,
			NetAmount:        p.NetAmount,
			CurrencyCode:     p.CurrencyCode,
			AccountNumber:    p.AccountNumber,
			PaymentDate:      p.PaymentDate,
		})
	}
	return result
}
