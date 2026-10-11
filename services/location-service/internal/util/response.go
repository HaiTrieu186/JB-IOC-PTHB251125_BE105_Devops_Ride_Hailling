package util

import (
	"github.com/gofiber/fiber/v2"
)

func SendSuccess(c *fiber.Ctx, status int, data interface{}) error {
	return c.Status(status).JSON(fiber.Map{
		"success": true,
		"data":    data,
		"error":   nil,
	})
}
