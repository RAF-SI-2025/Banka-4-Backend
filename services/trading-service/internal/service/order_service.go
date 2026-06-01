package service

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/audit"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/permission"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
)

const (
	orderExecutionPollInterval = time.Second
	stopCheckInterval          = 5 * time.Second
	executionRetryInterval     = 30 * time.Second
	maxOrdersPerTick           = 25
	afterHoursWindow           = 4 * time.Hour
	afterHoursExecutionDelay   = 30 * time.Minute
)

type exchangeSession struct {
	IsClosed   bool
	IsOpen     bool
	AfterHours bool
	NextOpen   time.Time
	LocalNow   time.Time
	CloseTime  time.Time
}

type tradeSettlement struct {
	SourceAmount        float64
	SourceCurrency      string
	DestinationAmount   float64
	DestinationCurrency string
}

type placeOrderParams struct {
	AccountNumber    string
	ListingID        uint
	OrderType        model.OrderType
	Direction        model.OrderDirection
	Quantity         uint
	LimitValue       *float64
	StopValue        *float64
	AllOrNone        bool
	Margin           bool
	OrderOwnerUserID uint
	OrderOwnerType   model.OwnerType
	AssetOwnerUserID uint
	AssetOwnerType   model.OwnerType
	CommissionExempt bool
	account          *pb.GetAccountByNumberResponse
}

type OrderService struct {
	orderRepo            repository.OrderRepository
	orderTransactionRepo repository.OrderTransactionRepository
	exchangeRepo         repository.ExchangeRepository
	listingRepo          repository.ListingRepository
	assetOwnershipRepo   repository.AssetOwnershipRepository
	futuresRepo          repository.FuturesContractRepository
	optionRepo           repository.OptionRepository
	fundRepo             repository.InvestmentFundRepository
	userClient           client.UserServiceClient
	bankingClient        client.BankingClient
	taxService           TaxRecorder
	auditSvc             *audit.Service
	now                  func() time.Time
	rng                  *rand.Rand

	mu     sync.Mutex
	cancel context.CancelFunc
}

func NewOrderService(
	orderRepo repository.OrderRepository,
	orderTransactionRepo repository.OrderTransactionRepository,
	exchangeRepo repository.ExchangeRepository,
	listingRepo repository.ListingRepository,
	assetOwnershipRepo repository.AssetOwnershipRepository,
	futuresRepo repository.FuturesContractRepository,
	optionRepo repository.OptionRepository,
	fundRepo repository.InvestmentFundRepository,
	userClient client.UserServiceClient,
	bankingClient client.BankingClient,
	taxService TaxRecorder,
	auditSvc *audit.Service,
) *OrderService {
	return &OrderService{
		orderRepo:            orderRepo,
		orderTransactionRepo: orderTransactionRepo,
		exchangeRepo:         exchangeRepo,
		listingRepo:          listingRepo,
		assetOwnershipRepo:   assetOwnershipRepo,
		futuresRepo:          futuresRepo,
		optionRepo:           optionRepo,
		fundRepo:             fundRepo,
		userClient:           userClient,
		bankingClient:        bankingClient,
		taxService:           taxService,
		auditSvc:             auditSvc,
		now:                  time.Now,
		rng:                  rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *OrderService) Start() {
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.mu.Unlock()

	ticker := time.NewTicker(orderExecutionPollInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.processDueOrders(ctx); err != nil {
					log.Printf("[orders] execution tick failed: %v", err)
				}
			}
		}
	}()
}

func (s *OrderService) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

func (s *OrderService) GetOrders(ctx context.Context, query dto.ListOrdersQuery) ([]model.Order, int64, error) {
	orders, total, err := s.orderRepo.FindAll(ctx, query.Page, query.PageSize, nil, nil, query.Status, query.Direction, query.IsDone)
	if err != nil {
		return nil, 0, errors.InternalErr(err)
	}

	return orders, total, nil
}

func (s *OrderService) CreateOrder(ctx context.Context, req dto.CreateOrderRequest) (*model.Order, error) {
	if err := validateOrderTypeFields(placeOrderParams{OrderType: req.OrderType, LimitValue: req.LimitValue, StopValue: req.StopValue}); err != nil {
		return nil, err
	}

	authCtx := auth.GetAuthFromContext(ctx)
	if authCtx == nil {
		return nil, errors.UnauthorizedErr("not authenticated")
	}

	account, err := s.validateAccount(ctx, req.AccountNumber, authCtx)
	if err != nil {
		return nil, err
	}

	ownerType := model.OwnerTypeClient
	userID := authCtx.ClientID
	if authCtx.IdentityType == auth.IdentityEmployee {
		ownerType = model.OwnerTypeActuary
		userID = authCtx.EmployeeID
	}

	return s.placeOrder(ctx, authCtx, placeOrderParams{
		AccountNumber:    req.AccountNumber,
		ListingID:        req.ListingID,
		OrderType:        req.OrderType,
		Direction:        req.Direction,
		Quantity:         req.Quantity,
		LimitValue:       req.LimitValue,
		StopValue:        req.StopValue,
		AllOrNone:        req.AllOrNone,
		Margin:           req.Margin,
		OrderOwnerUserID: *userID,
		OrderOwnerType:   ownerType,
		AssetOwnerUserID: *userID,
		AssetOwnerType:   ownerType,
		CommissionExempt: authCtx.IdentityType == auth.IdentityEmployee,
		account:          account,
	})
}

