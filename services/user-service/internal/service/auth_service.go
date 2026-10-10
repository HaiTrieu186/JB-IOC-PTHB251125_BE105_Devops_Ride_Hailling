package service

import (
	"context"

	"user-service/internal/dto"
	"user-service/internal/exception"
)

type AuthService interface {
	Register(ctx context.Context, req *dto.RegisterRequest) (*dto.RegisterResponse, *exception.AppError)
	Login(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, *exception.AppError)
	Refresh(ctx context.Context, req *dto.RefreshRequest) (*dto.RefreshResponse, *exception.AppError)
	Logout(ctx context.Context, req *dto.LogoutRequest, userID string, userRole string, authHeader string) *exception.AppError
	SeedAdmin(ctx context.Context) error
}
