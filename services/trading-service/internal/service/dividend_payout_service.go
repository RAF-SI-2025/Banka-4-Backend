package service

import (
	"context"
	"fmt"
	"log"

	"time"

	commonerrors "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/config"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
)

const dividendTaxRate = 0.15

type DividendPayoutService struct {
	dividendRepo  repository.DividendPayoutRepository
	ownershipRepo repository.AssetOwnershipRepository
	stockRepo     repository.StockRepository
	listingRepo   repository.ListingRepository
	taxService    *TaxService
	bankingClient client.BankingClient
	cfg           *config.Configuration
}

func NewDividendPayoutService(
	dividendRepo repository.DividendPayoutRepository,
	ownershipRepo repository.AssetOwnershipRepository,
	stockRepo repository.StockRepository,
	listingRepo repository.ListingRepository,
	taxService *TaxService,
	bankingClient client.BankingClient,
	cfg *config.Configuration,
) *DividendPayoutService {
	return &DividendPayoutService{
		dividendRepo:  dividendRepo,
		ownershipRepo: ownershipRepo,
		stockRepo:     stockRepo,
		listingRepo:   listingRepo,
		taxService:    taxService,
		bankingClient: bankingClient,
		cfg:           cfg,
	}
}

// ProcessDividends iterates all stocks with DividendYield > 0, finds every owner
// and pays them proportionally. Called by DividendPayoutJob on the last business
// day of March, June, September and December.
func (s *DividendPayoutService) ProcessDividends(ctx context.Context) error {
	stocks, err := s.stockRepo.FindAll(ctx)
	if err != nil {
		return fmt.Errorf("dividend: load stocks: %w", err)
	}

	paymentDate := time.Now()

	for _, stock := range stocks {
		if stock.DividendYield <= 0 {
			continue
		}
		if stock.Listing == nil || stock.Listing.Price == 0 {
			log.Printf("[dividend] stock %d has no listing/price, skipping", stock.StockID)
			continue
		}

		if err := s.processStockDividend(ctx, stock, paymentDate); err != nil {
			// Log and continue — one bad stock should not block others
			log.Printf("[dividend] error processing stock %d: %v", stock.StockID, err)
		}
	}

	return nil
}

func (s *DividendPayoutService) processStockDividend(ctx context.Context, stock model.Stock, paymentDate time.Time) error {
	// Fetch all ownerships for this asset
	ownerships, err := s.ownershipRepo.FindAllByAssetIDs(ctx, []uint{stock.AssetID})
	if err != nil {
		return fmt.Errorf("load ownerships for asset %d: %w", stock.AssetID, err)
	}

	price := stock.Listing.Price
	currencyCode, err := s.resolveCurrencyForStock(ctx, stock)
	if err != nil {
		log.Printf("[dividend] cannot resolve currency for stock %d: %v", stock.StockID, err)
		currencyCode = "RSD"
	}

	for _, ownership := range ownerships {
		if ownership.Amount <= 0 {
			continue
		}
		if err := s.payOwner(ctx, stock, ownership, price, currencyCode, paymentDate); err != nil {
			log.Printf("[dividend] failed to pay owner %d for stock %d: %v", ownership.UserId, stock.StockID, err)
		}
	}

	return nil
}

