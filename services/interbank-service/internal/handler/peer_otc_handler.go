package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/middleware"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/service"
)

// PeerOtcHandler exposes §3 OTC negotiation endpoints to peer banks.
// Authentication is per-peer via X-Api-Key (the same middleware that
// guards the /interbank §2 endpoint).
type PeerOtcHandler struct {
	service *service.PeerOtcService
}

func NewPeerOtcHandler(svc *service.PeerOtcService) *PeerOtcHandler {
	return &PeerOtcHandler{service: svc}
}

// CreateNegotiation godoc
// @Summary Create OTC negotiation (peer-initiated)
// @Description §3.2 — peer bank's buyer initiates a negotiation against
// @Description a seller in our bank. Returns the ForeignBankId of the
// @Description new negotiation owned by us.
// @Tags interbank-otc
// @Accept json
// @Produce json
// @Param X-Api-Key header string true "Peer bank API key"
// @Param request body dto.OtcOffer true "Initial offer"
// @Success 200 {object} dto.ForeignBankId
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Router /interbank/negotiations [post]
func (h *PeerOtcHandler) CreateNegotiation(c *gin.Context) {
	senderRouting, ok := senderRoutingFromContext(c)
	if !ok {
		return
	}

	var req dto.OtcOffer
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	id, err := h.service.CreateFromPeer(c.Request.Context(), senderRouting, req)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, id)
}

// GetNegotiation godoc
// @Summary Get OTC negotiation
// @Description §3.4 — returns the current state of a negotiation owned by
// @Description this bank. The :rn path parameter must match this bank's
// @Description routing number.
// @Tags interbank-otc
// @Produce json
// @Param X-Api-Key header string true "Peer bank API key"
// @Param rn path int true "Routing number (this bank)"
// @Param id path string true "Negotiation id"
// @Success 200 {object} dto.OtcNegotiation
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Router /interbank/negotiations/{rn}/{id} [get]
func (h *PeerOtcHandler) GetNegotiation(c *gin.Context) {
	rn, ok := parseRoutingNumber(c)
	if !ok {
		return
	}

	id := c.Param("id")
	if id == "" {
		_ = c.Error(errors.BadRequestErr("id is required"))
		return
	}

	negotiation, err := h.service.GetByID(c.Request.Context(), rn, id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, negotiation)
}

// UpdateNegotiation godoc
// @Summary Post counter-offer (peer-initiated)
// @Description §3.3 — STUB. Implementation pending.
// @Tags interbank-otc
// @Router /interbank/negotiations/{rn}/{id} [put]
func (h *PeerOtcHandler) UpdateNegotiation(c *gin.Context) {
	_ = c.Error(errors.NewAppError(http.StatusNotImplemented, "counter-offer not implemented yet", nil))
}

// DeleteNegotiation godoc
// @Summary Close OTC negotiation
// @Description §3.5 — STUB. Implementation pending.
// @Tags interbank-otc
// @Router /interbank/negotiations/{rn}/{id} [delete]
func (h *PeerOtcHandler) DeleteNegotiation(c *gin.Context) {
	_ = c.Error(errors.NewAppError(http.StatusNotImplemented, "close-negotiation not implemented yet", nil))
}

// AcceptNegotiation godoc
// @Summary Accept OTC negotiation
// @Description §3.6 — STUB. Triggers a §2 NEW_TX with the 4 premium +
// @Description option-pseudo postings. Implementation pending (depends
// @Description on §2 protocol layer).
// @Tags interbank-otc
// @Router /interbank/negotiations/{rn}/{id}/accept [get]
func (h *PeerOtcHandler) AcceptNegotiation(c *gin.Context) {
	_ = c.Error(errors.NewAppError(http.StatusNotImplemented, "accept-negotiation not implemented yet", nil))
}

// PublicStock godoc
// @Summary List public stocks
// @Description §3.1 — STUB. Will be wired to trading-service for the list
// @Description of stock holdings marked public for OTC by users in this bank.
// @Tags interbank-otc
// @Router /interbank/public-stock [get]
func (h *PeerOtcHandler) PublicStock(c *gin.Context) {
	_ = c.Error(errors.NewAppError(http.StatusNotImplemented, "public-stock not implemented yet", nil))
}

// UserLookup godoc
// @Summary Resolve user display name
// @Description §3.7 — STUB. Will be wired to user-service.
// @Tags interbank-otc
// @Router /interbank/user/{rn}/{id} [get]
func (h *PeerOtcHandler) UserLookup(c *gin.Context) {
	_ = c.Error(errors.NewAppError(http.StatusNotImplemented, "user-lookup not implemented yet", nil))
}

// senderRoutingFromContext pulls the routing number set by APIKeyAuth.
// Returns (0, false) if missing — error is already written.
func senderRoutingFromContext(c *gin.Context) (int, bool) {
	raw, ok := c.Get(middleware.PeerContextKey)
	if !ok {
		_ = c.Error(errors.UnauthorizedErr("peer routing number missing from context"))
		return 0, false
	}

	rn, ok := raw.(int)
	if !ok {
		_ = c.Error(errors.InternalErr(nil))
		return 0, false
	}

	return rn, true
}

// parseRoutingNumber extracts and validates the :rn path parameter.
func parseRoutingNumber(c *gin.Context) (int, bool) {
	raw := c.Param("rn")
	rn, err := strconv.Atoi(raw)
	if err != nil {
		_ = c.Error(errors.BadRequestErr("rn must be an integer"))
		return 0, false
	}

	return rn, true
}
