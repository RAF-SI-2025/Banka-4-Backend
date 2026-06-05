package service

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	commonerrors "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/config"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
)

const dividendTaxRate = 0.15

// defaultReinvestmentPercent is the split used when a fund has no explicit setting.
const defaultReinvestmentPercent = 50.0

type DividendPayoutService struct {
	dividendRepo  repository.DividendPayoutRepository
	ownershipRepo repository.AssetOwnershipRepository
	stockRepo     repository.StockRepository
	listingRepo   repository.ListingRepository
	taxService    *TaxService
	bankingClient client.BankingClient
	cfg           *config.Configuration
	fundRepo      repository.InvestmentFundRepository
	positionRepo  repository.ClientFundPositionRepository
	orderService  *OrderService
}

func NewDividendPayoutService(
	dividendRepo repository.DividendPayoutRepository,
	ownershipRepo repository.AssetOwnershipRepository,
	stockRepo repository.StockRepository,
	listingRepo repository.ListingRepository,
	taxService *TaxService,
	bankingClient client.BankingClient,
	cfg *config.Configuration,
	fundRepo repository.InvestmentFundRepository,
	positionRepo repository.ClientFundPositionRepository,
	orderService *OrderService,
) *DividendPayoutService {
	return &DividendPayoutService{
		dividendRepo:  dividendRepo,
		ownershipRepo: ownershipRepo,
		stockRepo:     stockRepo,
		listingRepo:   listingRepo,
		taxService:    taxService,
		bankingClient: bankingClient,
		cfg:           cfg,
		fundRepo:      fundRepo,
		positionRepo:  positionRepo,
		orderService:  orderService,
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

	isBankOwned := ownership.OwnerType == model.OwnerTypeBank
	isFundOwned := ownership.OwnerType == model.OwnerTypeFund

	// Aktuari: dividenda ide u profit banke, nema eksternog transfera
	if isBankOwned {
		payout := &model.DividendPayout{
			AssetOwnershipID: ownership.AssetOwnershipID,
			Quantity:         ownership.Amount,
			PricePerShare:    price,
			GrossAmount:      gross,
			TaxAmount:        0,
			NetAmount:        gross,
			CurrencyCode:     currencyCode,
			AccountNumber:    s.cfg.DividendAccountNumber,
			PaymentDate:      paymentDate,
		}
		if err := s.dividendRepo.Save(ctx, payout); err != nil {
			log.Printf("[dividend] failed to persist actuary payout record: %v", err)
		}
		return nil
	}

	// Fondovi: dividenda ide u fond, reinvestira se i distribuira klijentima
	if isFundOwned {
		return s.payFundOwner(ctx, stock, ownership, price, currencyCode, paymentDate)
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

// payFundOwner handles dividends for fund-owned stocks:
//  1. The gross dividend is credited to the fund's account.
//  2. A portion (DividendReinvestmentPercent) is reinvested by buying more shares.
//  3. The remainder is distributed to each fund client proportional to their units.
func (s *DividendPayoutService) payFundOwner(
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

	fund, err := s.fundRepo.FindByID(ctx, ownership.UserId)
	if err != nil {
		return fmt.Errorf("fund lookup: %w", err)
	}
	if fund == nil {
		return fmt.Errorf("fund %d not found", ownership.UserId)
	}

	// Convert gross to RSD — fund accounts are denominated in RSD
	grossRSD := gross
	if currencyCode != "RSD" {
		grossRSD, err = s.bankingClient.ConvertCurrency(ctx, gross, currencyCode, "RSD")
		if err != nil {
			return fmt.Errorf("currency conversion for fund dividend: %w", err)
		}
	}

	_, err = s.bankingClient.CreatePaymentWithoutVerification(ctx, &pb.CreatePaymentRequest{
		RecipientAccountNumber: fund.AccountNumber,
		PayerAccountNumber:     s.cfg.DividendAccountNumber,
		RecipientName:          "Dividend payout",
		Amount:                 grossRSD,
		PaymentCode:            "290",
		Purpose:                fmt.Sprintf("Dividenda za akciju %s", stock.Asset.Ticker),
	})
	if err != nil {
		return fmt.Errorf("fund dividend inflow payment failed: %w", err)
	}

	payout := &model.DividendPayout{
		AssetOwnershipID: ownership.AssetOwnershipID,
		Quantity:         ownership.Amount,
		PricePerShare:    price,
		GrossAmount:      grossRSD,
		TaxAmount:        0,
		NetAmount:        grossRSD,
		CurrencyCode:     "RSD",
		AccountNumber:    fund.AccountNumber,
		PaymentDate:      paymentDate,
	}
	if err := s.dividendRepo.Save(ctx, payout); err != nil {
		log.Printf("[dividend] failed to persist fund payout: %v", err)
	}

	reinvestPct := defaultReinvestmentPercent
	if fund.DividendReinvestmentPercent != nil {
		reinvestPct = *fund.DividendReinvestmentPercent
	}
	if reinvestPct < 0 {
		reinvestPct = 0
	}
	if reinvestPct > 100 {
		reinvestPct = 100
	}

	reinvestAmount := grossRSD * reinvestPct / 100.0
	payoutAmount := grossRSD - reinvestAmount

	if reinvestAmount > 0 && s.orderService != nil && stock.Listing != nil && stock.Listing.ListingID != 0 {
		priceRSD := price
		if currencyCode != "RSD" {
			if converted, convErr := s.bankingClient.ConvertCurrency(ctx, price, currencyCode, "RSD"); convErr == nil {
				priceRSD = converted
			}
		}
		if priceRSD > 0 {
			quantity := uint(math.Floor(reinvestAmount / priceRSD))
			if quantity > 0 {
				order, orderErr := s.orderService.CreateFundReinvestmentOrder(ctx, fund, stock.Listing.ListingID, quantity)
				if orderErr != nil {
					log.Printf("[dividend] reinvestment order failed for fund %d stock %d: %v", fund.FundID, stock.StockID, orderErr)
				} else if order.Status == model.OrderStatusApproved {
					if processErr := s.orderService.processOrder(ctx, order); processErr != nil {
						log.Printf("[dividend] reinvestment order processing failed: %v", processErr)
					}
				}
			}
		}
	}

	if payoutAmount <= 0 || s.positionRepo == nil {
		return nil
	}

	positions, err := s.positionRepo.FindByFund(ctx, fund.FundID)
	if err != nil {
		log.Printf("[dividend] failed to load client positions for fund %d: %v", fund.FundID, err)
		return nil
	}

	var totalUnits float64
	for _, pos := range positions {
		totalUnits += unitsFromPosition(pos)
	}

	if totalUnits <= 0 {
		return nil
	}

	for _, pos := range positions {

		clientUnits := unitsFromPosition(pos)
		if clientUnits <= 0 {
			continue
		}
		clientShare := (clientUnits / totalUnits) * payoutAmount
		if clientShare <= 0 {
			continue
		}
		if err := s.payFundClient(ctx, fund, pos, clientShare, "RSD", paymentDate); err != nil {
			log.Printf("[dividend] failed to pay client %d from fund %d: %v", pos.ClientID, fund.FundID, err)
		}
	}

	return nil
}

// payFundClient sends a client's proportional dividend share from the fund's account.
func (s *DividendPayoutService) payFundClient(
	ctx context.Context,
	fund *model.InvestmentFund,
	pos model.ClientFundPosition,
	shareAmount float64,
	currencyCode string,
	paymentDate time.Time,
) error {
	fakeOwnership := model.AssetOwnership{
		UserId:    pos.ClientID,
		OwnerType: pos.OwnerType,
	}
	accountNumber, finalCurrency, err := s.resolveTargetAccount(ctx, fakeOwnership, currencyCode)
	if err != nil {
		return fmt.Errorf("resolve client account: %w", err)
	}

	payableAmount := shareAmount
	if finalCurrency != currencyCode {
		payableAmount, err = s.bankingClient.ConvertCurrency(ctx, shareAmount, currencyCode, finalCurrency)
		if err != nil {
			return fmt.Errorf("currency conversion: %w", err)
		}
	}

	taxAmount := payableAmount * dividendTaxRate
	net := payableAmount - taxAmount

	_, err = s.bankingClient.CreatePaymentWithoutVerification(ctx, &pb.CreatePaymentRequest{
		RecipientAccountNumber: accountNumber,
		PayerAccountNumber:     fund.AccountNumber,
		RecipientName:          "Dividend payout",
		Amount:                 net,
		PaymentCode:            "290",
		Purpose:                fmt.Sprintf("Dividenda od fonda %s", fund.Name),
	})
	if err != nil {
		return fmt.Errorf("client dividend payment failed: %w", err)
	}

	if taxAmount > 0 {
		if err := s.taxService.RecordTax(ctx, accountNumber, nil, payableAmount, finalCurrency); err != nil {
			log.Printf("[dividend] tax record failed for client account %s: %v", accountNumber, err)
		}
	}

	return nil
}

// resolveTargetAccount implements the fallback chain for a given asset ownership:
//   - Fund ownership: returns the fund's own bank account.
//   - Client/Bank ownership: (1) account in the stock's currency, (2) RSD account, (3) any account.
func (s *DividendPayoutService) resolveTargetAccount(
	ctx context.Context,
	ownership model.AssetOwnership,
	preferredCurrency string,
) (accountNumber string, currency string, err error) {
	// For fund-owned assets, the target is the fund's own bank account
	if ownership.OwnerType == model.OwnerTypeFund {
		fund, err := s.fundRepo.FindByID(ctx, ownership.UserId)
		if err != nil {
			return "", "", fmt.Errorf("resolve fund account: %w", err)
		}
		if fund == nil {
			return "", "", commonerrors.InternalErr(fmt.Errorf("fund %d not found", ownership.UserId))
		}
		return fund.AccountNumber, "RSD", nil
	}

	accountsResp, err := s.bankingClient.GetAccountsByClientID(ctx, uint64(ownership.UserId))
	if err != nil {
		return "", "", fmt.Errorf("get client accounts: %w", err)
	}

	type accountInfo struct {
		number   string
		currency string
	}

	var accounts []accountInfo

	for _, acc := range accountsResp.Accounts {
		accCurrency, err := s.bankingClient.GetAccountCurrency(ctx, acc.AccountNumber)
		if err != nil {
			continue
		}

		accounts = append(accounts, accountInfo{
			number:   acc.AccountNumber,
			currency: accCurrency,
		})
	}

	// No usable accounts at all
	if len(accounts) == 0 {
		return "", "", commonerrors.InternalErr(
			fmt.Errorf("no usable account found for user %d", ownership.UserId),
		)
	}

	// 1. Preferred currency
	for _, acc := range accounts {
		if acc.currency == preferredCurrency {
			return acc.number, acc.currency, nil
		}
	}

	// 2. RSD account
	for _, acc := range accounts {
		if acc.currency == "RSD" {
			return acc.number, acc.currency, nil
		}
	}

	// 3. Any account
	return accounts[0].number, accounts[0].currency, nil
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
