package repository

import (
	"context"

	"github.com/google/uuid"
	"payment-service/internal/entity"
)

type TransactionRepository interface {
	FindByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]entity.Transaction, int64, error)
	ExistsByTripID(ctx context.Context, tripID uuid.UUID) (bool, error)
}
