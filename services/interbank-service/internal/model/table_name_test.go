package model

import "testing"

func TestTableNames(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		(InboundMessage{}).TableName():      "interbank_inbound_messages",
		(OutboundMessage{}).TableName():     "interbank_outbound_messages",
		(PeerContract{}).TableName():        "interbank_peer_contracts",
		(PeerNegotiation{}).TableName():     "interbank_peer_negotiations",
		(PreparedTransaction{}).TableName(): "interbank_prepared_transactions",
	}

	for got, want := range tests {
		if got != want {
			t.Fatalf("table name = %q, want %q", got, want)
		}
	}
}
