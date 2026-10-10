package repository

import (
	"context"

	"user-service/internal/entity"

	"github.com/google/uuid"
)

type UserRepository interface {
	FindByPhoneOrEmail(ctx context.Context, phoneOrEmail string) (*entity.User, error)
	FindByPhoneOrEmailDual(ctx context.Context, phone string, email *string) (*entity.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*entity.User, error)
	Create(ctx context.Context, user *entity.User) error
	UpdateDriverStatusConditional(ctx context.Context, driverID uuid.UUID, newStatus string) (int64, error)
	UpdateDriverStatusInternalConditional(ctx context.Context, driverID uuid.UUID, fromStatus string, toStatus string) (int64, error)
	FindOnlineDriverIDs(ctx context.Context, driverIDs []uuid.UUID) ([]string, error)
	CountByEmail(ctx context.Context, email string) (int64, error)
}