func (s *OrderService) CreateFundOrder(ctx context.Context, req dto.CreateFundOrderRequest) (*model.Order, error) {
	if err := validateOrderTypeFields(placeOrderParams{OrderType: req.OrderType, LimitValue: req.LimitValue, StopValue: req.StopValue}); err != nil {
		return nil, err
	}

	authCtx := auth.GetAuthFromContext(ctx)
	if authCtx == nil || authCtx.IdentityType != auth.IdentityEmployee || authCtx.EmployeeID == nil {
		return nil, errors.UnauthorizedErr("only employees can place fund orders")
	}

	fund, err := s.fundRepo.FindByID(ctx, req.FundID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if fund == nil {
		return nil, errors.NotFoundErr("investment fund not found")
	}
	if fund.ManagerID != *authCtx.EmployeeID {
		return nil, errors.ForbiddenErr("you are not the manager of this fund")
	}

	account, err := s.bankingClient.GetAccountByNumber(ctx, fund.AccountNumber)
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			return nil, errors.NotFoundErr("account not found")
		}
		return nil, errors.ServiceUnavailableErr(err)
	}

	return s.placeOrder(ctx, authCtx, placeOrderParams{
		AccountNumber:    fund.AccountNumber,
		ListingID:        req.ListingID,
		OrderType:        req.OrderType,
		Direction:        req.Direction,
		Quantity:         req.Quantity,
		LimitValue:       req.LimitValue,
		StopValue:        req.StopValue,
		AllOrNone:        req.AllOrNone,
		Margin:           req.Margin,
		OrderOwnerUserID: *authCtx.EmployeeID,
		OrderOwnerType:   model.OwnerTypeActuary,
		AssetOwnerUserID: req.FundID,
		AssetOwnerType:   model.OwnerTypeFund,
		CommissionExempt: true,
		account:          account,
	})
}

func (s *OrderService) CreateFundLiquidationOrder(ctx context.Context, fund *model.InvestmentFund, listingID uint, quantity uint) (*model.Order, error) {
	if fund == nil {
		return nil, errors.NotFoundErr("investment fund not found")
	}

	account, err := s.bankingClient.GetAccountByNumber(ctx, fund.AccountNumber)
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			return nil, errors.NotFoundErr("account not found")
		}
		return nil, errors.ServiceUnavailableErr(err)
	}

	managerID := fund.ManagerID
	managerCtx := auth.SetAuthOnContext(ctx, &auth.AuthContext{
		IdentityID:   managerID,
		IdentityType: auth.IdentityEmployee,
		EmployeeID:   &managerID,
	})
	managerAuth := auth.GetAuthFromContext(managerCtx)

	return s.placeOrder(managerCtx, managerAuth, placeOrderParams{
		AccountNumber:    fund.AccountNumber,
		ListingID:        listingID,
		OrderType:        model.OrderTypeMarket,
		Direction:        model.OrderDirectionSell,
		Quantity:         quantity,
		AllOrNone:        false,
		Margin:           false,
		OrderOwnerUserID: fund.ManagerID,
		OrderOwnerType:   model.OwnerTypeActuary,
		AssetOwnerUserID: fund.FundID,
		AssetOwnerType:   model.OwnerTypeFund,
		CommissionExempt: true,
		account:          account,
	})
}

func (s *OrderService) placeOrder(ctx context.Context, authCtx *auth.AuthContext, p placeOrderParams) (*model.Order, error) {
	listing, err := s.listingRepo.FindByID(ctx, p.ListingID, 0)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if listing == nil {
		return nil, errors.NotFoundErr("listing not found")
	}

	if err := s.validateSettlementDate(ctx, listing); err != nil {
		return nil, err
	}

	exchange, err := s.exchangeRepo.FindByMicCode(ctx, listing.ExchangeMIC)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if exchange == nil {
		return nil, errors.NotFoundErr("exchange not found")
	}

	if err := s.validateMarginRequirements(ctx, authCtx, p.Margin, listing, exchange, p.account); err != nil {
		return nil, err
	}

	if p.Direction == model.OrderDirectionSell && listing.Asset != nil {
		if err := s.validateSellOwnership(ctx, p.AssetOwnerUserID, p.AssetOwnerType, listing.AssetID, float64(p.Quantity)); err != nil {
			return nil, err
		}
	}

	session := s.resolveExchangeSession(exchange)

	order := model.Order{
		OrderOwnerUserID:  p.OrderOwnerUserID,
		OrderOwnerType:    p.OrderOwnerType,
		AssetOwnerUserID:  p.AssetOwnerUserID,
		AssetOwnerType:    p.AssetOwnerType,
		AccountNumber:     p.AccountNumber,
		ListingID:         p.ListingID,
		Listing:           *listing,
		OrderType:         p.OrderType,
		Direction:         p.Direction,
		Quantity:          p.Quantity,
		ContractSize:      s.resolveContractSize(ctx, listing),
		PricePerUnit:      calculateInitialPricePerUnit(p, listing),
		LimitValue:        p.LimitValue,
		StopValue:         p.StopValue,
		AllOrNone:         p.AllOrNone,
		Margin:            p.Margin,
		AfterHours:        session.AfterHours,
		Triggered:         p.OrderType == model.OrderTypeMarket || p.OrderType == model.OrderTypeLimit,
		CommissionCharged: false,
		CommissionExempt:  p.CommissionExempt,
		IsDone:            false,
		CreatedAt:         s.now(),
		UpdatedAt:         s.now(),
	}

	order.Status = s.resolveOrderStatus(ctx, authCtx, &order)
	if order.Status == model.OrderStatusApproved {
		nextExecutionAt := s.initialExecutionTime(session, order.AfterHours)
		order.NextExecutionAt = &nextExecutionAt

		if order.PricePerUnit == nil {
			return nil, errors.BadRequestErr("trying to create an order without a set price per unit")
		}

		grossAmount := *order.PricePerUnit * float64(order.Quantity)
		if err := s.updateActuaryUsedLimit(ctx, &order, grossAmount, exchange.Currency); err != nil {
			return nil, errors.InternalErr(err)
		}
	}

	if err := s.orderRepo.Create(ctx, &order); err != nil {
		return nil, errors.InternalErr(err)
	}

	return &order, nil
}

