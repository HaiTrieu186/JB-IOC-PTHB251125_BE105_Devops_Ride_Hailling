package middleware

import (
	"api-gateway/internal/exception"

	"github.com/gofiber/fiber/v2"
)

func AdminGuard() fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, ok := c.Locals("role").(string)
		if !ok || role != "ADMIN" {
			return exception.NewForbiddenError("Yêu cầu quyền Quản trị viên")
		}
		return c.Next()
	}
}
