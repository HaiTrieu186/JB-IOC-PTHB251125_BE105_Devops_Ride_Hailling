package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"dispatch-service/internal/exception"
)

func validateAuthHeaders(c *fiber.Ctx) (uuid.UUID, string, error) {
	userIDStr := c.Get("X-User-Id")
	role := c.Get("X-User-Role")

	if userIDStr == "" || role == "" {
		return uuid.Nil, "", exception.NewUnauthorizedError("Yêu cầu xác thực tài khoản")
	}

	userUUID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, "", exception.NewUnauthorizedError("X-User-Id không phải UUID hợp lệ")
	}

	if role != "CUSTOMER" && role != "DRIVER" && role != "ADMIN" {
		return uuid.Nil, "", exception.NewUnauthorizedError("Vai trò không hợp lệ")
	}

	return userUUID, role, nil
}

// RequireCustomer chỉ cho phép vai trò CUSTOMER
func RequireCustomer() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, role, err := validateAuthHeaders(c)
		if err != nil {
			return err
		}

		if role != "CUSTOMER" {
			return exception.NewForbiddenError("Chỉ khách hàng mới có quyền thực hiện thao tác này")
		}

		c.Locals("user_id", userID)
		c.Locals("user_role", role)
		return c.Next()
	}
}

// RequireDriver chỉ cho phép vai trò DRIVER
func RequireDriver() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, role, err := validateAuthHeaders(c)
		if err != nil {
			return err
		}

		if role != "DRIVER" {
			return exception.NewForbiddenError("Chỉ tài xế mới có quyền thực hiện thao tác này")
		}

		c.Locals("user_id", userID)
		c.Locals("user_role", role)
		return c.Next()
	}
}

// RequireAdmin chỉ cho phép vai trò ADMIN
func RequireAdmin() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, role, err := validateAuthHeaders(c)
		if err != nil {
			return err
		}

		if role != "ADMIN" {
			return exception.NewForbiddenError("Chỉ quản trị viên mới có quyền thực hiện thao tác này")
		}

		c.Locals("user_id", userID)
		c.Locals("user_role", role)
		return c.Next()
	}
}

// RequireCustomerOrDriver cho phép CUSTOMER hoặc DRIVER, cấm ADMIN (trả 403)
func RequireCustomerOrDriver() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, role, err := validateAuthHeaders(c)
		if err != nil {
			return err
		}

		if role == "ADMIN" {
			return exception.NewForbiddenError("Quản trị viên không có quyền truy cập endpoint này")
		}

		if role != "CUSTOMER" && role != "DRIVER" {
			return exception.NewForbiddenError("Quyền truy cập bị từ chối")
		}

		c.Locals("user_id", userID)
		c.Locals("user_role", role)
		return c.Next()
	}
}
