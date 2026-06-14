package validator

import (
	"testing"
	"time"

	"github.com/go-playground/validator/v10"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
)

func TestAccountValidators(t *testing.T) {
	t.Parallel()

	v := validator.New()
	if err := v.RegisterValidation("account_type", validateAccountType); err != nil {
		t.Fatalf("register account_type: %v", err)
	}
	if err := v.RegisterValidation("account_kind", validateAccountKind); err != nil {
		t.Fatalf("register account_kind: %v", err)
	}
	if err := v.RegisterValidation("currency_code", validateForeignCurrency); err != nil {
		t.Fatalf("register currency_code: %v", err)
	}

	payload := struct {
		AccountType  model.AccountType  `validate:"account_type"`
		AccountKind  model.AccountKind  `validate:"account_kind"`
		CurrencyCode model.CurrencyCode `validate:"currency_code"`
	}{
		AccountType:  model.AccountTypePersonal,
		AccountKind:  model.AccountKindForeign,
		CurrencyCode: model.EUR,
	}
	if err := v.Struct(payload); err != nil {
		t.Fatalf("expected valid account payload, got %v", err)
	}

	payload.AccountType = model.AccountTypeBank
	if err := v.Struct(payload); err == nil {
		t.Fatal("expected account type validation error")
	}

	payload.AccountType = model.AccountTypeBusiness
	payload.AccountKind = model.AccountKindInternal
	if err := v.Struct(payload); err == nil {
		t.Fatal("expected account kind validation error")
	}

	payload.AccountKind = model.AccountKindForeign
	payload.CurrencyCode = model.CurrencyCode("BTC")
	if err := v.Struct(payload); err == nil {
		t.Fatal("expected currency validation error")
	}
}

func TestCurrentAccountSubtypeValidation(t *testing.T) {
	t.Parallel()

	v := validator.New()
	v.RegisterStructValidation(validateCurrentAccountStruct, dto.CreateAccountRequest{})

	valid := dto.CreateAccountRequest{
		Name:         "Checking",
		ClientID:     1,
		EmployeeID:   2,
		AccountType:  model.AccountTypePersonal,
		AccountKind:  model.AccountKindCurrent,
		Subtype:      model.SubtypeSavings,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		CurrencyCode: model.EUR,
	}
	if err := v.Struct(valid); err != nil {
		t.Fatalf("expected personal subtype to pass, got %v", err)
	}

	valid.AccountType = model.AccountTypeBusiness
	valid.Subtype = model.SubtypeLLC
	if err := v.Struct(valid); err != nil {
		t.Fatalf("expected business subtype to pass, got %v", err)
	}

	valid.Subtype = model.SubtypeYouth
	if err := v.Struct(valid); err == nil {
		t.Fatal("expected invalid business subtype error")
	}

	valid.AccountKind = model.AccountKindForeign
	if err := v.Struct(valid); err != nil {
		t.Fatalf("foreign account should skip current subtype validation, got %v", err)
	}
}
