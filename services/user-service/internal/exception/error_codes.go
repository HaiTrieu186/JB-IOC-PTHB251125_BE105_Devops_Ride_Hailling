package exception

const (
	ErrValidation          = "VALIDATION_ERROR"
	ErrUnauthorized        = "UNAUTHORIZED"
	ErrForbidden           = "FORBIDDEN"
	ErrNotFound            = "NOT_FOUND"
	ErrUserAlreadyExists   = "USER_ALREADY_EXISTS"
	ErrInvalidRefreshToken = "INVALID_REFRESH_TOKEN"
	ErrDriverCannotLogout  = "DRIVER_CANNOT_LOGOUT"
	ErrDriverBusy          = "DRIVER_BUSY"
	ErrDriverNotAvailable  = "DRIVER_NOT_AVAILABLE"
	ErrServiceUnavailable  = "SERVICE_UNAVAILABLE"
	ErrInternal            = "INTERNAL_ERROR"
)
