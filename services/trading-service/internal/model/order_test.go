package model

import "testing"

func TestOrderRemainingPortions(t *testing.T) {
	t.Parallel()

	order := &Order{Quantity: 100, FilledQty: 35}

	if got := order.RemainingPortions(); got != 65 {
		t.Fatalf("remaining portions = %d, want 65", got)
	}
}
