package validator

import (
	"sync"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
)

// Gin exposes a shared validator engine, so custom rules must only be
// registered once. Integration tests build multiple routers in parallel,
// and repeated registration would race on that global validator state.
var registerOnce sync.Once

func RegisterValidators() {
	registerOnce.Do(func() {
		if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
			_ = v.RegisterValidation("account_type", validateAccountType)
			_ = v.RegisterValidation("account_kind", validateAccountKind)
			_ = v.RegisterValidation("currency_code", validateForeignCurrency)
			v.RegisterStructValidation(validateCurrentAccountStruct, dto.CreateAccountRequest{})
		}
	})
}
