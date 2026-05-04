package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/service"
)

type CardHandler struct {
	service *service.CardService
}

func NewCardHandler(service *service.CardService) *CardHandler {
	return &CardHandler{service: service}
}

// RequestCard godoc
// @Summary Request a new card
// @Description Client requests a new card for their account. The request is confirmed later using a confirmation code.
// @Tags cards
// @Accept json
// @Produce json
// @Param request body dto.RequestCardRequest true "Card request payload"
// @Success 200 {object} dto.MessageResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Failure 409 {object} errors.AppError
// @Security BearerAuth
// @Router /api/cards/request [post]
func (h *CardHandler) RequestCard(c *gin.Context) {
	var req dto.RequestCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	input := &service.RequestCardInput{
		AccountNumber: req.AccountNumber,
	}

	if req.AuthorizedPerson != nil {
		input.AuthorizedPerson = &service.AuthorizedPersonInput{
			FirstName:   req.AuthorizedPerson.FirstName,
			LastName:    req.AuthorizedPerson.LastName,
			DateOfBirth: req.AuthorizedPerson.DateOfBirth,
			Gender:      req.AuthorizedPerson.Gender,
			Email:       req.AuthorizedPerson.Email,
			PhoneNumber: req.AuthorizedPerson.PhoneNumber,
			Address:     req.AuthorizedPerson.Address,
		}
	}

	_, err := h.service.RequestCard(c.Request.Context(), input)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{
		Message: "Card request created successfully. Please confirm the code sent to your email.",
	})
}

// ConfirmCardRequest godoc
// @Summary Confirm a card request
// @Description Confirms a pending card request using the confirmation code and creates the card.
// @Tags cards
// @Accept json
// @Produce json
// @Param request body dto.ConfirmCardRequest true "Confirmation payload"
// @Success 201 {object} dto.CardResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Failure 409 {object} errors.AppError
// @Security BearerAuth
// @Router /api/cards/request/confirm [post]
func (h *CardHandler) ConfirmCardRequest(c *gin.Context) {
	var req dto.ConfirmCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	card, err := h.service.ConfirmCardRequest(c.Request.Context(), req.AccountNumber, req.ConfirmationCode)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, dto.ToCardResponse(card, ""))
}

// ListCardsByAccount godoc
// @Summary List cards for an account
// @Description Returns cards for the specified account. Clients can access only their own accounts, while employees can access any account.
// @Tags cards
// @Produce json
// @Param clientId path int true "Client ID"
// @Param accountNumber path string true "Account number"
// @Success 200 {object} dto.AccountCardsResponse
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Security BearerAuth
// @Router /api/clients/{clientId}/accounts/{accountNumber}/cards [get]
func (h *CardHandler) ListCardsByAccount(c *gin.Context) {
	accountNumber := c.Param("accountNumber")

	result, err := h.service.ListCardsForAccount(c.Request.Context(), accountNumber)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, dto.ToAccountCardsResponse(result.Account, result.Cards))
}

// BlockCard godoc
// @Summary Block a card
// @Description Blocks a card. Clients can block only their own cards, while employees can block any card.
// @Tags cards
// @Produce json
// @Param cardId path int true "Card ID"
// @Success 200 {object} dto.CardResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Security BearerAuth
// @Router /api/cards/{cardId}/block [put]
func (h *CardHandler) BlockCard(c *gin.Context) {
	cardID, err := strconv.ParseUint(c.Param("cardId"), 10, 64)
	if err != nil {
		_ = c.Error(errors.BadRequestErr("invalid card id"))
		return
	}

	card, svcErr := h.service.BlockCard(c.Request.Context(), uint(cardID))
	if svcErr != nil {
		_ = c.Error(svcErr)
		return
	}

	c.JSON(http.StatusOK, dto.ToCardResponse(card, ""))
}

// UnblockCard godoc
// @Summary Unblock a card
// @Description Unblocks a blocked card. Only employees can perform this action.
// @Tags cards
// @Produce json
// @Param cardId path int true "Card ID"
// @Success 200 {object} dto.CardResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Security BearerAuth
// @Router /api/cards/{cardId}/unblock [put]
func (h *CardHandler) UnblockCard(c *gin.Context) {
	cardID, err := strconv.ParseUint(c.Param("cardId"), 10, 64)
	if err != nil {
		_ = c.Error(errors.BadRequestErr("invalid card id"))
		return
	}

	card, svcErr := h.service.UnblockCard(c.Request.Context(), uint(cardID))
	if svcErr != nil {
		_ = c.Error(svcErr)
		return
	}

	c.JSON(http.StatusOK, dto.ToCardResponse(card, ""))
}

// DeactivateCard godoc
// @Summary Deactivate a card
// @Description Deactivates a card permanently. Only employees can perform this action.
// @Tags cards
// @Produce json
// @Param cardId path int true "Card ID"
// @Success 200 {object} dto.CardResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Security BearerAuth
// @Router /api/cards/{cardId}/deactivate [put]
func (h *CardHandler) DeactivateCard(c *gin.Context) {
	cardID, err := strconv.ParseUint(c.Param("cardId"), 10, 64)
	if err != nil {
		_ = c.Error(errors.BadRequestErr("invalid card id"))
		return
	}

	card, svcErr := h.service.DeactivateCard(c.Request.Context(), uint(cardID))
	if svcErr != nil {
		_ = c.Error(svcErr)
		return
	}

	c.JSON(http.StatusOK, dto.ToCardResponse(card, ""))
}
