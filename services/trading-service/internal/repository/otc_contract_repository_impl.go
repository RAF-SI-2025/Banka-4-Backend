package repository

import (
	"context"
	"errors"

	commondb "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/db"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type otcContractRepository struct {
	db *gorm.DB
}

func NewOtcContractRepository(db *gorm.DB) OtcContractRepository {
	return &otcContractRepository{db: db}
}

func (r *otcContractRepository) Create(ctx context.Context, contract *model.OtcContract) error {
	return commondb.DBFromContext(ctx, r.db).Create(contract).Error
}

func (r *otcContractRepository) FindByID(ctx context.Context, contractID uint) (*model.OtcContract, error) {
	return r.find(ctx, contractID, false)
}

func (r *otcContractRepository) FindByIDForUpdate(ctx context.Context, contractID uint) (*model.OtcContract, error) {
	return r.find(ctx, contractID, true)
}

func (r *otcContractRepository) Save(ctx context.Context, contract *model.OtcContract) error {
	return commondb.DBFromContext(ctx, r.db).Save(contract).Error
}

func (r *otcContractRepository) find(ctx context.Context, contractID uint, forUpdate bool) (*model.OtcContract, error) {
	query := commondb.DBFromContext(ctx, r.db).Preload("Asset")
	if forUpdate {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}

	var contract model.OtcContract
	if err := query.First(&contract, contractID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &contract, nil
}
