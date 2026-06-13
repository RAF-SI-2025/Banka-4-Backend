package service

import "context"

type TaxRecorder interface {
	RecordTax(ctx context.Context, accountNumber string, employeeID *uint, profit float64, currencyCode string) error

	// ReduceTax lowers the accumulated capital-gains tax for an account by 15% of
	// a realized loss base (e.g. a lost OTC premium), clamped at zero. Used when an
	// OTC option expires unexercised: the buyer's lost premium offsets that period's
	// capital gains.
	ReduceTax(ctx context.Context, accountNumber string, employeeID *uint, lossBase float64) error
}
