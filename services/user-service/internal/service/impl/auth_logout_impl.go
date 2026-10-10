package impl

import (
	"context"
	"strings"
	"time"

	"user-service/internal/dto"
	"user-service/internal/exception"
	"user-service/internal/util"
)

func (s *authServiceImpl) Logout(ctx context.Context, req *dto.LogoutRequest, userID string, userRole string, authHeader string) *exception.AppError {
	req.RefreshToken = strings.TrimSpace(req.RefreshToken)
	if req.RefreshToken == "" {
		return exception.NewValidationError("Thiếu refresh_token")
	}

	if userID == "" || userRole == "" {
		return exception.NewUnauthorizedError("Thiếu header xác thực X-User-Id hoặc X-User-Role")
	}

	if !strings.HasPrefix(authHeader, "Bearer ") {
		return exception.NewUnauthorizedError("Thiếu hoặc sai định dạng Authorization header")
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	claims, err := util.ValidateToken(tokenString, s.cfg.JWTSecret)
	if err != nil {
		return exception.NewUnauthorizedError("Access token không hợp lệ")
	}

	tokenUserID, _ := claims["user_id"].(string)
	if tokenUserID != userID {
		return exception.NewUnauthorizedError("Token không khớp với người dùng")
	}

	jti, _ := claims["jti"].(string)
	expFloat, _ := claims["exp"].(float64)

	// 1. Kiểm tra Refresh Token trong Redis thuộc đúng người gọi
	storedUserID, err := s.tokenRepo.GetRefreshToken(ctx, req.RefreshToken)
	if err != nil || storedUserID == "" || storedUserID != userID {
		return exception.NewAppError(exception.ErrInvalidRefreshToken, 401, "Refresh token không tồn tại hoặc đã được sử dụng")
	}

	// 2. Nếu là DRIVER: kiểm tra không BUSY rồi chuyển OFFLINE
	if userRole == "DRIVER" {
		driverUUID, err := util.ParseUUID(userID)
		if err == nil {
			affected, dbErr := s.userRepo.UpdateDriverStatusConditional(ctx, driverUUID, "OFFLINE")
			if dbErr != nil {
				return exception.NewServiceUnavailableError("Lỗi cập nhật trạng thái tài xế")
			}
			if affected == 0 {
				u, _ := s.userRepo.FindByID(ctx, driverUUID)
				if u != nil && u.DriverStatus != nil && *u.DriverStatus == "BUSY" {
					return exception.NewAppError(exception.ErrDriverCannotLogout, 400, "Tài xế đang trong chuyến đi, không thể đăng xuất")
				}
			}
		}
	}

	// 3. Xóa Refresh Token
	_ = s.tokenRepo.DelRefreshToken(ctx, req.RefreshToken)

	// 4. Blacklist Access Token
	nowUnix := time.Now().UTC().Unix()
	ttlSec := int64(expFloat) - nowUnix
	if ttlSec <= 0 {
		ttlSec = 1
	}
	if jti != "" {
		_ = s.tokenRepo.BlacklistToken(ctx, jti, time.Duration(ttlSec)*time.Second)
	}

	// 5. Phát tín hiệu ngắt kết nối WebSocket
	_ = s.wsPublisher.PublishDisconnect(ctx, userID)

	return nil
}