func (s *OrderService) resolveContractSize(ctx context.Context, listing *model.Listing) float64 {
	if listing.Asset == nil {
		return 1
	}
	switch listing.Asset.AssetType {
	case model.AssetTypeFuture:
		contracts, err := s.futuresRepo.FindByAssetIDs(ctx, []uint{listing.AssetID})
		if err != nil || len(contracts) == 0 {
			return 1
		}
		return contracts[0].ContractSize
	case model.AssetTypeOption:
		options, err := s.optionRepo.FindByAssetIDs(ctx, []uint{listing.AssetID})
		if err != nil || len(options) == 0 {
			return 100
		}
		return float64(options[0].ContractSize)
	case model.AssetTypeForexPair:
		return 1000
	default:
		return 1
	}
}

func (s *OrderService) ApproveOrder(ctx context.Context, orderID uint) (*model.Order, error) {
	order, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if order == nil {
		return nil, errors.NotFoundErr("order not found")
	}
	if order.Status != model.OrderStatusPending {
		return nil, errors.BadRequestErr("only pending orders can be approved")
	}

	authCtx := auth.GetAuthFromContext(ctx)
	if authCtx == nil {
		return nil, errors.UnauthorizedErr("not authenticated")
	}

	exchange, err := s.exchangeRepo.FindByMicCode(ctx, order.Listing.ExchangeMIC)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if exchange == nil {
		return nil, errors.NotFoundErr("exchange not found")
	}

	if err := s.validateSettlementDate(ctx, &order.Listing); err != nil {
		return nil, err
	}
	ownerType := model.OwnerTypeClient
	if authCtx.IdentityType == auth.IdentityEmployee {
		ownerType = model.OwnerTypeActuary
	}

	if order.Direction == model.OrderDirectionSell && order.Listing.Asset != nil {
		if err := s.validateSellOwnership(ctx, authCtx.IdentityID, ownerType, order.Listing.AssetID, float64(order.Quantity)); err != nil {
			return nil, err
		}
	}

	approverID := authCtx.IdentityID
	nextExecutionAt := s.initialExecutionTime(s.resolveExchangeSession(exchange), order.AfterHours)
	order.Status = model.OrderStatusApproved
	order.ApprovedBy = &approverID //TODO careful
	order.NextExecutionAt = &nextExecutionAt
	order.UpdatedAt = s.now()
	order.ReviewedAt = &order.UpdatedAt

	if order.PricePerUnit == nil {
		return nil, errors.BadRequestErr("trying to approve an order without a set price per unit")
	}

	grossAmount := *order.PricePerUnit * float64(order.Quantity)
	if err := s.updateActuaryUsedLimit(ctx, order, grossAmount, exchange.Currency); err != nil {
		return nil, errors.InternalErr(err)
	}

	if err := s.orderRepo.Save(ctx, order); err != nil {
		return nil, errors.InternalErr(err)
	}

	if authCtx.EmployeeID != nil {
		if err := s.auditSvc.Log(ctx, audit.ActionOrderApproved, *authCtx.EmployeeID, fmt.Sprintf("order_id=%d", orderID)); err != nil {
			return nil, errors.InternalErr(err)
		}
	}

	return order, nil
}

func (s *OrderService) DeclineOrder(ctx context.Context, orderID uint) (*model.Order, error) {
	order, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if order == nil {
		return nil, errors.NotFoundErr("order not found")
	}
	if order.Status != model.OrderStatusPending {
		return nil, errors.BadRequestErr("only pending orders can be declined")
	}

	authCtx := auth.GetAuthFromContext(ctx)
	if authCtx == nil {
		return nil, errors.UnauthorizedErr("not authenticated")
	}

	approverID := authCtx.IdentityID //TODO careful
	order.Status = model.OrderStatusDeclined
	order.ApprovedBy = &approverID
	order.IsDone = true
	order.NextExecutionAt = nil
	order.UpdatedAt = s.now()
	order.ReviewedAt = &order.UpdatedAt

	if err := s.orderRepo.Save(ctx, order); err != nil {
		return nil, errors.InternalErr(err)
	}

	if authCtx.EmployeeID != nil {
		if err := s.auditSvc.Log(ctx, audit.ActionOrderDeclined, *authCtx.EmployeeID, fmt.Sprintf("order_id=%d", orderID)); err != nil {
			return nil, errors.InternalErr(err)
		}
	}

	return order, nil
}

func (s *OrderService) CancelOrder(ctx context.Context, orderID uint) (*model.Order, error) {
	order, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if order == nil {
		return nil, errors.NotFoundErr("order not found")
	}

	authCtx := auth.GetAuthFromContext(ctx)
	if authCtx == nil {
		return nil, errors.UnauthorizedErr("not authenticated")
	}

	isOwner := false
	if authCtx.IdentityType == auth.IdentityEmployee {
		isOwner = order.OrderOwnerUserID == *authCtx.EmployeeID
	} else {
		isOwner = order.OrderOwnerUserID == *authCtx.ClientID
	}

	isSupervisor, err := s.checkSupervisor(ctx)
	if err != nil {
		return nil, err
	}
	if !isOwner && !isSupervisor {
		return nil, errors.ForbiddenErr("only the order owner or a supervisor can cancel an order")
	}
	if order.Status != model.OrderStatusPending && order.Status != model.OrderStatusApproved {
		return nil, errors.BadRequestErr("only pending or approved orders can be cancelled")
	}
	if order.IsDone {
		return nil, errors.BadRequestErr("cannot cancel a completed order")
	}

	order.Status = model.OrderStatusDeclined
	order.IsDone = true
	order.NextExecutionAt = nil
	order.UpdatedAt = s.now()

	if err := s.orderRepo.Save(ctx, order); err != nil {
		return nil, errors.InternalErr(err)
	}

	return order, nil
}

