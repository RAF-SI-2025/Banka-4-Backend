package validator

import (
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestFuturesTickerValidation(t *testing.T) {
	t.Parallel()

	v := validator.New()
	if err := v.RegisterValidation("futures_ticker", validateFuturesTicker); err != nil {
		t.Fatalf("register futures_ticker: %v", err)
	}

	var payload struct {
		Ticker string `validate:"futures_ticker"`
	}

	for _, ticker := range []string{"CLJ26", "ZCZ27", "MCLM26"} {
		payload.Ticker = ticker
		if err := v.Struct(payload); err != nil {
			t.Fatalf("expected %s to be valid, got %v", ticker, err)
		}
	}

	for _, ticker := range []string{"clj26", "TOOLONGJ26", "CLY26", "CLJ2026", "CL26"} {
		payload.Ticker = ticker
		if err := v.Struct(payload); err == nil {
			t.Fatalf("expected %s to be invalid", ticker)
		}
	}
}
