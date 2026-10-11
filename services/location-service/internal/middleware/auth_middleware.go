package middleware

import (
	"location-service/internal/exception"

	"github.com/gofiber/fiber/v2"
)

func RequireAdmin() fiber.Handler {
	return func(c *fiber.Ctx) error {
		role := c.Get("X-User-Role")
		if role != "ADMIN" {
			return exception.NewForbiddenError("Chỉ quản trị viên mới có quyền truy cập")
		}
		return c.Next()
	}
}
