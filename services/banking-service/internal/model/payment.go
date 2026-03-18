package model

type Payment struct {
	PaymentID        uint   `gorm:"primaryKey"`
	TransactionID    uint   `gorm:"not null"` 
	RecipientName    string
	ReferenceNumber  string
	PaymentCode      string
	Purpose          string

	Transaction      Transaction
}

