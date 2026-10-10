package middleware

import (
	"user-service/internal/exception"

	"github.com/gofiber/fiber/v2"
)

// AuthRequired kiểm tra sự tồn tại của header X-User-Id và X-User-Role
func AuthRequired() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := c.Get("X-User-Id")
		userRole := c.Get("X-User-Role")
		if userID == "" || userRole == "" {
			return exception.NewUnauthorizedError("Thiếu header xác thực X-User-Id hoặc X-User-Role")
		}
		c.Locals("user_id", userID)
		c.Locals("user_role", userRole)
		return c.Next()
	}
}

// RequireRoles kiểm tra X-User-Role có thuộc danh sách allowedRoles không
func RequireRoles(allowedRoles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := c.Get("X-User-Id")
		userRole := c.Get("X-User-Role")
		if userID == "" || userRole == "" {
			return exception.NewUnauthorizedError("Thiếu header xác thực X-User-Id hoặc X-User-Role")
		}

		c.Locals("user_id", userID)
		c.Locals("user_role", userRole)

		for _, role := range allowedRoles {
			if userRole == role {
				return c.Next()
			}
		}

		if len(allowedRoles) == 1 && allowedRoles[0] == "DRIVER" {
			return exception.NewForbiddenError("Chỉ tài xế mới có quyền cập nhật trạng thái")
		}
		return exception.NewForbiddenError("Quyền truy cập bị từ chối")
	}
}
