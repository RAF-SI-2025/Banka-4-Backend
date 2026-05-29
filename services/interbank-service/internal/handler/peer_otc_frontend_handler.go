package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/service"
)

// PeerOtcFrontendHandler exposes the user-facing /api/peer-otc/* routes.
// These are called by our own clients via JWT (Authorization: Bearer ...);
// the peer-facing /interbank/* routes use X-Api-Key and live in
// PeerOtcHandler.
type PeerOtcFrontendHandler struct {
	service *service.PeerOtcService
}

func NewPeerOtcFrontendHandler(svc *service.PeerOtcService) *PeerOtcFrontendHandler {
	return &PeerOtcFrontendHandler{service: svc}
}

// CreatePeerNegotiationRequest is the user-facing payload for initiating
// a cross-bank OTC negotiation. The buyer is always the authenticated
// user; only the seller and offer terms need to come from the client.
type CreatePeerNegotiationRequest struct {
	SellerID        dto.ForeignBankId `json:"sellerId"        binding:"required"`
	Ticker          string            `json:"ticker"          binding:"required,max=16"`
	Amount          int               `json:"amount"          binding:"required,min=1"`
	PricePerStock   float64           `json:"pricePerStock"   binding:"required"`
	PriceCurrency   string            `json:"priceCurrency"   binding:"required,max=8"`
	Premium         float64           `json:"premium"         binding:"required"`
	PremiumCurrency string            `json:"premiumCurrency" binding:"required,max=8"`
	SettlementDate  string            `json:"settlementDate"  binding:"required"`
}

// CounterPeerNegotiationRequest is the user-facing payload for a
// counter-offer on a cross-bank negotiation. Parties and ticker do not
// change during a negotiation, so they are not part of the request.
type CounterPeerNegotiationRequest struct {
	Amount          int     `json:"amount"          binding:"required,min=1"`
	PricePerStock   float64 `json:"pricePerStock"   binding:"required"`
	PriceCurrency   string  `json:"priceCurrency"   binding:"required,max=8"`
	Premium         float64 `json:"premium"         binding:"required"`
	PremiumCurrency string  `json:"premiumCurrency" binding:"required,max=8"`
	SettlementDate  string  `json:"settlementDate"  binding:"required"`
}

// ListPublicStocks godoc
// @Summary Browse public OTC stocks across all peer banks
// @Description Aggregates §3.1 /public-stock results from every peer in
// @Description the registry. Peers that fail are silently skipped.
// @Tags peer-otc
// @Produce json
// @Success 200 {array} dto.PublicStock
// @Failure 401 {object} errors.AppError
// @Security BearerAuth
// @Router /api/peer-otc/public-stocks [get]
func (h *PeerOtcFrontendHandler) ListPublicStocks(c *gin.Context) {
	stocks, err := h.service.ListAllPeerPublicStocks(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, stocks)
}

// ListMyNegotiations godoc
// @Summary List my cross-bank OTC negotiations
// @Description Returns every cross-bank negotiation in which the
// @Description authenticated user is a party.
// @Tags peer-otc
// @Produce json
// @Success 200 {array} dto.OtcNegotiation
// @Failure 401 {object} errors.AppError
// @Security BearerAuth
// @Router /api/peer-otc/negotiations [get]
func (h *PeerOtcFrontendHandler) ListMyNegotiations(c *gin.Context) {
	userID, err := auth.GetSubjectFromContext(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}

	negotiations, err := h.service.ListMyNegotiations(c.Request.Context(), userID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, negotiations)
}

// CreateNegotiation godoc
// @Summary Initiate a cross-bank OTC negotiation
// @Description The authenticated user becomes the buyer. The request is
// @Description forwarded to the seller's bank via §3.2; on success the
// @Description authoritative id assigned by the peer is returned.
// @Tags peer-otc
// @Accept json
// @Produce json
// @Param request body CreatePeerNegotiationRequest true "Offer terms"
// @Success 201 {object} dto.ForeignBankId
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 502 {object} errors.AppError "Peer unreachable or returned non-2xx"
// @Security BearerAuth
// @Router /api/peer-otc/negotiations [post]
func (h *PeerOtcFrontendHandler) CreateNegotiation(c *gin.Context) {
	userID, err := auth.GetSubjectFromContext(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}

	var req CreatePeerNegotiationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	id, err := h.service.CreateForLocalBuyer(c.Request.Context(), userID, service.LocalCreateRequest{
		SellerID:        req.SellerID,
		Ticker:          req.Ticker,
		Amount:          req.Amount,
		PricePerStock:   req.PricePerStock,
		PriceCurrency:   req.PriceCurrency,
		Premium:         req.Premium,
		PremiumCurrency: req.PremiumCurrency,
		SettlementDate:  req.SettlementDate,
	})
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, id)
}

// SendCounterOffer godoc
// @Summary Post a counter-offer on a cross-bank negotiation
// @Description Forwards a counter-offer to the authoritative bank via §3.3.
// @Tags peer-otc
// @Accept json
// @Produce json
// @Param rn path int true "Authoritative bank routing number"
// @Param id path string true "Negotiation id"
// @Param request body CounterPeerNegotiationRequest true "Counter terms"
// @Success 204
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Failure 502 {object} errors.AppError
// @Security BearerAuth
// @Router /api/peer-otc/negotiations/{rn}/{id}/counter [put]
func (h *PeerOtcFrontendHandler) SendCounterOffer(c *gin.Context) {
	userID, err := auth.GetSubjectFromContext(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}

	negotiationID, ok := parseNegotiationID(c)
	if !ok {
		return
	}

	var req CounterPeerNegotiationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	if err := h.service.SendCounterOfferAsLocal(c.Request.Context(), userID, negotiationID, service.LocalCounterRequest{
		Amount:          req.Amount,
		PricePerStock:   req.PricePerStock,
		PriceCurrency:   req.PriceCurrency,
		Premium:         req.Premium,
		PremiumCurrency: req.PremiumCurrency,
		SettlementDate:  req.SettlementDate,
	}); err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusNoContent)
}

// Withdraw godoc
// @Summary Withdraw from a cross-bank negotiation
// @Description Closes the negotiation locally and notifies the peer via §3.5.
// @Tags peer-otc
// @Produce json
// @Param rn path int true "Authoritative bank routing number"
// @Param id path string true "Negotiation id"
// @Success 204
// @Failure 401 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Failure 502 {object} errors.AppError
// @Security BearerAuth
// @Router /api/peer-otc/negotiations/{rn}/{id} [delete]
func (h *PeerOtcFrontendHandler) Withdraw(c *gin.Context) {
	userID, err := auth.GetSubjectFromContext(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}

	negotiationID, ok := parseNegotiationID(c)
	if !ok {
		return
	}

	if err := h.service.WithdrawAsLocal(c.Request.Context(), userID, negotiationID); err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusNoContent)
}

// parseNegotiationID extracts {rn, id} from the path into a ForeignBankId.
// Writes a 400 to the context and returns (_, false) on parse failure.
func parseNegotiationID(c *gin.Context) (dto.ForeignBankId, bool) {
	rnRaw := c.Param("rn")
	rn, err := strconv.Atoi(rnRaw)
	if err != nil {
		_ = c.Error(errors.BadRequestErr("rn must be an integer"))
		return dto.ForeignBankId{}, false
	}

	id := c.Param("id")
	if id == "" {
		_ = c.Error(errors.BadRequestErr("id is required"))
		return dto.ForeignBankId{}, false
	}

	return dto.ForeignBankId{RoutingNumber: rn, ID: id}, true
}
