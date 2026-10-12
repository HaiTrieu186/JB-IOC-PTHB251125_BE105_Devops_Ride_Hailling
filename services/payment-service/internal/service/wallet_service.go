package service

import (
	"context"

	"github.com/google/uuid"
	"payment-service/internal/dto"
	"payment-service/internal/exception"
)

type WalletService interface {
	TopUp(ctx context.Context, userID uuid.UUID, amount int64) (*dto.TopUpResponse, *exception.AppError)
	GetMe(ctx context.Context, userID uuid.UUID) (*dto.WalletMeResponse, *exception.AppError)
	GetTransactions(ctx context.Context, userID uuid.UUID, page, limit int) (*dto.TransactionListResponse, *exception.AppError)
	CheckBalance(ctx context.Context, req dto.CheckBalanceRequest) (*dto.CheckBalanceResponse, *exception.AppError)
}
