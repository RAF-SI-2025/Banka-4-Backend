package repository

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type investmentFundRepository struct {
	db *gorm.DB
}

func NewInvestmentFundRepository(db *gorm.DB) InvestmentFundRepository {
	return &investmentFundRepository{db: db}
}

func (r *investmentFundRepository) Create(ctx context.Context, fund *model.InvestmentFund) error {
	return r.db.WithContext(ctx).Create(fund).Error
}

func (r *investmentFundRepository) FindByID(ctx context.Context, id uint) (*model.InvestmentFund, error) {
	var fund model.InvestmentFund
	result := r.db.WithContext(ctx).First(&fund, id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &fund, result.Error
}

func (r *investmentFundRepository) FindByAccountNumber(ctx context.Context, accountNumber string) (*model.InvestmentFund, error) {
	var fund model.InvestmentFund
	result := r.db.WithContext(ctx).Where("account_number = ?", accountNumber).First(&fund)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &fund, result.Error
}

func (r *investmentFundRepository) FindByName(ctx context.Context, name string) (*model.InvestmentFund, error) {
	var fund model.InvestmentFund
	result := r.db.WithContext(ctx).Where("name = ?", name).First(&fund)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &fund, result.Error
}

func (r *investmentFundRepository) GetAllInvestmentFunds(ctx context.Context) ([]model.InvestmentFund, error) {
	var funds []model.InvestmentFund

	err := r.db.WithContext(ctx).
		Preload("Positions").
		Find(&funds).Error

	return funds, err
}

func (r *investmentFundRepository) FindAll(ctx context.Context, name string, sortBy string, sortDir string, page int, pageSize int) ([]model.InvestmentFund, int64, error) {
	var funds []model.InvestmentFund
	var count int64

	db := r.db.WithContext(ctx).Model(&model.InvestmentFund{})

	if name != "" {
		db = db.Where("LOWER(name) LIKE ?", "%"+strings.ToLower(name)+"%")
	}

	if err := db.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	allowedSortFields := map[string]string{
		"name":                 "name",
		"minimum_contribution": "minimum_contribution",
		"created_at":           "created_at",
		"liquid_assets":        "liquid_assets",
	}
	dbField, ok := allowedSortFields[strings.ToLower(sortBy)]
	if !ok {
		dbField = "name"
	}

	dir := "ASC"
	if strings.ToLower(sortDir) == "desc" {
		dir = "DESC"
	}

	offset := (page - 1) * pageSize
	err := db.Preload("Positions").
		Order(dbField + " " + dir).
		Limit(pageSize).
		Offset(offset).
		Find(&funds).Error
	return funds, count, err
}

func (r *investmentFundRepository) FindByManagerID(ctx context.Context, managerID uint) ([]model.InvestmentFund, error) {
	var funds []model.InvestmentFund
	err := r.db.WithContext(ctx).
		Where("manager_id = ?", managerID).
		Preload("Positions").
		Find(&funds).Error
	return funds, err
}