func (s *OrderService) processDueOrders(ctx context.Context) error {
	orders, err := s.orderRepo.FindReadyForExecution(ctx, s.now(), maxOrdersPerTick)
	if err != nil {
		return errors.InternalErr(err)
	}

	for i := range orders {
		order := orders[i]
		if err := s.processOrder(ctx, &order); err != nil {
			log.Printf("[orders] failed to process order %d: %v", order.OrderID, err)
		}
	}

	return nil
}

func (s *OrderService) processOrder(ctx context.Context, order *model.Order) error {
	listing, err := s.listingRepo.FindByID(ctx, order.ListingID, 0)
	if err != nil {
		return err
	}
	if listing == nil {
		return s.failOrder(ctx, order, model.OrderStatusDeclined)
	}
	order.Listing = *listing

	exchange, err := s.exchangeRepo.FindByMicCode(ctx, listing.ExchangeMIC)
	if err != nil {
		return err
	}
	if exchange == nil {
		return s.failOrder(ctx, order, model.OrderStatusDeclined)
	}

	session := s.resolveExchangeSession(exchange)
	if !session.IsOpen && !session.AfterHours {
		nextOpen := s.initialExecutionTime(session, order.AfterHours)
		order.NextExecutionAt = &nextOpen
		order.UpdatedAt = s.now()
		return s.orderRepo.Save(ctx, order)
	}

	if !order.Triggered {
		if !isStopConditionMet(order, listing) {
			nextExecutionAt := s.now().Add(stopCheckInterval)
			order.NextExecutionAt = &nextExecutionAt
			order.UpdatedAt = s.now()
			return s.orderRepo.Save(ctx, order)
		}
		order.Triggered = true
	}

	pricePerUnit, canExecute := resolveExecutionPrice(order, listing)
	if !canExecute {
		nextExecutionAt := s.now().Add(stopCheckInterval)
		order.NextExecutionAt = &nextExecutionAt
		order.UpdatedAt = s.now()
		return s.orderRepo.Save(ctx, order)
	}

	fillQty := s.resolveFillQuantity(order)
	if fillQty == 0 {
		nextExecutionAt := s.now().Add(stopCheckInterval)
		order.NextExecutionAt = &nextExecutionAt
		order.UpdatedAt = s.now()
		return s.orderRepo.Save(ctx, order)
	}

	grossAmount := float64(fillQty) * order.ContractSize * pricePerUnit
	commission := 0.0
	settlementAmount := grossAmount
	if !order.CommissionCharged && !order.CommissionExempt {
		commission = calculateCommission(order.OrderType, approximateOrderValue(order, pricePerUnit))
		if order.Direction == model.OrderDirectionBuy {
			settlementAmount += commission
		} else {
			settlementAmount -= commission
		}
	}
	if settlementAmount <= 0 {
		return s.failOrder(ctx, order, model.OrderStatusDeclined)
	}

	tradeCurrency := normalizeCurrencyCode(exchange.Currency)
	settlement, err := s.executeTradeSettlement(ctx, order, tradeCurrency, settlementAmount)
	if err != nil {
		st, ok := status.FromError(err)
		if ok && (st.Code() == codes.FailedPrecondition || st.Code() == codes.NotFound) {
			return s.failOrder(ctx, order, model.OrderStatusDeclined)
		}

		nextExecutionAt := s.now().Add(executionRetryInterval)
		order.NextExecutionAt = &nextExecutionAt
		order.UpdatedAt = s.now()
		if saveErr := s.orderRepo.Save(ctx, order); saveErr != nil {
			return saveErr
		}
		return err
	}

	orderTransaction := &model.OrderTransaction{
		OrderID:      order.OrderID,
		Quantity:     fillQty,
		PricePerUnit: pricePerUnit,
		TotalPrice:   grossAmount,
		Commission:   commission,
		ExecutedAt:   s.now(),
		CreatedAt:    s.now(),
	}
	if err := s.orderTransactionRepo.Create(ctx, orderTransaction); err != nil {
		return err
	}

	order.FilledQty += fillQty
	order.CommissionCharged = order.CommissionCharged || commission > 0
	order.PricePerUnit = &pricePerUnit
	order.UpdatedAt = s.now()
	if err := s.recordProfitTax(ctx, order, fillQty, pricePerUnit, tradeCurrency); err != nil {
		return err
	}
	if order.RemainingPortions() == 0 {
		order.IsDone = true
		order.NextExecutionAt = nil
	} else {
		nextExecutionAt := s.nextExecutionAt(ctx, order)
		order.NextExecutionAt = &nextExecutionAt
	}

	if err := s.orderRepo.Save(ctx, order); err != nil {
		return err
	}

	if err := s.updateAssetOwnership(ctx, order, fillQty, pricePerUnit, tradeCurrency); err != nil {
		return err
	}

	_ = settlement
	return nil
}

func (s *OrderService) updateActuaryUsedLimit(ctx context.Context, order *model.Order, grossAmount float64, tradeCurrency string) error {
	if order.OwnerType != model.OwnerTypeActuary || order.Direction != model.OrderDirectionBuy || grossAmount <= 0 {
		return nil
	}

	employee, err := s.userClient.GetEmployeeById(ctx, uint64(order.OrderOwnerUserID))
	if err != nil {
		return err
	}
	if !employee.IsAgent {
		return nil
	}

	amountRSD := grossAmount
	if tradeCurrency != "RSD" {
		amountRSD, err = s.bankingClient.ConvertCurrency(ctx, grossAmount, tradeCurrency, "RSD")
		if err != nil {
			return err
		}
	}

	_, err = s.userClient.IncrementUsedLimit(ctx, uint64(order.OrderOwnerUserID), amountRSD)
	if err != nil {
		return err
	}

	return nil
}

