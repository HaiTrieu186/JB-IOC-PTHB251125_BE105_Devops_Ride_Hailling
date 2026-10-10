package exception

import "fmt"

type AppError struct {
	Code       string `json:"code"`
	HTTPStatus int    `json:"-"`
	Message    string `json:"message"`
}

func (e *AppError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func NewAppError(code string, httpStatus int, message string) *AppError {
	return &AppError{
		Code:       code,
		HTTPStatus: httpStatus,
		Message:    message,
	}
}

func NewValidationError(message string) *AppError {
	return NewAppError(ErrValidation, 400, message)
}

func NewUnauthorizedError(message string) *AppError {
	return NewAppError(ErrUnauthorized, 401, message)
}

func NewForbiddenError(message string) *AppError {
	return NewAppError(ErrForbidden, 403, message)
}

func NewNotFoundError(message string) *AppError {
	return NewAppError(ErrNotFound, 404, message)
}

func NewConflictError(message string) *AppError {
	return NewAppError(ErrUserAlreadyExists, 409, message)
}

func NewServiceUnavailableError(message string) *AppError {
	return NewAppError(ErrServiceUnavailable, 503, message)
}
