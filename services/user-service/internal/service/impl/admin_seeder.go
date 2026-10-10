package impl

import (
	"context"
	"log/slog"
	"time"

	"user-service/internal/entity"
	"user-service/internal/util"

	"github.com/google/uuid"
)

func (s *authServiceImpl) SeedAdmin(ctx context.Context) error {
	if s.cfg.AdminEmail == "" || s.cfg.AdminPassword == "" {
		return nil
	}

	count, err := s.userRepo.CountByEmail(ctx, s.cfg.AdminEmail)
	if err != nil {
		return err
	}
	if count > 0 {
		slog.Info("admin account already exists, skipping seed", "email", s.cfg.AdminEmail)
		return nil
	}

	hash, err := util.HashPassword(s.cfg.AdminPassword)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	admin := entity.User{
		ID:           uuid.New(),
		PhoneNumber:  nil,
		Email:        &s.cfg.AdminEmail,
		PasswordHash: hash,
		FullName:     "Administrator",
		Role:         "ADMIN",
		DriverStatus: nil,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.userRepo.Create(ctx, &admin); err != nil {
		return err
	}

	slog.Info("seeded admin account successfully", "email", s.cfg.AdminEmail, "user_id", admin.ID)
	return nil
}
