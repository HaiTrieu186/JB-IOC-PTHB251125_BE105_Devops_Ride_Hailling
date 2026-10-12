package impl

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"payment-service/internal/entity"
	"payment-service/internal/repository"
)

type transactionRepositoryImpl struct {
	db *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) repository.TransactionRepository {
	return &transactionRepositoryImpl{db: db}
}

func (r *transactionRepositoryImpl) FindByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]entity.Transaction, int64, error) {
	var total int64
	var txs []entity.Transaction

	// Filtering by user_id automatically excludes COMMISSION entries which have user_id IS NULL
	baseQuery := r.db.WithContext(ctx).Model(&entity.Transaction{}).Where("user_id = ?", userID)

	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := baseQuery.Order("created_at DESC").Limit(limit).Offset(offset).Find(&txs).Error; err != nil {
		return nil, 0, err
	}

	return txs, total, nil
}

func (r *transactionRepositoryImpl) ExistsByTripID(ctx context.Context, tripID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&entity.Transaction{}).Where("trip_id = ?", tripID).Limit(1).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
