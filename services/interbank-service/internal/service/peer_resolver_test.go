package service

import (
	"testing"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/config"
)

func TestPeerResolverDelegatesToRegistry(t *testing.T) {
	t.Parallel()

	peer := config.Peer{
		RoutingNumber: 555,
		BaseURL:       "https://peer.example.com",
		OurAPIKey:     "ours",
		TheirAPIKey:   "theirs",
		DisplayName:   "Peer Bank",
	}
	resolver := NewPeerResolver(
		config.NewPeerRegistry([]config.Peer{peer}),
		&config.Configuration{OurRoutingNumber: 444, OurBankDisplayName: "Banka 4"},
	)

	if got := resolver.OurRoutingNumber(); got != 444 {
		t.Fatalf("our routing number = %d, want 444", got)
	}
	if got := resolver.OurBankDisplayName(); got != "Banka 4" {
		t.Fatalf("our display name = %q, want Banka 4", got)
	}

	byRouting, ok := resolver.ByRoutingNumber(555)
	if !ok || byRouting.BaseURL != peer.BaseURL {
		t.Fatalf("lookup by routing = %#v, %v", byRouting, ok)
	}

	byKey, ok := resolver.ByTheirAPIKey("theirs")
	if !ok || byKey.RoutingNumber != peer.RoutingNumber {
		t.Fatalf("lookup by key = %#v, %v", byKey, ok)
	}

	all := resolver.All()
	if len(all) != 1 || all[0].DisplayName != peer.DisplayName {
		t.Fatalf("all peers = %#v", all)
	}
}
