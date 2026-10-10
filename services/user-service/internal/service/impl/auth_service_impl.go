package impl

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"user-service/internal/config"
	"user-service/internal/dto"
	"user-service/internal/entity"
	"user-service/internal/exception"
	"user-service/internal/repository"
	"user-service/internal/service"
	"user-service/internal/util"

	"github.com/google/uuid"
)

type authServiceImpl struct {
	userRepo    repository.UserRepository
	tokenRepo   repository.TokenRepository
	wsPublisher repository.WSControlPublisher
	cfg         *config.Config
}

func NewAuthService(
	userRepo repository.UserRepository,
	tokenRepo repository.TokenRepository,
	wsPublisher repository.WSControlPublisher,
	cfg *config.Config,
) service.AuthService {
	return &authServiceImpl{
		userRepo:    userRepo,
		tokenRepo:   tokenRepo,
		wsPublisher: wsPublisher,
		cfg:         cfg,
	}
}

func (s *authServiceImpl) Register(ctx context.Context, req *dto.RegisterRequest) (*dto.RegisterResponse, *exception.AppError) {
	req.PhoneNumber = strings.TrimSpace(req.PhoneNumber)
	req.FullName = strings.TrimSpace(req.FullName)
	req.Role = strings.TrimSpace(req.Role)

	if req.PhoneNumber == "" {
		return nil, exception.NewValidationError("Số điện thoại không được để trống")
	}
	if req.FullName == "" {
		return nil, exception.NewValidationError("Họ và tên không được để trống")
	}
	if len(req.Password) < 6 {
		return nil, exception.NewValidationError("Mật khẩu phải từ 6 ký tự trở lên")
	}
	if req.Role != "CUSTOMER" && req.Role != "DRIVER" {
		return nil, exception.NewValidationError("Vai trò phải là CUSTOMER hoặc DRIVER")
	}

	var cleanEmail *string
	if req.Email != nil {
		em := strings.TrimSpace(*req.Email)
		if em != "" {
			if !strings.Contains(em, "@") || !strings.Contains(em, ".") {
				return nil, exception.NewValidationError("Email không đúng định dạng")
			}
			cleanEmail = &em
		}
	}

	existing, err := s.userRepo.FindByPhoneOrEmailDual(ctx, req.PhoneNumber, cleanEmail)
	if err == nil && existing != nil {
		return nil, exception.NewAppError(exception.ErrUserAlreadyExists, 409, "Số điện thoại hoặc email đã tồn tại trong hệ thống")
	}

	hashedPassword, err := util.HashPassword(req.Password)
	if err != nil {
		slog.Error("failed to hash password", "error", err)
		return nil, exception.NewAppError(exception.ErrInternal, 500, "Lỗi mã hóa mật khẩu")
	}

	var driverStatus *string
	if req.Role == "DRIVER" {
		status := "OFFLINE"
		driverStatus = &status
	}

	now := time.Now().UTC()
	newUser := entity.User{
		ID:           uuid.New(),
		PhoneNumber:  &req.PhoneNumber,
		Email:        cleanEmail,
		PasswordHash: hashedPassword,
		FullName:     req.FullName,
		Role:         req.Role,
		DriverStatus: driverStatus,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.userRepo.Create(ctx, &newUser); err != nil {
		return nil, exception.NewAppError(exception.ErrUserAlreadyExists, 409, "Số điện thoại hoặc email đã tồn tại trong hệ thống")
	}

	return &dto.RegisterResponse{
		UserID:    newUser.ID.String(),
		Role:      newUser.Role,
		FullName:  newUser.FullName,
		CreatedAt: newUser.CreatedAt.Format(time.RFC3339),
	}, nil
}

func (s *authServiceImpl) Login(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, *exception.AppError) {
	req.PhoneOrEmail = strings.TrimSpace(req.PhoneOrEmail)
	if req.PhoneOrEmail == "" || req.Password == "" {
		return nil, exception.NewValidationError("Vui lòng nhập đầy đủ tài khoản và mật khẩu")
	}

	user, err := s.userRepo.FindByPhoneOrEmail(ctx, req.PhoneOrEmail)
	if err != nil || user == nil {
		util.CheckDummyPassword(req.Password)
		return nil, exception.NewUnauthorizedError("Số điện thoại/email hoặc mật khẩu không chính xác")
	}

	if !util.CheckPassword(user.PasswordHash, req.Password) {
		return nil, exception.NewUnauthorizedError("Số điện thoại/email hoặc mật khẩu không chính xác")
	}

	accessToken, _, _, err := util.GenerateToken(user.ID.String(), user.Role, s.cfg.AccessTokenExpiry, s.cfg.JWTSecret)
	if err != nil {
		slog.Error("failed to generate access token", "error", err)
		return nil, exception.NewAppError(exception.ErrInternal, 500, "Lỗi tạo token")
	}

	refreshToken := uuid.New().String()
	ttl := time.Duration(s.cfg.RefreshTokenExpiry) * time.Second
	if err := s.tokenRepo.SaveRefreshToken(ctx, refreshToken, user.ID.String(), ttl); err != nil {
		slog.Error("failed to save refresh token", "error", err)
		return nil, exception.NewServiceUnavailableError("Lỗi lưu trữ phiên")
	}

	return &dto.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    s.cfg.AccessTokenExpiry,
		User: dto.UserInfo{
			ID:           user.ID.String(),
			Role:         user.Role,
			FullName:     user.FullName,
			DriverStatus: user.DriverStatus,
		},
	}, nil
}

func (s *authServiceImpl) Refresh(ctx context.Context, req *dto.RefreshRequest) (*dto.RefreshResponse, *exception.AppError) {
	req.RefreshToken = strings.TrimSpace(req.RefreshToken)
	if req.RefreshToken == "" {
		return nil, exception.NewValidationError("Thiếu refresh_token")
	}

	userIDStr, err := s.tokenRepo.GetDelRefreshToken(ctx, req.RefreshToken)
	if err != nil || userIDStr == "" {
		return nil, exception.NewAppError(exception.ErrInvalidRefreshToken, 401, "Refresh token không tồn tại hoặc đã được sử dụng")
	}

	userUUID, err := util.ParseUUID(userIDStr)
	if err != nil {
		return nil, exception.NewAppError(exception.ErrInvalidRefreshToken, 401, "Refresh token không tồn tại hoặc đã được sử dụng")
	}

	user, err := s.userRepo.FindByID(ctx, userUUID)
	if err != nil || user == nil {
		return nil, exception.NewAppError(exception.ErrInvalidRefreshToken, 401, "Refresh token không tồn tại hoặc đã được sử dụng")
	}

	newAccessToken, _, _, err := util.GenerateToken(user.ID.String(), user.Role, s.cfg.AccessTokenExpiry, s.cfg.JWTSecret)
	if err != nil {
		slog.Error("failed to generate new access token", "error", err)
		return nil, exception.NewAppError(exception.ErrInternal, 500, "Lỗi tạo token")
	}

	newRefreshToken := uuid.New().String()
	ttl := time.Duration(s.cfg.RefreshTokenExpiry) * time.Second
	if err := s.tokenRepo.SaveRefreshToken(ctx, newRefreshToken, user.ID.String(), ttl); err != nil {
		slog.Error("failed to save new refresh token", "error", err)
		return nil, exception.NewServiceUnavailableError("Lỗi lưu trữ phiên")
	}

	return &dto.RefreshResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    s.cfg.AccessTokenExpiry,
	}, nil
}
