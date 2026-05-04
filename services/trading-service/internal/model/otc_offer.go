package model

import "time"

// OtcOfferStatus označava trenutno stanje pregovora.
type OtcOfferStatus string

const (
	// OtcOfferStatusActive — pregovor traje (back-and-forth).
	OtcOfferStatusActive OtcOfferStatus = "ACTIVE"
	// OtcOfferStatusAccepted — druga strana je prihvatila; kreiran je opcioni ugovor.
	OtcOfferStatusAccepted OtcOfferStatus = "ACCEPTED"
	// OtcOfferStatusRejected — jedna strana je odustala.
	OtcOfferStatusRejected OtcOfferStatus = "REJECTED"
)

// OtcOffer predstavlja AKTIVNU OTC ponudu — entitet "Aktivna ponuda" iz Celine 4.
//
// Bitno: ovo NIJE opcioni ugovor. Ovo je pregovor. Kada druga strana prihvati
// (AcceptOffer), kreira se OtcOptionContract i ova ponuda dobija status ACCEPTED.
//
// Polja Amount, PricePerStock, Premium i SettlementDate menjaju se po
// kontraponudi — entitet OSTAJE ISTI, samo se ažurira (uz LastModified i
// ModifiedBy). Strane (Buyer/Seller) se NIKAD ne menjaju u toku pregovora.
type OtcOffer struct {
	OtcOfferID uint `gorm:"primaryKey;autoIncrement"`

	// Strane — postavljene pri kreiranju, nepromenljive tokom pregovora.
	BuyerID  uint `gorm:"not null;index"`
	SellerID uint `gorm:"not null;index"`

	// Stock (akcije) — spec: trguju se SAMO akcije.
	StockAssetID uint  `gorm:"not null"`
	Stock        Stock `gorm:"foreignKey:StockAssetID;references:AssetID"`

	// Pregovarani parametri — menjaju se po kontraponudi.
	Amount         int       `gorm:"not null"`
	PricePerStock  float64   `gorm:"not null"`
	Premium        float64   `gorm:"not null"`
	SettlementDate time.Time `gorm:"not null"`

	// Računi za izvršenje premium transfera kada se ponuda prihvati.
	// BuyerAccountNumber se postavlja pri kreiranju (kupac je inicijator).
	// SellerAccountNumber se postavlja pri prvom prodavčevom potezu
	// (counter offer ili accept).
	BuyerAccountNumber  string  `gorm:"not null;size:64"`
	SellerAccountNumber *string `gorm:"size:64"`

	// Status pregovora.
	Status OtcOfferStatus `gorm:"not null;size:16;default:'ACTIVE'"`

	// Tracking poslednje izmene — spec: LastModified, ModifiedBy.
	LastModified time.Time `gorm:"not null"`
	ModifiedBy   uint      `gorm:"not null;index"`

	// Link na kreirani opcioni ugovor (popunjava se pri Accept).
	// Držimo samo ID — bez relacije ka OtcOptionContract da se izbegne cirkularna
	// FK između tabela (OtcOptionContract već ima OtcOfferID koji upućuje nazad).
	// Aplikacija po potrebi učita ugovor preko OtcOptionContractRepository.FindByID.
	OptionContractID *uint

	CreatedAt time.Time
	UpdatedAt time.Time
}