func assetOwner(order *model.Order) (uint, model.OwnerType) {
	if order.AssetOwnerUserID != 0 {
		return order.AssetOwnerUserID, order.AssetOwnerType
	}
	return order.OrderOwnerUserID, order.OrderOwnerType
}

func (s *OrderService) updateAssetOwnership(ctx context.Context, order *model.Order, fillQty uint, pricePerUnit float64, currency string) error {
	if order.Listing.Asset == nil {
		return fmt.Errorf("listing %d has no asset", order.ListingID)
	}

	fillAmount := float64(fillQty) * order.ContractSize
	assetID := order.Listing.AssetID
	ownerID, ownerType := assetOwner(order)

	existing, err := s.assetOwnershipRepo.FindByUserId(ctx, ownerID, ownerType)
	if err != nil {
		return err
	}

	var ownership *model.AssetOwnership
	for i := range existing {
		if existing[i].AssetID == assetID {
			ownership = &existing[i]
			break
		}
	}

	if ownership == nil {
		ownership = &model.AssetOwnership{
			UserId:    ownerID,
			OwnerType: ownerType,
			AssetID:   assetID,
		}
	}

	switch order.Direction {
	case model.OrderDirectionBuy:
		pricePerUnitRSD := pricePerUnit
		if currency != "RSD" {
			pricePerUnitRSD, err = s.bankingClient.ConvertCurrency(ctx, pricePerUnit, currency, "RSD")
			if err != nil {
				return err
			}
		}
		newAmount := ownership.Amount + fillAmount
		if newAmount > 0 {
			ownership.AvgBuyPriceRSD = (ownership.AvgBuyPriceRSD*ownership.Amount + pricePerUnitRSD*fillAmount) / newAmount
		}
		ownership.Amount = newAmount
	case model.OrderDirectionSell:
		if ownership.Amount < fillAmount {
			return errors.BadRequestErr("insufficient asset ownership to sell")
		}
		ownership.Amount -= fillAmount
	}

	ownership.UpdatedAt = s.now()
	return s.assetOwnershipRepo.Upsert(ctx, ownership)
}

func (s *OrderService) resolveOrderStatus(ctx context.Context, authCtx *auth.AuthContext, order *model.Order) model.OrderStatus {
	if authCtx.IdentityType == auth.IdentityClient {
		return model.OrderStatusApproved
	}

	isSupervisor, err := s.checkSupervisor(ctx)
	if isSupervisor && err == nil {
		return model.OrderStatusApproved
	}

	if authCtx.IdentityType != auth.IdentityEmployee || authCtx.EmployeeID == nil {
		return model.OrderStatusPending
	}

	resp, err := s.userClient.GetEmployeeById(ctx, uint64(*authCtx.EmployeeID))
	if err != nil {
		return model.OrderStatusPending
	}
	if !resp.IsAgent {
		return model.OrderStatusPending
	}
	if resp.NeedApproval {
		return model.OrderStatusPending
	}

	exchange, err := s.exchangeRepo.FindByMicCode(ctx, order.Listing.ExchangeMIC)
	if err != nil {
		return model.OrderStatusPending
	}
	if exchange == nil {
		return model.OrderStatusPending
	}

	orderValue := approximateOrderValue(order, dereferencePrice(order.PricePerUnit))
	orderValueRSD, err := s.bankingClient.ConvertCurrency(ctx, orderValue, exchange.Currency, "RSD")
	if err != nil {
		return model.OrderStatusPending
	}

	if orderValueRSD > resp.OrderLimit-resp.UsedLimit {
		return model.OrderStatusPending
	}

	return model.OrderStatusApproved
}

func (s *OrderService) validateSellOwnership(ctx context.Context, userId uint, ownerType model.OwnerType, assetID uint, quantity float64) error {
	ownerships, err := s.assetOwnershipRepo.FindByUserId(ctx, userId, ownerType)
	if err != nil {
		return errors.InternalErr(err)
	}
	for _, o := range ownerships {
		if o.AssetID == assetID {
			if o.Amount < quantity {
				return errors.BadRequestErr("insufficient asset ownership to sell")
			}
			return nil
		}
	}
	return errors.BadRequestErr("insufficient asset ownership to sell")
}

func (s *OrderService) validateAccount(ctx context.Context, accountNumber string, authCtx *auth.AuthContext) (*pb.GetAccountByNumberResponse, error) {
	account, err := s.bankingClient.GetAccountByNumber(ctx, accountNumber)
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			return nil, errors.NotFoundErr("account not found")
		}
		return nil, errors.ServiceUnavailableErr(err)
	}

	switch authCtx.IdentityType {
	case auth.IdentityClient:
		if authCtx.ClientID == nil || uint64(*authCtx.ClientID) != account.ClientId {
			return nil, errors.ForbiddenErr("account does not belong to you")
		}
	case auth.IdentityEmployee:
		if account.AccountType != "Bank" {
			return nil, errors.BadRequestErr("employees must use a bank account")
		}
	}

	return account, nil
}

func (s *OrderService) validateMarginRequirements(
	ctx context.Context,
	authCtx *auth.AuthContext,
	margin bool,
	listing *model.Listing,
	exchange *model.Exchange,
	account *pb.GetAccountByNumberResponse,
) error {
	if !margin {
		return nil
	}

	if authCtx == nil {
		return errors.UnauthorizedErr("not authenticated")
	}

	if authCtx.IdentityType == auth.IdentityEmployee && !auth.HasPermission(authCtx.Permissions, permission.TradingMargin) {
		return errors.ForbiddenErr("margin trading permission required")
	}

	if authCtx.IdentityType == auth.IdentityClient {
		if authCtx.ClientID == nil {
			return errors.UnauthorizedErr("not authenticated")
		}

		loanResp, err := s.bankingClient.HasActiveLoan(ctx, uint64(*authCtx.ClientID))
		if err != nil {
			return errors.ServiceUnavailableErr(err)
		}

		if !loanResp.GetHasActiveLoan() {
			return errors.ForbiddenErr("active loan required for margin trading")
		}
	}

	initialMarginCost, err := s.initialMarginCostInAccountCurrency(ctx, listing, exchange, account)
	if err != nil {
		return err
	}

	if account.GetAvailableBalance() <= initialMarginCost {
		return errors.ForbiddenErr("insufficient account funds for initial margin cost")
	}

	return nil
}

