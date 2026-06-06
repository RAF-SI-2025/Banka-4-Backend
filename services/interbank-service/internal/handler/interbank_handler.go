package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/middleware"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/service"
)

// InterbankHandler exposes the single /interbank endpoint that receives all
// bank-to-bank messages.
type InterbankHandler struct {
	processor *service.MessageProcessor
}

func NewInterbankHandler(p *service.MessageProcessor) *InterbankHandler {
	return &InterbankHandler{processor: p}
}

// Receive godoc
// @Summary Receive an inter-bank message
// @Description Single ingress endpoint for the bank-to-bank protocol (§2.11).
// @Description Accepts NEW_TX, COMMIT_TX and ROLLBACK_TX messages. The
// @Description X-Api-Key header authenticates the sending bank.
// @Tags interbank
// @Accept json
// @Produce json
// @Param X-Api-Key header string true "Peer bank API key issued to the caller"
// @Param request body dto.MessageEnvelope true "Message envelope (messageType + body)"
// @Success 200 {object} dto.TransactionVote "NEW_TX accepted with a vote"
// @Success 202 "Accepted, sender must retransmit later"
// @Success 204 "Accepted, no response body"
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 500 {object} errors.AppError
// @Router /interbank [post]
func (h *InterbankHandler) Receive(c *gin.Context) {
	peerRoutingRaw, ok := c.Get(middleware.PeerContextKey)
	if !ok {
		_ = c.Error(errors.UnauthorizedErr("peer routing number missing from context"))
		return
	}
	peerRouting, ok := peerRoutingRaw.(int)
	if !ok {
		_ = c.Error(errors.InternalErr(nil))
		return
	}

	// First pass: sniff messageType. Gin caches the body so the second
	// ShouldBindBodyWithJSON below decodes the same bytes.
	var envelope dto.MessageEnvelope
	if err := c.ShouldBindBodyWithJSON(&envelope); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	// The sender's idempotence key MUST be tagged with their own routing
	// number — §2.2. Reject mismatches so peers can't impersonate others
	// by forging the key.
	if envelope.IdempotenceKey.RoutingNumber != peerRouting {
		_ = c.Error(errors.UnauthorizedErr("idempotenceKey.routingNumber does not match X-Api-Key sender"))
		return
	}

	ctx := c.Request.Context()

	switch envelope.MessageType {
	case dto.MessageTypeNewTx:
		var msg dto.NewTxMessage
		if err := c.ShouldBindBodyWithJSON(&msg); err != nil {
			_ = c.Error(errors.BadRequestErr(err.Error()))
			return
		}

		status, body, err := h.processor.ProcessNewTx(ctx, peerRouting, envelope.IdempotenceKey, &msg.Message)
		h.writeResult(c, status, body, err)

	case dto.MessageTypeCommitTx:
		var msg dto.CommitTxMessage
		if err := c.ShouldBindBodyWithJSON(&msg); err != nil {
			_ = c.Error(errors.BadRequestErr(err.Error()))
			return
		}

		status, body, err := h.processor.ProcessCommitTx(ctx, peerRouting, envelope.IdempotenceKey, &msg.Message)
		h.writeResult(c, status, body, err)

	case dto.MessageTypeRollbackTx:
		var msg dto.RollbackTxMessage
		if err := c.ShouldBindBodyWithJSON(&msg); err != nil {
			_ = c.Error(errors.BadRequestErr(err.Error()))
			return
		}

		status, body, err := h.processor.ProcessRollbackTx(ctx, peerRouting, envelope.IdempotenceKey, &msg.Message)
		h.writeResult(c, status, body, err)
	}
}

func (h *InterbankHandler) writeResult(c *gin.Context, status int, body any, err error) {
	if err != nil {
		_ = c.Error(errors.InternalErr(err))
		return
	}

	switch status {
	case http.StatusNoContent, http.StatusAccepted:
		c.Status(status)
	case http.StatusOK:
		c.JSON(status, body)
	default:
		_ = c.Error(errors.InternalErr(nil))
	}
}
