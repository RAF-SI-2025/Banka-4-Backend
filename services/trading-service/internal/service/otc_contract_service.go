package service

import (
	"context"
	"fmt"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
)

type OtcContractService struct {
	otcContractRepo    repository.OtcContractRepository
	assetOwnershipRepo repository.AssetOwnershipRepository
}

func NewOtcContractService(
	otcContractRepo repository.OtcContractRepository,
	assetOwnershipRepo repository.AssetOwnershipRepository,
) *OtcContractService {
	return &OtcContractService{
		otcContractRepo:    otcContractRepo,
		assetOwnershipRepo: assetOwnershipRepo,
	}
}

// CreateContract kreira novu OTC ponudu. Kupac se automatski smatra da je prihvatio
// slanjem ponude.
func (s *OtcContractService) CreateContract(ctx context.Context, req dto.CreateOtcContractRequest) (*model.OtcContract, error) {
	authCtx := auth.GetAuthFromContext(ctx)
	if authCtx == nil {
		return nil, errors.UnauthorizedErr("nije autentifikovan")
	}

	buyerID, err := auth.GetSubjectFromContext(ctx)
	if err != nil {
		return nil, err
	}

	ownerships, err := s.assetOwnershipRepo.FindByIdentity(ctx, req.SellerID, model.OwnerTypeClient)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	var sellerOwnership *model.AssetOwnership
	for i := range ownerships {
		if ownerships[i].AssetID == req.AssetID {
			sellerOwnership = &ownerships[i]
			break
		}
	}

	if sellerOwnership == nil || sellerOwnership.PublicAmount < req.Quantity {
		return nil, errors.BadRequestErr("prodavac nema dovoljno javnih akcija")
	}

	totalPrice := req.Quantity * req.PricePerUnit
	contractNumber := fmt.Sprintf("%d/%d", time.Now().UnixNano(), req.AssetID)

	contract := &model.OtcContract{
		BuyerID:        buyerID,
		SellerID:       req.SellerID,
		AssetID:        req.AssetID,
		Quantity:       req.Quantity,
		PricePerUnit:   req.PricePerUnit,
		TotalPrice:     totalPrice,
		ContractNumber: contractNumber,
	}

	if err := s.otcContractRepo.Create(ctx, contract); err != nil {
		return nil, errors.InternalErr(err)
	}

	return contract, nil
}

// AcceptContract poziva prodavac kada prihvata ponudu.
func (s *OtcContractService) AcceptContract(ctx context.Context, contractID uint, sellerID uint) (*model.OtcContract, error) {
	contract, err := s.otcContractRepo.FindByID(ctx, contractID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if contract == nil {
		return nil, errors.NotFoundErr("ugovor nije pronadjen")
	}
	if contract.SellerID != sellerID {
		return nil, errors.ForbiddenErr("nemate pravo da prihvatite ovaj ugovor")
	}
	if contract.SellerApproved != nil {
		return nil, errors.BadRequestErr("ugovor je vec obradjeni")
	}

	approved := true
	contract.SellerApproved = &approved

	if contract.BankApproved != nil && *contract.BankApproved {
		now := time.Now()
		contract.FinalizedAt = &now
	}

	if err := s.otcContractRepo.Save(ctx, contract); err != nil {
		return nil, errors.InternalErr(err)
	}

	return contract, nil
}

// RejectContract poziva prodavac kada odbija ponudu uz komentar razloga.
func (s *OtcContractService) RejectContract(ctx context.Context, contractID uint, sellerID uint, req dto.RejectOtcContractRequest) (*model.OtcContract, error) {
	contract, err := s.otcContractRepo.FindByID(ctx, contractID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if contract == nil {
		return nil, errors.NotFoundErr("ugovor nije pronadjen")
	}
	if contract.SellerID != sellerID {
		return nil, errors.ForbiddenErr("nemate pravo da odbijete ovaj ugovor")
	}
	if contract.SellerApproved != nil {
		return nil, errors.BadRequestErr("ugovor je vec obradjeni")
	}

	rejected := false
	contract.SellerApproved = &rejected
	contract.Comment = &req.Comment

	if err := s.otcContractRepo.Save(ctx, contract); err != nil {
		return nil, errors.InternalErr(err)
	}

	return contract, nil
}

// ApproveBankContract poziva supervizor banke kada odobrava ugovor.
func (s *OtcContractService) ApproveBankContract(ctx context.Context, contractID uint) (*model.OtcContract, error) {
	contract, err := s.otcContractRepo.FindByID(ctx, contractID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if contract == nil {
		return nil, errors.NotFoundErr("ugovor nije pronadjen")
	}
	if contract.BankApproved != nil {
		return nil, errors.BadRequestErr("ugovor je vec obradjeni")
	}

	approved := true
	contract.BankApproved = &approved

	if contract.SellerApproved != nil && *contract.SellerApproved {
		// TODO: finalizacija — prenos novca i akcija (ostaviti za zasebni task)
		now := time.Now()
		contract.FinalizedAt = &now
	}

	if err := s.otcContractRepo.Save(ctx, contract); err != nil {
		return nil, errors.InternalErr(err)
	}
	return contract, nil
}

// RejectBankContract poziva supervizor banke kada odbija ugovor.
func (s *OtcContractService) RejectBankContract(ctx context.Context, contractID uint, req dto.RejectOtcContractRequest) (*model.OtcContract, error) {
	contract, err := s.otcContractRepo.FindByID(ctx, contractID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if contract == nil {
		return nil, errors.NotFoundErr("ugovor nije pronadjen")
	}
	if contract.BankApproved != nil {
		return nil, errors.BadRequestErr("ugovor je vec obradjeni")
	}

	rejected := false
	contract.BankApproved = &rejected
	contract.Comment = &req.Comment

	if err := s.otcContractRepo.Save(ctx, contract); err != nil {
		return nil, errors.InternalErr(err)
	}
	return contract, nil
}

// GetContractsForBuyer vraca sve ugovore u kojima je dati korisnik kupac.
func (s *OtcContractService) GetContractsForBuyer(ctx context.Context, buyerID uint) ([]model.OtcContract, error) {
	contracts, err := s.otcContractRepo.FindByBuyerID(ctx, buyerID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	return contracts, nil
}

// GetContractsForSeller vraca sve ugovore u kojima je dati korisnik prodavac.
func (s *OtcContractService) GetContractsForSeller(ctx context.Context, sellerID uint) ([]model.OtcContract, error) {
	contracts, err := s.otcContractRepo.FindBySellerID(ctx, sellerID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	return contracts, nil
}

// GetPendingBankApproval vraca sve ugovore koji cekaju odobrenje supervizora banke.
func (s *OtcContractService) GetPendingBankApproval(ctx context.Context) ([]model.OtcContract, error) {
	contracts, err := s.otcContractRepo.FindPendingBankApproval(ctx)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	return contracts, nil
}

