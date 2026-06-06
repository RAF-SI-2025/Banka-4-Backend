package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/dto"
)

// peerHTTPTimeout caps each outbound call. The §3 endpoints are not §2
// transactions — they're synchronous request/response — so a single short
// timeout is enough; retries are the caller's concern.
const peerHTTPTimeout = 10 * time.Second

// PeerOtcClient is the outbound HTTP client our service uses to call other
// banks' /interbank/* endpoints. The PeerResolver provides each peer's
// base URL and the API key the peer issued to us, which we send back as
// X-Api-Key on every request.
//
// Each method targets one §3 endpoint and uses the same DTO shapes as the
// inbound handler — the protocol is symmetric.
type PeerOtcClient struct {
	httpClient *http.Client
	peers      *PeerResolver
}

func NewPeerOtcClient(peers *PeerResolver) *PeerOtcClient {
	return &PeerOtcClient{
		httpClient: &http.Client{Timeout: peerHTTPTimeout},
		peers:      peers,
	}
}

// CreateNegotiation calls §3.2 POST /interbank/negotiations on the seller's
// bank, returning the negotiation id assigned by that bank.
func (c *PeerOtcClient) CreateNegotiation(ctx context.Context, offer dto.OtcOffer) (*dto.ForeignBankId, error) {
	var result dto.ForeignBankId
	if err := c.do(ctx, offer.SellerID.RoutingNumber, http.MethodPost, "/interbank/negotiations", offer, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateCounter calls §3.3 PUT /interbank/negotiations/{rn}/{id}. The path
// {rn} is always the negotiation's authoritative (seller's) routing number,
// while targetRouting is the OPPOSING party's bank the notification is sent
// to (the buyer's bank when the seller counters, or vice versa).
func (c *PeerOtcClient) UpdateCounter(ctx context.Context, targetRouting int, negotiationID dto.ForeignBankId, offer dto.OtcOffer) error {
	path := fmt.Sprintf("/interbank/negotiations/%d/%s", negotiationID.RoutingNumber, negotiationID.ID)
	return c.do(ctx, targetRouting, http.MethodPut, path, offer, nil)
}

// GetNegotiation calls §3.4 GET /interbank/negotiations/{rn}/{id} on the
// authoritative bank.
func (c *PeerOtcClient) GetNegotiation(ctx context.Context, negotiationID dto.ForeignBankId) (*dto.OtcNegotiation, error) {
	path := fmt.Sprintf("/interbank/negotiations/%d/%s", negotiationID.RoutingNumber, negotiationID.ID)

	var result dto.OtcNegotiation
	if err := c.do(ctx, negotiationID.RoutingNumber, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Close calls §3.5 DELETE /interbank/negotiations/{rn}/{id}. As with
// UpdateCounter, the path {rn} is the authoritative routing number while
// targetRouting is the opposing party's bank the notification is sent to.
func (c *PeerOtcClient) Close(ctx context.Context, targetRouting int, negotiationID dto.ForeignBankId) error {
	path := fmt.Sprintf("/interbank/negotiations/%d/%s", negotiationID.RoutingNumber, negotiationID.ID)
	return c.do(ctx, targetRouting, http.MethodDelete, path, nil, nil)
}

// Accept calls §3.6 GET /interbank/negotiations/{rn}/{id}/accept on the
// authoritative bank. The peer is expected to drive the resulting §2
// NEW_TX flow before returning a success status; consumers should treat
// the timeout accordingly.
func (c *PeerOtcClient) Accept(ctx context.Context, negotiationID dto.ForeignBankId) (*dto.PeerContract, error) {
	path := fmt.Sprintf("/interbank/negotiations/%d/%s/accept", negotiationID.RoutingNumber, negotiationID.ID)

	var result dto.PeerContract
	if err := c.do(ctx, negotiationID.RoutingNumber, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *PeerOtcClient) SendNewTx(ctx context.Context, peerRouting int, key string, tx dto.Transaction) (*dto.TransactionVote, error) {
	msg := dto.NewTxMessage{
		IdempotenceKey: dto.IdempotenceKey{
			RoutingNumber:       c.peers.OurRoutingNumber(),
			LocallyGeneratedKey: key,
		},
		MessageType: dto.MessageTypeNewTx,
		Message:     tx,
	}

	var vote dto.TransactionVote
	if err := c.do(ctx, peerRouting, http.MethodPost, "/interbank", msg, &vote); err != nil {
		return nil, err
	}
	return &vote, nil
}

func (c *PeerOtcClient) SendCommitTx(ctx context.Context, peerRouting int, key string, txID dto.ForeignBankId) error {
	msg := dto.CommitTxMessage{
		IdempotenceKey: dto.IdempotenceKey{
			RoutingNumber:       c.peers.OurRoutingNumber(),
			LocallyGeneratedKey: key,
		},
		MessageType: dto.MessageTypeCommitTx,
		Message:     dto.CommitTransaction{TransactionID: txID},
	}
	return c.do(ctx, peerRouting, http.MethodPost, "/interbank", msg, nil)
}

func (c *PeerOtcClient) SendRollbackTx(ctx context.Context, peerRouting int, key string, txID dto.ForeignBankId) error {
	msg := dto.RollbackTxMessage{
		IdempotenceKey: dto.IdempotenceKey{
			RoutingNumber:       c.peers.OurRoutingNumber(),
			LocallyGeneratedKey: key,
		},
		MessageType: dto.MessageTypeRollbackTx,
		Message:     dto.RollbackTransaction{TransactionID: txID},
	}
	return c.do(ctx, peerRouting, http.MethodPost, "/interbank", msg, nil)
}

func (c *PeerOtcClient) PublicStock(ctx context.Context, peerRouting int) ([]dto.PublicStock, error) {
	var result []dto.PublicStock
	if err := c.do(ctx, peerRouting, http.MethodGet, "/interbank/public-stock", nil, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// UserLookup calls §3.7 GET /interbank/user/{rn}/{id} on the bank that
// owns the user.
func (c *PeerOtcClient) UserLookup(ctx context.Context, userID dto.ForeignBankId) (*dto.UserInformation, error) {
	path := fmt.Sprintf("/interbank/user/%d/%s", userID.RoutingNumber, userID.ID)

	var result dto.UserInformation
	if err := c.do(ctx, userID.RoutingNumber, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// do is the shared HTTP machinery: peer lookup, request build, X-Api-Key
// header, JSON encoding, status-code mapping. body may be nil for GET /
// DELETE; out may be nil when the call returns no body (e.g. 204).
func (c *PeerOtcClient) do(ctx context.Context, peerRouting int, method, path string, body any, out any) error {
	peer, ok := c.peers.ByRoutingNumber(peerRouting)
	if !ok {
		return errors.NewAppError(
			http.StatusBadGateway,
			fmt.Sprintf("unknown peer routing number %d", peerRouting),
			nil,
		)
	}

	var bodyReader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return errors.InternalErr(err)
		}

		bodyReader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, peer.BaseURL+path, bodyReader)
	if err != nil {
		return errors.InternalErr(err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	req.Header.Set("X-Api-Key", peer.OurAPIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return errors.NewAppError(
			http.StatusBadGateway,
			fmt.Sprintf("peer %d unreachable: %s", peerRouting, err.Error()),
			err,
		)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return errors.NewAppError(
			resp.StatusCode,
			fmt.Sprintf("peer %d returned %d: %s", peerRouting, resp.StatusCode, string(respBody)),
			nil,
		)
	}

	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return errors.NewAppError(
			http.StatusBadGateway,
			fmt.Sprintf("peer %d returned malformed JSON: %s", peerRouting, err.Error()),
			err,
		)
	}

	return nil
}