func (s *OrderService) initialMarginCostInAccountCurrency(
	ctx context.Context,
	listing *model.Listing,
	exchange *model.Exchange,
	account *pb.GetAccountByNumberResponse,
) (float64, error) {
	if listing == nil {
		return 0, errors.BadRequestErr("listing not found")
	}

	if exchange == nil {
		return 0, errors.BadRequestErr("exchange not found")
	}

	if account == nil {
		return 0, errors.BadRequestErr("account not found")
	}

	initialMarginCost := listing.MaintenanceMargin * 1.1
	if initialMarginCost <= 0 {
		return 0, nil
	}

	tradeCurrency := normalizeCurrencyCode(exchange.Currency)
	accountCurrency := normalizeCurrencyCode(account.GetCurrencyCode())
	if tradeCurrency == accountCurrency {
		return initialMarginCost, nil
	}

	converted, err := s.bankingClient.ConvertCurrency(ctx, initialMarginCost, tradeCurrency, accountCurrency)
	if err != nil {
		return 0, errors.ServiceUnavailableErr(err)
	}

	return converted, nil
}

func (s *OrderService) checkSupervisor(ctx context.Context) (bool, error) {
	authCtx := auth.GetAuthFromContext(ctx)
	if authCtx == nil || authCtx.IdentityType != auth.IdentityEmployee || authCtx.EmployeeID == nil {
		return false, nil
	}

	resp, err := s.userClient.GetEmployeeById(ctx, uint64(*authCtx.EmployeeID))
	if err != nil {
		return false, errors.InternalErr(err)
	}

	return resp.IsSupervisor, nil
}

func (s *OrderService) resolveExchangeSession(exchange *model.Exchange) exchangeSession {
	now := s.now()
	if exchange == nil || !exchange.TradingEnabled {
		return exchangeSession{IsOpen: true, LocalNow: now}
	}

	localNow := now.UTC().Add(time.Duration(exchange.TimeZone) * time.Hour)
	openTime, openErr := time.Parse("15:04", exchange.OpenTime)
	closeTime, closeErr := time.Parse("15:04", exchange.CloseTime)
	if openErr != nil || closeErr != nil {
		return exchangeSession{IsOpen: true, LocalNow: localNow}
	}

	openToday := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), openTime.Hour(), openTime.Minute(), 0, 0, localNow.Location())
	closeToday := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), closeTime.Hour(), closeTime.Minute(), 0, 0, localNow.Location())
	nextOpen := nextTradingOpen(openToday)
	lastClose := previousTradingClose(closeToday, localNow)
	isAfterHours := !localNow.Before(lastClose) && localNow.Before(lastClose.Add(afterHoursWindow))

	if isWeekend(localNow) {
		return exchangeSession{IsClosed: true, IsOpen: false, AfterHours: isAfterHours, NextOpen: nextOpen, LocalNow: localNow, CloseTime: closeToday}
	}

	switch {
	case localNow.Before(openToday):
		nextOpen = nextTradingOpen(openToday)
		return exchangeSession{IsClosed: true, IsOpen: false, AfterHours: isAfterHours, NextOpen: nextOpen, LocalNow: localNow, CloseTime: closeToday}
	case localNow.Before(closeToday):
		return exchangeSession{IsOpen: true, LocalNow: localNow, CloseTime: closeToday}
	default:
		nextOpen = nextTradingOpen(openToday.Add(24 * time.Hour))
		return exchangeSession{IsClosed: true, IsOpen: false, AfterHours: isAfterHours, NextOpen: nextOpen, LocalNow: localNow, CloseTime: closeToday}
	}
}

func previousTradingClose(candidate time.Time, localNow time.Time) time.Time {
	if !localNow.After(candidate) {
		candidate = candidate.Add(-24 * time.Hour)
	}

	for isWeekend(candidate) {
		candidate = candidate.Add(-24 * time.Hour)
	}

	return candidate
}

func nextTradingOpen(candidate time.Time) time.Time {
	for isWeekend(candidate) {
		candidate = candidate.Add(24 * time.Hour)
	}

	return candidate
}

func isWeekend(t time.Time) bool {
	return t.Weekday() == time.Saturday || t.Weekday() == time.Sunday
}

func (s *OrderService) initialExecutionTime(session exchangeSession, afterHours bool) time.Time {
	if afterHours {
		return s.now().Add(afterHoursExecutionDelay)
	}

	nextExecutionAt := s.now()
	if session.IsOpen {
		return nextExecutionAt
	}

	nextExecutionAt = session.NextOpen
	return nextExecutionAt
}

func (s *OrderService) nextExecutionAt(ctx context.Context, order *model.Order) time.Time {
	remaining := order.RemainingPortions()
	if remaining == 0 {
		return s.now()
	}

	volume := math.Max(float64(s.resolveDailyVolume(ctx, order.ListingID)), 10)
	maxSeconds := math.Max(1, float64(24*60)/(volume/float64(remaining)))
	waitSeconds := s.rng.Float64() * maxSeconds
	nextExecutionAt := s.now().Add(time.Duration(waitSeconds * float64(time.Second)))
	if order.AfterHours {
		nextExecutionAt = nextExecutionAt.Add(afterHoursExecutionDelay)
	}

	return nextExecutionAt
}

