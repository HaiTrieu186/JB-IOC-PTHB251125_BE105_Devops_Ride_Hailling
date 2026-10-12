package repository

import (
	"context"

	"github.com/google/uuid"
	"payment-service/internal/entity"
)

type WalletRepository interface {
	EnsureWallet(ctx context.Context, userID uuid.UUID) (*entity.Wallet, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) (*entity.Wallet, error)
	TopUp(ctx context.Context, userID uuid.UUID, amount int64) (*entity.Wallet, *entity.Transaction, error)
}
