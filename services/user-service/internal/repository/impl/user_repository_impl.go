package impl

import (
	"context"

	"user-service/internal/entity"
	"user-service/internal/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type userRepositoryImpl struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) repository.UserRepository {
	return &userRepositoryImpl{db: db}
}

func (r *userRepositoryImpl) FindByPhoneOrEmail(ctx context.Context, phoneOrEmail string) (*entity.User, error) {
	var user entity.User
	err := r.db.WithContext(ctx).
		Where("phone_number = ? OR email = ?", phoneOrEmail, phoneOrEmail).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepositoryImpl) FindByPhoneOrEmailDual(ctx context.Context, phone string, email *string) (*entity.User, error) {
	var user entity.User
	q := r.db.WithContext(ctx).Where("phone_number = ?", phone)
	if email != nil {
		q = r.db.WithContext(ctx).Where("phone_number = ? OR email = ?", phone, *email)
	}
	err := q.First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepositoryImpl) FindByID(ctx context.Context, id uuid.UUID) (*entity.User, error) {
	var user entity.User
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepositoryImpl) Create(ctx context.Context, user *entity.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepositoryImpl) UpdateDriverStatusConditional(ctx context.Context, driverID uuid.UUID, newStatus string) (int64, error) {
	res := r.db.WithContext(ctx).Exec(
		"UPDATE users SET driver_status = ?, updated_at = now() WHERE id = ? AND role = 'DRIVER' AND driver_status != 'BUSY'",
		newStatus, driverID,
	)
	return res.RowsAffected, res.Error
}

func (r *userRepositoryImpl) UpdateDriverStatusInternalConditional(ctx context.Context, driverID uuid.UUID, fromStatus string, toStatus string) (int64, error) {
	res := r.db.WithContext(ctx).Exec(
		"UPDATE users SET driver_status = ?, updated_at = now() WHERE id = ? AND role = 'DRIVER' AND driver_status = ?",
		toStatus, driverID, fromStatus,
	)
	return res.RowsAffected, res.Error
}

func (r *userRepositoryImpl) FindOnlineDriverIDs(ctx context.Context, driverIDs []uuid.UUID) ([]string, error) {
	var onlineIDs []string
	err := r.db.WithContext(ctx).Model(&entity.User{}).
		Where("id IN ? AND role = 'DRIVER' AND driver_status = 'ONLINE'", driverIDs).
		Pluck("id", &onlineIDs).Error
	if err != nil {
		return nil, err
	}
	if onlineIDs == nil {
		onlineIDs = []string{}
	}
	return onlineIDs, nil
}

func (r *userRepositoryImpl) CountByEmail(ctx context.Context, email string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&entity.User{}).Where("email = ?", email).Count(&count).Error
	return count, err
}
