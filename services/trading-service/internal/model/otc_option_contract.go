package model

import "time"

// OtcOptionContract predstavlja sklopljeni opcioni ugovor (CALL opciju)
// koji nastaje AUTOMATSKI kada se OtcOffer prihvati.
//
// Spec ("Postignut dogovor"): "Kada se postigne dogovor, sistem automatski
// kreira opcioni ugovor i premija se isplaćuje sa računa kupca na račun
// prodavca. Kupoprodaja se kasnije izvršava po odluci kupca."
//
// Ovo je odvojen entitet od `Option` (koji je za exchange-listed opcije i
// koristi se za Black-Scholes simulaciju). OTC opcije imaju konkretnog
// kupca i prodavca, fiksnu količinu i cenu, i platenu premiju.
type OtcOptionContract struct {
	OtcOptionContractID uint `gorm:"primaryKey;autoIncrement"`

	// Backreference na ponudu iz koje je nastao ugovor.
	OtcOfferID uint `gorm:"not null;uniqueIndex"`

	// Strane — preuzete iz OtcOffer-a, ne menjaju se.
	BuyerID  uint `gorm:"not null;index"`
	SellerID uint `gorm:"not null;index"`

	// Predmet ugovora.
	StockAssetID uint  `gorm:"not null;index"`
	Stock        Stock `gorm:"foreignKey:StockAssetID;references:AssetID"`

	// Parametri ugovora — fiksirani u trenutku prihvatanja.
	Amount         int       `gorm:"not null"`
	StrikePrice    float64   `gorm:"not null"` // = OtcOffer.PricePerStock
	Premium        float64   `gorm:"not null"` // već isplaćeno
	SettlementDate time.Time `gorm:"not null"`

	// Status izvršenja opcije. Spec scenariji:
	//   - cena poraste -> kupac iskorišćava (IsExercised=true, kupoprodaja po SAGA)
	//   - cena padne   -> kupac ne iskorišćava, opcija ekspirira (gubi premiju)
	IsExercised bool `gorm:"not null;default:false"`
	ExercisedAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}
