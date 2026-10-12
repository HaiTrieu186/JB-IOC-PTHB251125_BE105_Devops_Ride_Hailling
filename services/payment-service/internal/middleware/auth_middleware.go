package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"payment-service/internal/exception"
)

func RequireWalletUser() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userIDStr := c.Get("X-User-Id")
		role := c.Get("X-User-Role")

		if userIDStr == "" || role == "" {
			return exception.NewUnauthorizedError("Yêu cầu xác thực tài khoản")
		}

		userUUID, err := uuid.Parse(userIDStr)
		if err != nil {
			return exception.NewUnauthorizedError("X-User-Id không phải UUID hợp lệ")
		}

		if role != "CUSTOMER" && role != "DRIVER" && role != "ADMIN" {
			return exception.NewUnauthorizedError("Vai trò không hợp lệ")
		}

		// Public wallet endpoints forbid ADMIN per LLD Section 3.1 & Section 5
		if role == "ADMIN" {
			return exception.NewForbiddenError("Chỉ khách hàng hoặc tài xế mới có quyền truy cập ví")
		}

		c.Locals("user_id", userUUID)
		c.Locals("user_role", role)

		return c.Next()
	}
}
