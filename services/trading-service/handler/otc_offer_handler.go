package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/service"
)

// OtcOfferHandler handles HTTP requests for OTC offer negotiation and option contracts.
type OtcOfferHandler struct {
	service *service.OtcOfferService
}

// NewOtcOfferHandler creates a new OtcOfferHandler with the provided service.
func NewOtcOfferHandler(svc *service.OtcOfferService) *OtcOfferHandler {
	return &OtcOfferHandler{service: svc}
}

// CreateOffer initiates a new OTC negotiation as the authenticated buyer.
//
// @Summary     Create OTC offer
// @Description Buyer initiates a new OTC negotiation with a seller for publicly listed shares.
// @Tags        otc
// @Accept      json
// @Produce     json
// @Param       request body     dto.CreateOtcOfferRequest true "Offer details"
// @Success     201     {object} dto.OtcOfferResponse
// @Failure     400     {object} errors.AppError
// @Failure     401     {object} errors.AppError
// @Failure     403     {object} errors.AppError
// @Router      /api/otc/offers [post]
func (h *OtcOfferHandler) CreateOffer(c *gin.Context) {
	var req dto.CreateOtcOfferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}
	offer, err := h.service.CreateOffer(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, dto.ToOtcOfferResponse(*offer))
}

// SendCounterOffer updates the negotiation parameters on behalf of either party.
//
// @Summary     Send counter-offer
// @Description Either party may update the offer parameters (amount, price in RSD, premium in RSD, settlement date).
//
//	Parties alternate turns — the same user cannot send two consecutive counter-offers.
//
// @Tags        otc
// @Accept      json
// @Produce     json
// @Param       id      path     int                      true "Offer ID"
// @Param       request body     dto.CounterOfferRequest  true "Updated offer parameters"
// @Success     200     {object} dto.OtcOfferResponse
// @Failure     400     {object} errors.AppError
// @Failure     401     {object} errors.AppError
// @Failure     403     {object} errors.AppError
// @Failure     404     {object} errors.AppError
// @Router      /api/otc/offers/{id}/counter [put]
func (h *OtcOfferHandler) SendCounterOffer(c *gin.Context) {
	id, err := parseOfferID(c)
	if err != nil {
		_ = c.Error(err)
		return
	}
	var req dto.CounterOfferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}
	offer, err := h.service.SendCounterOffer(c.Request.Context(), id, req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.ToOtcOfferResponse(*offer))
}

// AcceptOffer accepts the current offer, creating an option contract and transferring the premium in RSD.
//
// @Summary     Accept OTC offer
// @Description The party opposite to ModifiedBy accepts the offer. An option contract is created
//
//	and the premium in RSD is immediately transferred from the buyer's account to the seller's.
//	If the seller has not yet provided their account number, it must be supplied here.
//
// @Tags        otc
// @Accept      json
// @Produce     json
// @Param       id      path     int                    true  "Offer ID"
// @Param       request body     dto.AcceptOfferRequest false "Seller account number (required on seller's first participation)"
// @Success     201     {object} dto.OtcOptionContractResponse
// @Failure     400     {object} errors.AppError
// @Failure     401     {object} errors.AppError
// @Failure     403     {object} errors.AppError
// @Failure     404     {object} errors.AppError
// @Router      /api/otc/offers/{id}/accept [patch]
func (h *OtcOfferHandler) AcceptOffer(c *gin.Context) {
	id, err := parseOfferID(c)
	if err != nil {
		_ = c.Error(err)
		return
	}
	var req dto.AcceptOfferRequest
	_ = c.ShouldBindJSON(&req) // body is optional

	contract, err := h.service.AcceptOffer(c.Request.Context(), id, req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, dto.ToOtcOptionContractResponse(*contract))
}

// RejectOffer allows either party to withdraw from the negotiation at any time.
//
// @Summary     Reject OTC offer
// @Description Either party may reject the offer, terminating the negotiation.
// @Tags        otc
// @Accept      json
// @Produce     json
// @Param       id      path     int                    true  "Offer ID"
// @Param       request body     dto.RejectOfferRequest false "Optional rejection comment"
// @Success     200     {object} dto.OtcOfferResponse
// @Failure     400     {object} errors.AppError
// @Failure     401     {object} errors.AppError
// @Failure     403     {object} errors.AppError
// @Failure     404     {object} errors.AppError
// @Router      /api/otc/offers/{id}/reject [patch]
func (h *OtcOfferHandler) RejectOffer(c *gin.Context) {
	id, err := parseOfferID(c)
	if err != nil {
		_ = c.Error(err)
		return
	}
	var req dto.RejectOfferRequest
	_ = c.ShouldBindJSON(&req) // body is optional

	offer, err := h.service.RejectOffer(c.Request.Context(), id, req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.ToOtcOfferResponse(*offer))
}