func (s *OrderService) resolveDailyVolume(ctx context.Context, listingID uint) uint {
	dailyInfo, err := s.listingRepo.FindLatestDailyPriceInfo(ctx, listingID)
	if err != nil || dailyInfo == nil || dailyInfo.Volume == 0 {
		return 0
	}

	return dailyInfo.Volume
}

func (s *OrderService) resolveFillQuantity(order *model.Order) uint {
	remaining := order.RemainingPortions()
	if remaining == 0 {
		return 0
	}
	if order.AllOrNone {
		return remaining
	}
	if remaining == 1 {
		return 1
	}

	return uint(s.rng.Intn(int(remaining)) + 1)
}

func (s *OrderService) executeTradeSettlement(ctx context.Context, order *model.Order, tradeCurrency string, amount float64) (*tradeSettlement, error) {
	direction := pb.TradeSettlementDirection_TRADE_SETTLEMENT_DIRECTION_BUY
	if order.Direction == model.OrderDirectionSell {
		direction = pb.TradeSettlementDirection_TRADE_SETTLEMENT_DIRECTION_SELL
	}

	resp, err := s.bankingClient.ExecuteTradeSettlement(ctx,
		order.AccountNumber,
		tradeCurrency,
		direction,
		amount,
	)
	if err != nil {
		return nil, err
	}

	return &tradeSettlement{
		SourceAmount:        resp.GetSourceAmount(),
		SourceCurrency:      resp.GetSourceCurrencyCode(),
		DestinationAmount:   resp.GetDestinationAmount(),
		DestinationCurrency: resp.GetDestinationCurrencyCode(),
	}, nil
}

func (s *OrderService) failOrder(ctx context.Context, order *model.Order, statusValue model.OrderStatus) error {
	order.Status = statusValue
	order.IsDone = true
	order.NextExecutionAt = nil
	order.UpdatedAt = s.now()
	return s.orderRepo.Save(ctx, order)
}

func validateOrderTypeFields(p placeOrderParams) error {
	switch p.OrderType {
	case model.OrderTypeLimit:
		if p.LimitValue == nil {
			return errors.BadRequestErr("limitValue is required for LIMIT orders")
		}
	case model.OrderTypeStop:
		if p.StopValue == nil {
			return errors.BadRequestErr("stopValue is required for STOP orders")
		}
	case model.OrderTypeStopLimit:
		if p.LimitValue == nil {
			return errors.BadRequestErr("limitValue is required for STOP_LIMIT orders")
		}
		if p.StopValue == nil {
			return errors.BadRequestErr("stopValue is required for STOP_LIMIT orders")
		}
	}
	return nil
}

func calculateInitialPricePerUnit(p placeOrderParams, listing *model.Listing) *float64 {
	switch p.OrderType {
	case model.OrderTypeLimit, model.OrderTypeStopLimit:
		return p.LimitValue
	case model.OrderTypeStop:
		return p.StopValue
	case model.OrderTypeMarket:
		var price float64
		if p.Direction == model.OrderDirectionBuy {
			price = listing.Ask
		} else {
			price = listing.Price
		}
		return &price
	default:
		return nil
	}
}

func isStopConditionMet(order *model.Order, listing *model.Listing) bool {
	if order.StopValue == nil {
		return true
	}

	switch order.Direction {
	case model.OrderDirectionBuy:
		return listing.Ask >= *order.StopValue
	case model.OrderDirectionSell:
		return listing.Price <= *order.StopValue
	default:
		return false
	}
}

func resolveExecutionPrice(order *model.Order, listing *model.Listing) (float64, bool) {
	switch order.OrderType {
	case model.OrderTypeMarket, model.OrderTypeStop:
		if order.Direction == model.OrderDirectionBuy {
			return listing.Ask, true
		}
		return listing.Price, true
	case model.OrderTypeLimit:
		return resolveLimitPrice(order.Direction, order.LimitValue, listing)
	case model.OrderTypeStopLimit:
		return resolveLimitPrice(order.Direction, order.LimitValue, listing)
	default:
		return 0, false
	}
}

func resolveLimitPrice(direction model.OrderDirection, limitValue *float64, listing *model.Listing) (float64, bool) {
	if limitValue == nil {
		return 0, false
	}

	switch direction {
	case model.OrderDirectionBuy:
		if listing.Ask > *limitValue {
			return 0, false
		}
		return math.Min(*limitValue, listing.Ask), true
	case model.OrderDirectionSell:
		if listing.Price < *limitValue {
			return 0, false
		}
		return math.Max(*limitValue, listing.Price), true
	default:
		return 0, false
	}
}

func calculateCommission(orderType model.OrderType, orderValue float64) float64 {
	if orderValue <= 0 {
		return 0
	}

	switch orderType {
	case model.OrderTypeMarket, model.OrderTypeStop:
		return math.Min(0.14*orderValue, 7)
	case model.OrderTypeLimit, model.OrderTypeStopLimit:
		return math.Min(0.24*orderValue, 12)
	default:
		return 0
	}
}

func approximateOrderValue(order *model.Order, fallbackPricePerUnit float64) float64 {
	pricePerUnit := dereferencePrice(order.PricePerUnit)
	if pricePerUnit == 0 {
		pricePerUnit = fallbackPricePerUnit
	}

	return float64(order.Quantity) * order.ContractSize * pricePerUnit
}

