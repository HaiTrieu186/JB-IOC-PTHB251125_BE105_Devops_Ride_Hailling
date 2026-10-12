package exception

import (
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v2"
)

const (
	ErrCodeUnauthorized       = "UNAUTHORIZED"
	ErrCodeForbidden          = "FORBIDDEN"
	ErrCodeServiceUnavailable = "SERVICE_UNAVAILABLE"
	ErrCodeUpgradeRequired    = "UPGRADE_REQUIRED"
	ErrCodeNotFound           = "NOT_FOUND"
)

type AppError struct {
	HTTPStatus int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	Err        error  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func NewAppError(status int, code, message string, err error) *AppError {
	return &AppError{
		HTTPStatus: status,
		Code:       code,
		Message:    message,
		Err:        err,
	}
}

func NewUnauthorizedError(message string) *AppError {
	return NewAppError(http.StatusUnauthorized, ErrCodeUnauthorized, message, nil)
}

func NewForbiddenError(message string) *AppError {
	return NewAppError(http.StatusForbidden, ErrCodeForbidden, message, nil)
}

func NewServiceUnavailableError(message string, err error) *AppError {
	return NewAppError(http.StatusServiceUnavailable, ErrCodeServiceUnavailable, message, err)
}

func NewUpgradeRequiredError(message string) *AppError {
	return NewAppError(http.StatusUpgradeRequired, ErrCodeUpgradeRequired, message, nil)
}

func CustomErrorHandler(c *fiber.Ctx, err error) error {
	var appErr *AppError
	if errors.As(err, &appErr) {
		c.Response().Reset()
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
		c.Response().Reset()
		code := ErrCodeServiceUnavailable
		statusCode := fiberErr.Code
		message := fiberErr.Message

		switch fiberErr.Code {
		case fiber.StatusUpgradeRequired:
			code = ErrCodeUpgradeRequired
			message = "Nâng cấp giao thức WebSocket yêu cầu"
		case fiber.StatusNotFound, fiber.StatusMethodNotAllowed:
			statusCode = fiber.StatusNotFound
			code = ErrCodeNotFound
			message = "Đường dẫn không tồn tại"
		case fiber.StatusUnauthorized:
			code = ErrCodeUnauthorized
			message = "Không có quyền truy cập"
		case fiber.StatusForbidden:
			code = ErrCodeForbidden
			message = "Truy cập bị từ chối"
		default:
			if fiberErr.Code >= 500 {
				statusCode = fiber.StatusServiceUnavailable
				code = ErrCodeServiceUnavailable
				message = "Dịch vụ tạm thời không khả dụng"
			}
		}

		return c.Status(statusCode).JSON(fiber.Map{
			"success": false,
			"data":    nil,
			"error": fiber.Map{
				"code":    code,
				"message": message,
			},
		})
	}

	c.Response().Reset()
	return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
		"success": false,
		"data":    nil,
		"error": fiber.Map{
			"code":    ErrCodeServiceUnavailable,
			"message": "Lỗi nội bộ hệ thống",
		},
	})
}
