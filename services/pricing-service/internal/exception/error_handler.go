package exception

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v2"
)

func CustomErrorHandler(c *fiber.Ctx, err error) error {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return c.Status(appErr.HTTPStatus).JSON(fiber.Map{
			"success": false,
			"data":    nil,
			"error": fiber.Map{
				"code":    appErr.Code,
				"message": appErr.Message,
			},
		})
	}

	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		code := ErrServiceUnavailable
		if fiberErr.Code == fiber.StatusNotFound || fiberErr.Code == fiber.StatusMethodNotAllowed {
			code = ErrNotFound
		} else if fiberErr.Code == fiber.StatusBadRequest {
			code = ErrValidation
		} else if fiberErr.Code == fiber.StatusUnauthorized {
			code = ErrUnauthorized
		} else if fiberErr.Code == fiber.StatusForbidden {
			code = ErrForbidden
		}
		return c.Status(fiberErr.Code).JSON(fiber.Map{
			"success": false,
			"data":    nil,
			"error": fiber.Map{
				"code":    code,
				"message": fiberErr.Message,
			},
		})
	}

	slog.Error("unhandled internal error", "error", err)
	return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
		"success": false,
		"data":    nil,
		"error": fiber.Map{
			"code":    ErrServiceUnavailable,
			"message": "Dịch vụ tạm thời không khả dụng",
		},
	})
}
