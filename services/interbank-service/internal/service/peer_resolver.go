package service

import (
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/config"
)

// PeerResolver exposes the peer registry to the rest of the service. It is
// the only consumer of the YAML-loaded config in the service layer, so that
// future swaps (e.g., a DB-backed registry) only touch this file.
type PeerResolver struct {
	registry        *config.PeerRegistry
	ours            int
	bankDisplayName string
}

func NewPeerResolver(reg *config.PeerRegistry, cfg *config.Configuration) *PeerResolver {
	return &PeerResolver{
		registry:        reg,
		ours:            cfg.OurRoutingNumber,
		bankDisplayName: cfg.OurBankDisplayName,
	}
}

func (r *PeerResolver) OurRoutingNumber() int { return r.ours }

// OurBankDisplayName returns this bank's human-readable name. Surfaced in
// §3.7 UserInformation responses so peer banks can render "<displayName> @
// <bankDisplayName>" in their UI.
func (r *PeerResolver) OurBankDisplayName() string { return r.bankDisplayName }

func (r *PeerResolver) ByRoutingNumber(rn int) (config.Peer, bool) {
	return r.registry.ByRoutingNumber(rn)
}

func (r *PeerResolver) ByTheirAPIKey(key string) (config.Peer, bool) {
	return r.registry.ByTheirAPIKey(key)
}

func (r *PeerResolver) All() []config.Peer { return r.registry.All() }