func dereferencePrice(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func normalizeCurrencyCode(currency string) string {
	return strings.ToUpper(strings.TrimSpace(currency))
}

func (s *OrderService) recordProfitTax(ctx context.Context, order *model.Order, fillQty uint, pricePerUnit float64, tradeCurrency string) error {
	if order.Direction != model.OrderDirectionSell || order.RemainingPortions() != 0 {
		return nil
	}

	ownership, err := s.getOwnershipForOrder(ctx, order)
	if err != nil {
		return err
	}
	if ownership == nil || ownership.AvgBuyPriceRSD <= 0 {
		return nil
	}

	fillAmount := float64(fillQty) * order.ContractSize

	AvgBuyPriceTradeCurrency, err := s.bankingClient.ConvertCurrency(ctx, ownership.AvgBuyPriceRSD, "RSD", tradeCurrency)
	if err != nil {
		return err
	}

	profitInTradeCurrency := (pricePerUnit - AvgBuyPriceTradeCurrency) * fillAmount
	if profitInTradeCurrency <= 0 {
		return nil
	}

	accountCurrency, err := s.bankingClient.GetAccountCurrency(ctx, order.AccountNumber)
	if err != nil {
		return err
	}

	profitInAccountCurrency, err := s.bankingClient.ConvertCurrency(ctx, profitInTradeCurrency, tradeCurrency, accountCurrency)
	if err != nil {
		return err
	}

	var employeeID *uint
	if order.OrderOwnerType == model.OwnerTypeActuary {
		employeeID = &order.OrderOwnerUserID
	}
	return s.taxService.RecordTax(ctx, order.AccountNumber, employeeID, profitInAccountCurrency, accountCurrency)
}

func (s *OrderService) getOwnershipForOrder(ctx context.Context, order *model.Order) (*model.AssetOwnership, error) {
	if order.Listing.Asset == nil {
		return nil, nil
	}

	ownerID, ownerType := assetOwner(order)
	existing, err := s.assetOwnershipRepo.FindByUserId(ctx, ownerID, ownerType)
	if err != nil {
		return nil, err
	}

	for i := range existing {
		if existing[i].AssetID == order.Listing.AssetID {
			return &existing[i], nil
		}
	}
	return nil, nil
}

func (s *OrderService) validateSettlementDate(ctx context.Context, listing *model.Listing) error {
	if listing.Asset == nil {
		return nil
	}

	now := s.now()

	switch listing.Asset.AssetType {
	case model.AssetTypeFuture:
		contracts, err := s.futuresRepo.FindByAssetIDs(ctx, []uint{listing.AssetID})
		if err != nil {
			return errors.InternalErr(err)
		}
		if len(contracts) > 0 && !contracts[0].SettlementDate.After(now) {
			return errors.BadRequestErr("cannot place order on an expired futures contract")
		}

	case model.AssetTypeOption:
		options, err := s.optionRepo.FindByAssetIDs(ctx, []uint{listing.AssetID})
		if err != nil {
			return errors.InternalErr(err)
		}
		if len(options) > 0 && !options[0].SettlementDate.After(now) {
			return errors.BadRequestErr("cannot place order on an expired option")
		}
	}

	return nil
}

func (s *OrderService) GetMyOrders(ctx context.Context, query dto.UserOrdersQuery, userID uint, ownerType model.OwnerType) ([]dto.UserOrderResponse, int64, error) {
	orders, total, err := s.orderRepo.FindUserOrders(ctx, userID, ownerType, query)
	if err != nil {
		return nil, 0, errors.InternalErr(err)
	}

	responses := make([]dto.UserOrderResponse, len(orders))
	for i, o := range orders {

		var execDate *time.Time
		if o.IsDone {
			execDate = &o.UpdatedAt
		}

		assetType := model.AssetType("")
		if o.Listing.Asset != nil {
			assetType = o.Listing.Asset.AssetType
		}
		responses[i] = dto.UserOrderResponse{
			OrderID:   o.OrderID,
			OrderType: o.OrderType,
			Ticker: func() string {
				if o.Listing.Asset != nil {
					return o.Listing.Asset.Ticker
				}
				return ""
			}(),
			ListingName: func() string {
				if o.Listing.Asset != nil {
					return o.Listing.Asset.Name
				}
				return ""
			}(),
			Quantity:          o.Quantity,
			PricePerUnit:      o.PricePerUnit,
			Status:            o.Status,
			CreatedAt:         o.CreatedAt,
			ReviewedAt:        o.ReviewedAt,
			ExecutionDate:     execDate,
			CommissionCharged: o.CommissionCharged,
			AssetType:         assetType,
		}
	}
	return responses, total, nil
}

type SystemOrderParams struct {
	AccountNumber string
	ListingID     uint
	Direction     model.OrderDirection
	Quantity      uint
	OwnerUserID   uint
	OwnerType     model.OwnerType
}

func (s *OrderService) CreateSystemOrder(ctx context.Context, p SystemOrderParams) (*model.Order, error) {
	account, err := s.bankingClient.GetAccountByNumber(ctx, p.AccountNumber)
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			return nil, errors.NotFoundErr("account not found")
		}
		return nil, errors.ServiceUnavailableErr(err)
	}

	ownerID := p.OwnerUserID
	var sysCtx context.Context
	if p.OwnerType == model.OwnerTypeClient {
		sysCtx = auth.SetAuthOnContext(ctx, &auth.AuthContext{
			IdentityID:   ownerID,
			IdentityType: auth.IdentityClient,
			ClientID:     &ownerID,
		})
	} else {
		sysCtx = auth.SetAuthOnContext(ctx, &auth.AuthContext{
			IdentityID:   ownerID,
			IdentityType: auth.IdentityEmployee,
			EmployeeID:   &ownerID,
		})
	}
	authCtx := auth.GetAuthFromContext(sysCtx)

	return s.placeOrder(sysCtx, authCtx, placeOrderParams{
		AccountNumber:    p.AccountNumber,
		ListingID:        p.ListingID,
		OrderType:        model.OrderTypeMarket,
		Direction:        p.Direction,
		Quantity:         p.Quantity,
		AllOrNone:        false,
		Margin:           false,
		OrderOwnerUserID: p.OwnerUserID,
		OrderOwnerType:   p.OwnerType,
		AssetOwnerUserID: p.OwnerUserID,
		AssetOwnerType:   p.OwnerType,
		CommissionExempt: p.OwnerType == model.OwnerTypeActuary,
		account:          account,
	})
}
