package dto

type CreatePaymentResponse struct {
	PaymentID uint   `json:"id"`
}

type VerifyPaymentResponse struct {
	PaymentID uint   `json:"id"`
}
