package exception

import (
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v2"
)

const (
	ErrCodeValidation         = "VALIDATION_ERROR"
	ErrCodeUnauthorized       = "UNAUTHORIZED"
	ErrCodeForbidden          = "FORBIDDEN"
	ErrCodeNotFound           = "NOT_FOUND"
	ErrCodeTripNotFound       = "TRIP_NOT_FOUND"
	ErrCodeInsufficientBalance = "INSUFFICIENT_BALANCE"
	ErrCodeDriverNotAvailable = "DRIVER_NOT_AVAILABLE"
	ErrCodeNotOffered         = "NOT_OFFERED"
	ErrCodeTripAlreadyTaken   = "TRIP_ALREADY_TAKEN"
	ErrCodeInvalidTripStatus  = "INVALID_TRIP_STATUS"
	ErrCodeActiveTripExists   = "ACTIVE_TRIP_EXISTS"
	ErrCodeServiceUnavailable = "SERVICE_UNAVAILABLE"
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

func NewValidationError(message string) *AppError {
	return NewAppError(http.StatusBadRequest, ErrCodeValidation, message, nil)
}

func NewUnauthorizedError(message string) *AppError {
	return NewAppError(http.StatusUnauthorized, ErrCodeUnauthorized, message, nil)
}

func NewForbiddenError(message string) *AppError {
	return NewAppError(http.StatusForbidden, ErrCodeForbidden, message, nil)
}

func NewTripNotFoundError(message string) *AppError {
	return NewAppError(http.StatusNotFound, ErrCodeTripNotFound, message, nil)
}

func NewInsufficientBalanceError(message string) *AppError {
	return NewAppError(http.StatusBadRequest, ErrCodeInsufficientBalance, message, nil)
}

func NewDriverNotAvailableError(message string) *AppError {
	return NewAppError(http.StatusBadRequest, ErrCodeDriverNotAvailable, message, nil)
}

func NewNotOfferedError(message string) *AppError {
	return NewAppError(http.StatusForbidden, ErrCodeNotOffered, message, nil)
}

func NewTripAlreadyTakenError(message string) *AppError {
	return NewAppError(http.StatusConflict, ErrCodeTripAlreadyTaken, message, nil)
}

func NewInvalidTripStatusError(message string) *AppError {
	return NewAppError(http.StatusBadRequest, ErrCodeInvalidTripStatus, message, nil)
}

func NewActiveTripExistsError(message string) *AppError {
	return NewAppError(http.StatusConflict, ErrCodeActiveTripExists, message, nil)
}

func NewServiceUnavailableError(message string, err error) *AppError {
	return NewAppError(http.StatusServiceUnavailable, ErrCodeServiceUnavailable, message, err)
}

func ErrorHandler(c *fiber.Ctx, err error) error {
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
		code := ErrCodeValidation
		if fiberErr.Code == fiber.StatusNotFound {
			code = ErrCodeNotFound
		} else if fiberErr.Code == fiber.StatusUnauthorized {
			code = ErrCodeUnauthorized
		} else if fiberErr.Code == fiber.StatusForbidden {
			code = ErrCodeForbidden
		} else if fiberErr.Code >= 500 {
			code = ErrCodeServiceUnavailable
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

	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"success": false,
		"data":    nil,
		"error": fiber.Map{
			"code":    ErrCodeServiceUnavailable,
			"message": "Lỗi nội bộ hệ thống",
		},
	})
}