// GetMyActiveOffers returns all active negotiations in which the authenticated user participates.
//
// @Summary     List active OTC offers
// @Description Returns all ongoing negotiations (status=ACTIVE) where the authenticated user
//
//	is either the buyer or the seller.
//
// @Tags        otc
// @Produce     json
// @Success     200 {array}  dto.OtcOfferResponse
// @Failure     401 {object} errors.AppError
// @Router      /api/otc/offers/active [get]
func (h *OtcOfferHandler) GetMyActiveOffers(c *gin.Context) {
	userID, err := auth.GetSubjectFromContext(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}
	offers, err := h.service.GetActiveOffersForUser(c.Request.Context(), userID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, offers)
}

// GetMyOptionContracts returns all option contracts in which the authenticated user participates.
//
// @Summary     List OTC option contracts
// @Description Returns all option contracts (CALL) created from accepted OTC offers where the
//
//	authenticated user is either the buyer or the seller.
//
// @Tags        otc
// @Produce     json
// @Success     200 {array}  dto.OtcOptionContractResponse
// @Failure     401 {object} errors.AppError
// @Router      /api/otc/contracts [get]
func (h *OtcOfferHandler) GetMyOptionContracts(c *gin.Context) {
	userID, err := auth.GetSubjectFromContext(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}
	resp, err := h.service.GetOptionContractsForUser(c.Request.Context(), userID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ExerciseContract starts or resumes settlement of an active OTC option contract.
//
// @Summary     Exercise OTC contract
// @Description The buyer who holds the OTC option contract starts or resumes the same-bank settlement saga.
// @Tags        otc
// @Produce     json
// @Param       id path int true "OTC contract ID"
// @Success     200 {object} dto.OtcExecutionSagaResponse
// @Failure     400 {object} errors.AppError
// @Failure     401 {object} errors.AppError
// @Failure     403 {object} errors.AppError
// @Failure     404 {object} errors.AppError
// @Failure     409 {object} errors.AppError
// @Router      /api/otc/contracts/{id}/exercise [post]
func (h *OtcOfferHandler) ExerciseContract(c *gin.Context) {
	id, err := parseContractID(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	execution, err := h.service.ExerciseContract(c.Request.Context(), id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	resp := dto.ToOtcExecutionSagaResponse(*execution)
	if entries, logErr := h.service.GetExecutionLog(c.Request.Context(), execution.OtcExecutionSagaID); logErr == nil {
		resp.Log = dto.ToOtcExecutionLogEntryResponses(entries)
	}

	c.JSON(http.StatusOK, resp)
}

// GetExecution returns the current state and per-step attempt log of an OTC
// execution saga.
//
// @Summary     Get OTC execution saga
// @Description Returns the settlement saga state and its step-by-step log. Only the contract's buyer or seller may read it.
// @Tags        otc
// @Produce     json
// @Param       id path int true "OTC execution saga ID"
// @Success     200 {object} dto.OtcExecutionSagaResponse
// @Failure     400 {object} errors.AppError
// @Failure     401 {object} errors.AppError
// @Failure     403 {object} errors.AppError
// @Failure     404 {object} errors.AppError
// @Router      /api/otc/executions/{id} [get]
func (h *OtcOfferHandler) GetExecution(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		_ = c.Error(errors.BadRequestErr("invalid execution id"))
		return
	}

	execution, entries, err := h.service.GetExecution(c.Request.Context(), uint(id))
	if err != nil {
		_ = c.Error(err)
		return
	}

	resp := dto.ToOtcExecutionSagaResponse(*execution)
	resp.Log = dto.ToOtcExecutionLogEntryResponses(entries)
	c.JSON(http.StatusOK, resp)
}

// parseOfferID extracts and validates the :id path parameter as a uint.
func parseOfferID(c *gin.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return 0, errors.BadRequestErr("invalid offer id")
	}
	return uint(id), nil
}

func parseContractID(c *gin.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return 0, errors.BadRequestErr("invalid contract id")
	}
	return uint(id), nil
}
