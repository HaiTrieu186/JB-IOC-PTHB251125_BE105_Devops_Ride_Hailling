package middleware

import (
	"pricing-service/internal/exception"

	"github.com/gofiber/fiber/v2"
)

// RequireAuth kiểm tra header X-User-Id và X-User-Role cho các endpoint công khai yêu cầu đăng nhập
func RequireAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := c.Get("X-User-Id")
		role := c.Get("X-User-Role")

		if userID == "" || (role != "CUSTOMER" && role != "DRIVER" && role != "ADMIN") {
			return exception.NewUnauthorizedError("Yêu cầu xác thực tài khoản")
		}

		return c.Next()
	}
}

// RequireAdmin kiểm tra vai trò ADMIN cho các endpoint quản trị
func RequireAdmin() fiber.Handler {
	return func(c *fiber.Ctx) error {
		role := c.Get("X-User-Role")
		if role != "ADMIN" {
			return exception.NewForbiddenError("Chỉ quản trị viên mới có quyền truy cập")
		}
		return c.Next()
	}
}
