package impl

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"payment-service/internal/entity"
	"payment-service/internal/repository"
)

type walletRepositoryImpl struct {
	db *gorm.DB
}

func NewWalletRepository(db *gorm.DB) repository.WalletRepository {
	return &walletRepositoryImpl{db: db}
}

func (r *walletRepositoryImpl) EnsureWallet(ctx context.Context, userID uuid.UUID) (*entity.Wallet, error) {
	// Atomic lazy wallet creation
	w := entity.Wallet{
		UserID:    userID,
		Balance:   0,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoNothing: true,
		}).
		Create(&w).Error
	if err != nil {
		return nil, err
	}

	var found entity.Wallet
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&found).Error; err != nil {
		return nil, err
	}

	return &found, nil
}

func (r *walletRepositoryImpl) GetByUserID(ctx context.Context, userID uuid.UUID) (*entity.Wallet, error) {
	var w entity.Wallet
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&w).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &w, nil
}

func (r *walletRepositoryImpl) TopUp(ctx context.Context, userID uuid.UUID, amount int64) (*entity.Wallet, *entity.Transaction, error) {
	var wallet entity.Wallet
	var txRec entity.Transaction

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. Ensure wallet exists
		w := entity.Wallet{
			UserID:    userID,
			Balance:   0,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoNothing: true,
		}).Create(&w).Error; err != nil {
			return err
		}

		// 2. Lock & update wallet balance
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", userID).
			First(&wallet).Error; err != nil {
			return err
		}

		wallet.Balance += amount
		wallet.UpdatedAt = time.Now().UTC()
		if err := tx.Save(&wallet).Error; err != nil {
			return err
		}

		// 3. Insert transaction log
		txRec = entity.Transaction{
			WalletID:  &wallet.ID,
			UserID:    &userID,
			TripID:    nil,
			Type:      entity.TxTypeTopup,
			Amount:    amount,
			Status:    entity.TxStatusSuccess,
			CreatedAt: time.Now().UTC(),
		}
		if err := tx.Create(&txRec).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, nil, err
	}

	return &wallet, &txRec, nil
}
