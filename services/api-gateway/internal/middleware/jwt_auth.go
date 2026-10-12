package middleware

import (
	"context"
	"strings"
	"time"

	"api-gateway/internal/config"
	"api-gateway/internal/exception"
	"api-gateway/internal/repository"
	"api-gateway/internal/util"

	"github.com/gofiber/fiber/v2"
)

func JWTAuth(cfg *config.Config, blacklistRepo repository.BlacklistRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get(fiber.HeaderAuthorization)
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			return exception.NewUnauthorizedError("Token xác thực bị thiếu hoặc không đúng định dạng")
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		tokenStr = strings.TrimSpace(tokenStr)
		if tokenStr == "" {
			return exception.NewUnauthorizedError("Token xác thực không hợp lệ")
		}

		claims, err := util.ValidateToken(tokenStr, cfg.JWTSecret)
		if err != nil {
			return exception.NewUnauthorizedError("Token không hợp lệ hoặc đã hết hạn")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		isBlacklisted, err := blacklistRepo.IsBlacklisted(ctx, claims.JTI)
		if err != nil {
			return exception.NewServiceUnavailableError("Dịch vụ xác thực tạm thời không khả dụng", err)
		}
		if isBlacklisted {
			return exception.NewUnauthorizedError("Token đã bị thu hồi")
		}

		c.Locals("user_id", claims.UserID)
		c.Locals("role", claims.Role)

		c.Request().Header.Set("X-User-Id", claims.UserID)
		c.Request().Header.Set("X-User-Role", claims.Role)

		return c.Next()
	}
}