func (s *DividendPayoutService) payOwner(
	ctx context.Context,
	stock model.Stock,
	ownership model.AssetOwnership,
	price float64,
	currencyCode string,
	paymentDate time.Time,
) error {
	gross := ownership.Amount * price * (stock.DividendYield / 400.0)
	if gross <= 0 {
		return nil
	}

	isBankOwned := ownership.OwnerType == model.OwnerTypeActuary || ownership.OwnerType == model.OwnerTypeFund

	// Aktuari i fondovi: dividenda ide u profit banke, nema eksternog transfera
	if isBankOwned {
		payout := &model.DividendPayout{
			AssetOwnershipID: ownership.AssetOwnershipID,
			Quantity:         ownership.Amount,
			PricePerShare:    price,
			GrossAmount:      gross,
			TaxAmount:        0,
			NetAmount:        gross,
			CurrencyCode:     currencyCode,
			AccountNumber:    s.cfg.DividendAccountNumber, // bankini račun
			PaymentDate:      paymentDate,
		}
		if err := s.dividendRepo.Save(ctx, payout); err != nil {
			log.Printf("[dividend] failed to persist actuary payout record: %v", err)
		}
		return nil
	}

	// Klijenti: normalan flow
	accountNumber, finalCurrency, err := s.resolveTargetAccount(ctx, ownership, currencyCode)
	if err != nil {
		return fmt.Errorf("resolve account: %w", err)
	}

	payableGross := gross
	if finalCurrency != currencyCode {
		converted, err := s.bankingClient.ConvertCurrency(ctx, gross, currencyCode, finalCurrency)
		if err != nil {
			return fmt.Errorf("currency conversion: %w", err)
		}
		payableGross = converted
	}

	taxAmount := payableGross * dividendTaxRate
	net := payableGross - taxAmount

	_, err = s.bankingClient.CreatePaymentWithoutVerification(ctx, &pb.CreatePaymentRequest{
		RecipientAccountNumber: accountNumber,
		PayerAccountNumber:     s.cfg.DividendAccountNumber,
		RecipientName:          "Dividend payout",
		Amount:                 net,
		PaymentCode:            "290",
		Purpose:                fmt.Sprintf("Dividenda za akciju %s", stock.Asset.Ticker),
	})
	if err != nil {
		return fmt.Errorf("payment failed: %w", err)
	}

	if taxAmount > 0 {
		if err := s.taxService.RecordTax(ctx, accountNumber, nil, payableGross, finalCurrency); err != nil {
			log.Printf("[dividend] tax record failed for account %s: %v", accountNumber, err)
		}
	}

	payout := &model.DividendPayout{
		AssetOwnershipID: ownership.AssetOwnershipID,
		Quantity:         ownership.Amount,
		PricePerShare:    price,
		GrossAmount:      payableGross,
		TaxAmount:        taxAmount,
		NetAmount:        net,
		CurrencyCode:     finalCurrency,
		AccountNumber:    accountNumber,
		PaymentDate:      paymentDate,
	}
	if err := s.dividendRepo.Save(ctx, payout); err != nil {
		log.Printf("[dividend] failed to persist payout record: %v", err)
	}

	return nil
}

// resolveTargetAccount implements the fallback chain from the spec:
//  1. Original account (the one used to buy the stock)
//  2. Client's default account in the stock's currency
//  3. Convert to RSD and use default RSD account
func (s *DividendPayoutService) resolveTargetAccount(
	ctx context.Context,
	ownership model.AssetOwnership,
	preferredCurrency string,
) (accountNumber string, currency string, err error) {
	// We don't store the original purchase account on AssetOwnership directly,
	// so we try to get the client's accounts and find one matching the currency.
	accountsResp, err := s.bankingClient.GetAccountsByClientID(ctx, uint64(ownership.UserId))
	if err != nil {
		return "", "", fmt.Errorf("get client accounts: %w", err)
	}

	// 1. Look for an account in the preferred currency
	for _, acc := range accountsResp.Accounts {
		accCurrency, err := s.bankingClient.GetAccountCurrency(ctx, acc.AccountNumber)
		if err != nil {
			continue
		}
		if accCurrency == preferredCurrency {
			return acc.AccountNumber, preferredCurrency, nil
		}
	}

	// 2. Fall back to RSD account
	for _, acc := range accountsResp.Accounts {
		accCurrency, err := s.bankingClient.GetAccountCurrency(ctx, acc.AccountNumber)
		if err != nil {
			continue
		}
		if accCurrency == "RSD" {
			return acc.AccountNumber, "RSD", nil
		}
	}

	// 3. No account found at all
	return "", "", commonerrors.InternalErr(fmt.Errorf("no suitable account found for user %d", ownership.UserId))
}

func (s *DividendPayoutService) resolveCurrencyForStock(_ context.Context, stock model.Stock) (string, error) {
	if stock.Listing == nil {
		return "USD", nil
	}
	if stock.Listing.Exchange != nil && stock.Listing.Exchange.Currency != "" {
		return stock.Listing.Exchange.Currency, nil
	}
	if stock.Listing.ExchangeMIC == model.SimulatedExchangeMIC {
		return "RSD", nil
	}
	return "USD", nil
}

// GetAllPayouts returns every dividend payout — used by the admin/supervisor endpoint.
func (s *DividendPayoutService) GetAllPayouts(ctx context.Context) ([]model.DividendPayout, error) {
	payouts, err := s.dividendRepo.FindAll(ctx)
	if err != nil {
		return nil, commonerrors.InternalErr(err)
	}
	return payouts, nil
}

func (s *DividendPayoutService) GetPayoutsForAssetOwnership(ctx context.Context, assetOwnershipID uint) ([]model.DividendPayout, error) {
	payouts, err := s.dividendRepo.FindAllByAssetOwnershipID(ctx, assetOwnershipID)
	if err != nil {
		return nil, commonerrors.InternalErr(err)
	}
	return payouts, nil
}
