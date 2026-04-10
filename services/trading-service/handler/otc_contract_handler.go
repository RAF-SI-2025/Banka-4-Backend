package handler

import (
	"net/http"
	"strconv"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/service"
	"github.com/gin-gonic/gin"
)

type OtcContractHandler struct {
	service *service.OtcContractService
}

func NewOtcContractHandler(svc *service.OtcContractService) *OtcContractHandler {
	return &OtcContractHandler{service: svc}
}

// CreateOffer — klijent kreira ponudu
func (h *OtcContractHandler) CreateOffer(c *gin.Context) {
	var req dto.CreateOtcContractRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.BadRequestErr(err.Error()))
		return
	}
	contract, err := h.service.CreateContract(c.Request.Context(), req)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, dto.ToOtcContractResponse(*contract))
}

// GetMyOffers — vrati sve ponude ulogovanog korisnika (kao kupac i prodavac)
func (h *OtcContractHandler) GetMyOffers(c *gin.Context) {
	userID, err := auth.GetSubjectFromContext(c.Request.Context())
	if err != nil {
		c.Error(err)
		return
	}

	bought, err := h.service.GetContractsForBuyer(c.Request.Context(), userID)
	if err != nil {
		c.Error(err)
		return
	}
	sold, err := h.service.GetContractsForSeller(c.Request.Context(), userID)
	if err != nil {
		c.Error(err)
		return
	}

	all := append(bought, sold...)
	c.JSON(http.StatusOK, dto.ToOtcContractResponseList(all))
}

// AcceptOffer — prodavac prihvata
func (h *OtcContractHandler) AcceptOffer(c *gin.Context) {
	id, err := parseContractID(c)
	if err != nil {
		c.Error(err)
		return
	}

	sellerID, err := auth.GetSubjectFromContext(c.Request.Context())
	if err != nil {
		c.Error(err)
		return
	}

	contract, err := h.service.AcceptContract(c.Request.Context(), id, sellerID)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.ToOtcContractResponse(*contract))
}

// RejectOffer — prodavac odbija
func (h *OtcContractHandler) RejectOffer(c *gin.Context) {
	id, err := parseContractID(c)
	if err != nil {
		c.Error(err)
		return
	}

	var req dto.RejectOtcContractRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	sellerID, err := auth.GetSubjectFromContext(c.Request.Context())
	if err != nil {
		c.Error(err)
		return
	}

	contract, err := h.service.RejectContract(c.Request.Context(), id, sellerID, req)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.ToOtcContractResponse(*contract))
}

// CounterOffer — prodavac šalje protivponudu
func (h *OtcContractHandler) CounterOffer(c *gin.Context) {
	id, err := parseContractID(c)
	if err != nil {
		c.Error(err)
		return
	}

	var req dto.CounterOfferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	contract, err := h.service.CreateCounterOffer(c.Request.Context(), id, req)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, dto.ToOtcContractResponse(*contract))
}

// ApproveBankOffer — supervizor banke odobrava (opciono za ovaj sprint)
func (h *OtcContractHandler) ApproveBankOffer(c *gin.Context) {
	id, err := parseContractID(c)
	if err != nil {
		c.Error(err)
		return
	}
	contract, err := h.service.ApproveBankContract(c.Request.Context(), id)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.ToOtcContractResponse(*contract))
}

func parseContractID(c *gin.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return 0, errors.BadRequestErr("invalid contract id")
	}
	return uint(id), nil
}
