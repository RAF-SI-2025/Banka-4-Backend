package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	pkgerrors "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/service"
)

type OTCHandler struct {
	service *service.OTCService
}

func NewOTCHandler(service *service.OTCService) *OTCHandler {
	return &OTCHandler{service: service}
}

// PublishAssetClient godoc
// @Summary Publish assets for OTC trading
// @Description Appends the number of assets the caller makes publicly visible on the OTC portal.
// The amount replaces the current public amount. Must be non-negative and cannot exceed
// owned minus reserved. Accessible by the owning client or actuary only.
// @Tags otc
// @Accept json
// @Param ownershipId path int true "Asset ownership ID"
// @Param request body dto.PublishAssetRequest true "Amount to make public"
// @Success 204
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Failure 500 {object} errors.AppError
// @Security BearerAuth
// @Router /api/client/{clientId}/assets/{ownershipId}/publish [patch]
func (h *OTCHandler) PublishAssetClient(c *gin.Context) {
	ownershipID, err := strconv.ParseUint(c.Param("ownershipId"), 10, 64)
	if err != nil {
		_ = c.Error(pkgerrors.BadRequestErr("invalid asset ownership id"))
		return
	}

	clientId, err := strconv.ParseUint(c.Param("clientId"), 10, 64)
	if err != nil {
		_ = c.Error(pkgerrors.BadRequestErr("invalid client id"))
		return
	}

	var req dto.PublishAssetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(pkgerrors.BadRequestErr("invalid request body"))
		return
	}
	amount := req.Amount

	ownerType := model.OwnerTypeClient

	if svcErr := h.service.PublishAsset(c.Request.Context(), uint(ownershipID), uint(clientId), ownerType, amount); svcErr != nil {
		_ = c.Error(svcErr)
		return
	}

	c.Status(http.StatusNoContent)
}

// PublishAssetActuary godoc
// @Summary Publish assets for OTC trading
// @Description Appends the number of assets the caller makes publicly visible on the OTC portal.
// The amount replaces the current public amount. Must be non-negative and cannot exceed
// owned minus reserved. Accessible by the owning client or actuary only.
// @Tags otc
// @Accept json
// @Param ownershipId path int true "Asset ownership ID"
// @Param request body dto.PublishAssetRequest true "Amount to make public"
// @Success 204
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Failure 500 {object} errors.AppError
// @Security BearerAuth
// @Router /api/actuary/{actId}/assets/{ownershipId}/publish [patch]
func (h *OTCHandler) PublishAssetActuary(c *gin.Context) {
	ownershipID, err := strconv.ParseUint(c.Param("ownershipId"), 10, 64)
	if err != nil {
		_ = c.Error(pkgerrors.BadRequestErr("invalid asset ownership id"))
		return
	}

	actuaryId, err := strconv.ParseUint(c.Param("actId"), 10, 64)
	if err != nil {
		_ = c.Error(pkgerrors.BadRequestErr("invalid actuary id"))
		return
	}

	var req dto.PublishAssetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(pkgerrors.BadRequestErr("invalid request body"))
		return
	}
	amount := req.Amount

	ownerType := model.OwnerTypeActuary

	if svcErr := h.service.PublishAsset(c.Request.Context(), uint(ownershipID), uint(actuaryId), ownerType, amount); svcErr != nil {
		_ = c.Error(svcErr)
		return
	}

	c.Status(http.StatusNoContent)
}

// GetPublicOTCAssets godoc
// @Summary List all publicly available OTC assets
// @Description Returns a paginated list of assets that have been marked public by their owners on the OTC portal.
// Each entry includes name, ticker, security type, current price, currency, available amount, last updated timestamp, and owner name.
// @Tags otc
// @Produce json
// @Param page query int false "Page number (default 1)"
// @Param page_size query int false "Page size (default 10)"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} errors.AppError
// @Failure 500 {object} errors.AppError
// @Security BearerAuth
// @Router /api/otc/public [get]
func (h *OTCHandler) GetPublicOTCAssets(c *gin.Context) {
	var q dto.OTCListRequest
	if err := c.ShouldBindQuery(&q); err != nil {
		_ = c.Error(pkgerrors.BadRequestErr(err.Error()))
		return
	}
	q.Normalize()

	assets, total, err := h.service.GetPublicOTCAssets(c.Request.Context(), q.Page, q.PageSize)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      assets,
		"total":     total,
		"page":      q.Page,
		"page_size": q.PageSize,
	})
}
