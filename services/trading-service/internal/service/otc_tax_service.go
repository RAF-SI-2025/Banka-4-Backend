package service

import (
	"context"
	"fmt"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
)

// OtcTaxService implements the spec extension "Obračun poreza" for OTC option
// contracts. It records the 15% capital-gains tax for the two taxable OTC events:
//
//  1. Premija (seller): when an OTC option contract is created the seller receives
//     the premium, which is taxable income — tax = 15% × premium.
//
//  2. Iskorišćavanje opcije (buyer): when the buyer exercises a profitable option
//     the realized gain is taxable —
//     tax = 15% × ((market price − strike price) × quantity − premium).
//
// Izuzetak: aktuari koji trguju u ime banke ne plaćaju ovaj porez — njihov profit
// (uključujući premije) ostaje kao profit banke, isto kao kod dividendi
// (vidi DividendPayoutService, OwnerTypeBank). OTC ugovori su denominovani u RSD
// (PremiumRSD, StrikePriceRSD), pa se i porez evidentira u RSD.
type OtcTaxService struct {
	taxRecorder   TaxRecorder
	userClient    client.UserServiceClient
	stockRepo     repository.StockRepository
	bankingClient client.BankingClient
}

func NewOtcTaxService(
	taxRecorder TaxRecorder,
	userClient client.UserServiceClient,
	stockRepo repository.StockRepository,
	bankingClient client.BankingClient,
) *OtcTaxService {
	return &OtcTaxService{
		taxRecorder:   taxRecorder,
		userClient:    userClient,
		stockRepo:     stockRepo,
		bankingClient: bankingClient,
	}
}

// RecordPremiumTax records the seller's tax on a received OTC premium.
// Tax = 15% × premium. Actuaries (bank) are exempt.
func (s *OtcTaxService) RecordPremiumTax(ctx context.Context, contract *model.OtcOptionContract) error {
	if contract == nil || contract.PremiumRSD <= 0 {
		return nil
	}

	actuary, err := s.isActuary(ctx, contract.SellerID)
	if err != nil {
		return err
	}
	if actuary {
		return nil
	}

	// RecordTax computes 15% of the supplied base, so the premium is the base.
	return s.taxRecorder.RecordTax(ctx, contract.SellerAccountNumber, nil, contract.PremiumRSD, "RSD")
}

// RecordExerciseTax records the buyer's tax on the profit realized by exercising
// an OTC option. Taxable base = (market price − strike) × quantity − premium.
// Nothing is recorded when the base is not positive or when the buyer is an actuary.
func (s *OtcTaxService) RecordExerciseTax(ctx context.Context, contract *model.OtcOptionContract) error {
	if contract == nil {
		return nil
	}

	marketPriceRSD, err := s.marketPriceRSD(ctx, contract.StockAssetID)
	if err != nil {
		return err
	}

	taxableBase := (marketPriceRSD-contract.StrikePriceRSD)*float64(contract.Amount) - contract.PremiumRSD
	if taxableBase <= 0 {
		return nil
	}

	actuary, err := s.isActuary(ctx, contract.BuyerID)
	if err != nil {
		return err
	}
	if actuary {
		return nil
	}

	return s.taxRecorder.RecordTax(ctx, contract.BuyerAccountNumber, nil, taxableBase, "RSD")
}

// marketPriceRSD resolves the current market price of a stock (by asset id) in RSD.
func (s *OtcTaxService) marketPriceRSD(ctx context.Context, stockAssetID uint) (float64, error) {
	stocks, err := s.stockRepo.FindByAssetIDs(ctx, []uint{stockAssetID})
	if err != nil {
		return 0, errors.InternalErr(err)
	}

	var stock *model.Stock
	for i := range stocks {
		if stocks[i].AssetID == stockAssetID {
			stock = &stocks[i]
			break
		}
	}
	if stock == nil || stock.Listing == nil {
		return 0, errors.InternalErr(fmt.Errorf("no listing for stock asset %d", stockAssetID))
	}

	price := stock.Listing.Price
	currency := "RSD"
	if stock.Listing.Exchange != nil && stock.Listing.Exchange.Currency != "" {
		currency = stock.Listing.Exchange.Currency
	}

	if currency == "RSD" {
		return price, nil
	}

	priceRSD, err := s.bankingClient.ConvertCurrency(ctx, price, currency, "RSD")
	if err != nil {
		return 0, errors.InternalErr(err)
	}
	return priceRSD, nil
}

// isActuary reports whether an OTC participant trades on behalf of the bank.
//
// OTC participant ids are client ids (offers are created by clients on
// client-owned stock), so the canonical signal is the presence of a client
// record: when the user-service has a client with this id the participant is a
// regular client and owes tax; otherwise the participant is treated as a bank
// actuary and is exempt.
func (s *OtcTaxService) isActuary(ctx context.Context, participantID uint) (bool, error) {
	resp, err := s.userClient.GetClientById(ctx, uint64(participantID))
	if err != nil {
		// No client with this id — treat as a bank actuary (exempt).
		return true, nil
	}
	if resp == nil || resp.Id == 0 {
		return true, nil
	}
	return false, nil
}
