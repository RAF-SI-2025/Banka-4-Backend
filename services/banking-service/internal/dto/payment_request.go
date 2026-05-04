package dto

type CreatePaymentRequest struct {
	RecipientName          string  `json:"recipient_name" binding:"required"`
	RecipientAccountNumber string  `json:"recipient_account_number" binding:"required"`
	Amount                 float64 `json:"amount" binding:"required,gt=0"`
	ReferenceNumber        string  `json:"reference_number"`
	PaymentCode            string  `json:"payment_code"`
	Purpose                string  `json:"purpose"`
	PayerAccountNumber     string  `json:"payer_account_number" binding:"required"`
	CommissionExempt       bool    `json:"commission_exempt"`
}

type VerifyPaymentRequest struct {
	Code string `json:"code" binding:"required"`
}
