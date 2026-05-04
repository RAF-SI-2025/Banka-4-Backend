package validator

import (
	"sync"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// Gin exposes a shared validator engine, so custom rules must only be
// registered once. Integration tests build multiple routers in parallel,
// and repeated registration would race on that global validator state.
var registerOnce sync.Once

func RegisterValidators() {
	registerOnce.Do(func() {
		if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
			_ = v.RegisterValidation("futures_ticker", validateFuturesTicker)
		}
	})
}
