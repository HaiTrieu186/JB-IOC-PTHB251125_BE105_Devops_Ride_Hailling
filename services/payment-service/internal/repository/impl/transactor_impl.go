package impl

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"payment-service/internal/entity"
	"payment-service/internal/repository"
)

type transactorImpl struct {
	db *gorm.DB
}

func NewTransactor(db *gorm.DB) repository.Transactor {
	return &transactorImpl{db: db}
}

func (t *transactorImpl) ExecuteSettlement(ctx context.Context, params repository.SettlementParams) (*repository.SettlementResult, error) {
	result := &repository.SettlementResult{}

	err := t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()

		// 1. Ensure wallets exist for customer and driver
		cw := entity.Wallet{
			UserID:    params.CustomerID,
			Balance:   0,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoNothing: true,
		}).Create(&cw).Error; err != nil {
			return err
		}

		dw := entity.Wallet{
			UserID:    params.DriverID,
			Balance:   0,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoNothing: true,
		}).Create(&dw).Error; err != nil {
			return err
		}

		// 2. Lock 2 wallets in fixed UUID order to prevent deadlock
		firstID := params.CustomerID
		secondID := params.DriverID
		if params.CustomerID.String() > params.DriverID.String() {
			firstID = params.DriverID
			secondID = params.CustomerID
		}

		var w1, w2 entity.Wallet
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", firstID).First(&w1).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", secondID).First(&w2).Error; err != nil {
			return err
		}

		var customerWallet, driverWallet *entity.Wallet
		if w1.UserID == params.CustomerID {
			customerWallet = &w1
			driverWallet = &w2
		} else {
			customerWallet = &w2
			driverWallet = &w1
		}

		// 3. Balance verification and branch execution
		if customerWallet.Balance >= params.Fare {
			// Case sufficient funds: deduct customer, add driver income, insert 3 ledger rows
			customerWallet.Balance -= params.Fare
			customerWallet.UpdatedAt = now
			if err := tx.Save(customerWallet).Error; err != nil {
				return err
			}

			driverWallet.Balance += params.DriverIncome
			driverWallet.UpdatedAt = now
			if err := tx.Save(driverWallet).Error; err != nil {
				return err
			}

			txPayment := entity.Transaction{
				WalletID:  &customerWallet.ID,
				UserID:    &params.CustomerID,
				TripID:    &params.TripID,
				Type:      entity.TxTypeTripPayment,
				Amount:    -params.Fare,
				Status:    entity.TxStatusSuccess,
				CreatedAt: now,
			}
			txIncome := entity.Transaction{
				WalletID:  &driverWallet.ID,
				UserID:    &params.DriverID,
				TripID:    &params.TripID,
				Type:      entity.TxTypeTripIncome,
				Amount:    params.DriverIncome,
				Status:    entity.TxStatusSuccess,
				CreatedAt: now,
			}
			txCommission := entity.Transaction{
				WalletID:  nil,
				UserID:    nil,
				TripID:    &params.TripID,
				Type:      entity.TxTypeCommission,
				Amount:    params.Commission,
				Status:    entity.TxStatusSuccess,
				CreatedAt: now,
			}

			if err := tx.Create(&txPayment).Error; err != nil {
				return err
			}
			if err := tx.Create(&txIncome).Error; err != nil {
				return err
			}
			if err := tx.Create(&txCommission).Error; err != nil {
				return err
			}

			result.Success = true
			return nil
		}

		// Case insufficient funds: Do NOT deduct negative, do NOT credit driver
		// Record 1 FAILED transaction row
		note := "INSUFFICIENT_FUNDS_ON_COMPLETE"
		txFailed := entity.Transaction{
			WalletID:  &customerWallet.ID,
			UserID:    &params.CustomerID,
			TripID:    &params.TripID,
			Type:      entity.TxTypeTripPayment,
			Amount:    -params.Fare,
			Status:    entity.TxStatusFailed,
			Note:      &note,
			CreatedAt: now,
		}
		if err := tx.Create(&txFailed).Error; err != nil {
			return err
		}

		result.InsufficientFund = true
		return nil
	})

	if err != nil {
		// Unique constraint violation (SQLSTATE 23505) -> treat as duplicate idempotent
		if strings.Contains(err.Error(), "23505") ||
			strings.Contains(err.Error(), "duplicate key") ||
			strings.Contains(err.Error(), "idx_transactions_trip_type") {
			return &repository.SettlementResult{Duplicate: true}, nil
		}
		return nil, err
	}

	return result, nil
}
