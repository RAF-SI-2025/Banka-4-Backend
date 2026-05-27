package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/service"
)

type WatchlistHandler struct {
	svc *service.WatchlistService
}

func NewWatchlistHandler(svc *service.WatchlistService) *WatchlistHandler {
	return &WatchlistHandler{svc: svc}
}

func parseWatchlistID(c *gin.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("watchlistId"), 10, 64)
	if err != nil {
		return 0, errors.BadRequestErr("invalid watchlist id")
	}
	return uint(id), nil
}

// GetWatchlists godoc
// @Summary List watchlists
// @Description Lists every watchlist owned by the authenticated user (client or actuary), with item counts.
// @Tags watchlists
// @Produce json
// @Success 200 {array} dto.WatchlistResponse
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Security BearerAuth
// @Router /api/watchlists [get]
func (h *WatchlistHandler) GetWatchlists(c *gin.Context) {
	result, err := h.svc.GetWatchlists(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// CreateWatchlist godoc
// @Summary Create a watchlist
// @Description Creates a new, empty watchlist for the authenticated user. Names are unique per user.
// @Tags watchlists
// @Accept json
// @Produce json
// @Param request body dto.CreateWatchlistRequest true "Watchlist name"
// @Success 201 {object} dto.WatchlistResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 409 {object} errors.AppError
// @Security BearerAuth
// @Router /api/watchlists [post]
func (h *WatchlistHandler) CreateWatchlist(c *gin.Context) {
	var req dto.CreateWatchlistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	result, err := h.svc.CreateWatchlist(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, result)
}

// GetWatchlistDetail godoc
// @Summary Get watchlist details
// @Description Returns a single watchlist with all of its tracked listings, each enriched with current price, daily change and volume. Optionally filtered by asset type.
// @Tags watchlists
// @Produce json
// @Param watchlistId path int true "Watchlist ID"
// @Param asset_type query string false "Filter by asset type: stock|option|future|forexPair"
// @Success 200 {object} dto.WatchlistDetailResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Security BearerAuth
// @Router /api/watchlists/{watchlistId} [get]
func (h *WatchlistHandler) GetWatchlistDetail(c *gin.Context) {
	id, err := parseWatchlistID(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	result, err := h.svc.GetWatchlistDetail(c.Request.Context(), id, c.Query("asset_type"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// DeleteWatchlist godoc
// @Summary Delete a watchlist
// @Description Deletes one of the authenticated user's watchlists and all of its items.
// @Tags watchlists
// @Param watchlistId path int true "Watchlist ID"
// @Success 204
// @Failure 401 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Security BearerAuth
// @Router /api/watchlists/{watchlistId} [delete]
func (h *WatchlistHandler) DeleteWatchlist(c *gin.Context) {
	id, err := parseWatchlistID(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	if err := h.svc.DeleteWatchlist(c.Request.Context(), id); err != nil {
		_ = c.Error(err)
		return
	}
	c.Status(http.StatusNoContent)
}

// AddListing godoc
// @Summary Add a listing to a watchlist
// @Description Adds a listing (stock, option, future or forex pair) to one of the authenticated user's watchlists.
// @Tags watchlists
// @Accept json
// @Produce json
// @Param watchlistId path int true "Watchlist ID"
// @Param request body dto.AddWatchlistItemRequest true "Listing to add"
// @Success 204
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Failure 409 {object} errors.AppError
// @Security BearerAuth
// @Router /api/watchlists/{watchlistId}/items [post]
func (h *WatchlistHandler) AddListing(c *gin.Context) {
	id, err := parseWatchlistID(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	var req dto.AddWatchlistItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	if err := h.svc.AddListing(c.Request.Context(), id, req); err != nil {
		_ = c.Error(err)
		return
	}
	c.Status(http.StatusNoContent)
}

// RemoveListing godoc
// @Summary Remove a listing from a watchlist
// @Description Removes a listing from one of the authenticated user's watchlists.
// @Tags watchlists
// @Param watchlistId path int true "Watchlist ID"
// @Param listingId path int true "Listing ID"
// @Success 204
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Security BearerAuth
// @Router /api/watchlists/{watchlistId}/items/{listingId} [delete]
func (h *WatchlistHandler) RemoveListing(c *gin.Context) {
	id, err := parseWatchlistID(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	listingID, err := strconv.ParseUint(c.Param("listingId"), 10, 64)
	if err != nil {
		_ = c.Error(errors.BadRequestErr("invalid listing id"))
		return
	}

	if err := h.svc.RemoveListing(c.Request.Context(), id, uint(listingID)); err != nil {
		_ = c.Error(err)
		return
	}
	c.Status(http.StatusNoContent)
}
