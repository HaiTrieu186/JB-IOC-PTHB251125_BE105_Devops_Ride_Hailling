package util

import (
	"github.com/gofiber/fiber/v2"
)

func SuccessResponse(c *fiber.Ctx, statusCode int, data any) error {
	return c.Status(statusCode).JSON(fiber.Map{
		"success": true,
		"data":    data,
		"error":   nil,
	})
}
