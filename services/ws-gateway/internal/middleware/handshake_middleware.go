package middleware

import (
	"strings"

	"ws-gateway/internal/config"
	"ws-gateway/internal/exception"
	"ws-gateway/internal/repository"
	"ws-gateway/internal/service"
	"ws-gateway/internal/util"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

func NewHandshakeMiddleware(cfg *config.Config, blacklistRepo repository.BlacklistReader, hub service.ConnectionHub) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. Kiểm tra yêu cầu nâng cấp WebSocket; nếu không phải upgrade -> 426
		if !websocket.IsWebSocketUpgrade(c) {
			return exception.NewUpgradeRequiredError("Yêu cầu nâng cấp giao thức WebSocket")
		}

		// 2. Trích xuất Bearer token từ header Authorization (bỏ qua query string -> 401)
		authHeader := c.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			return exception.NewUnauthorizedError("Token không được cung cấp hoặc sai định dạng")
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == "" {
			return exception.NewUnauthorizedError("Token rỗng")
		}

		// 3. Xác thực chữ ký JWT (HS256, exp)
		claims, err := util.ValidateToken(tokenString, cfg.JWTSecret)
		if err != nil {
			return exception.NewUnauthorizedError("Token không hợp lệ hoặc đã hết hạn")
		}

		// 4. Kiểm tra blacklist trong Redis (fail-closed nếu Redis lỗi -> 503)
		isBlacklisted, err := blacklistRepo.IsBlacklisted(c.Context(), claims.JTI)
		if err != nil {
			return exception.NewServiceUnavailableError("Dịch vụ xác thực tạm thời gián đoạn", err)
		}
		if isBlacklisted {
			return exception.NewUnauthorizedError("Phiên đăng nhập đã bị thu hồi")
		}

		// 5. Kiểm tra role (ADMIN -> 403)
		if claims.Role == "ADMIN" {
			return exception.NewForbiddenError("Quản trị viên không được phép kết nối WebSocket")
		}
		if claims.Role != "CUSTOMER" && claims.Role != "DRIVER" {
			return exception.NewForbiddenError("Vai trò không hợp lệ")
		}

		// 6. Kiểm tra MAX_CONNECTIONS (vượt ngưỡng -> 503)
		if hub.ActiveConnections() >= cfg.MaxConnections {
			return exception.NewServiceUnavailableError("Hệ thống đã đạt giới hạn kết nối đồng thời", nil)
		}

		// 7. Lưu user_id và role vào c.Locals cho handler
		c.Locals("user_id", claims.UserID)
		c.Locals("role", claims.Role)

		return c.Next()
	}
}
