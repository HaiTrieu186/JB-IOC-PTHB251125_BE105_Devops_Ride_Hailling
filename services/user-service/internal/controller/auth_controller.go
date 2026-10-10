package controller

import (
	"user-service/internal/dto"
	"user-service/internal/exception"
	"user-service/internal/service"
	"user-service/internal/util"

	"github.com/gofiber/fiber/v2"
)

type AuthController struct {
	authService service.AuthService
}

func NewAuthController(authService service.AuthService) *AuthController {
	return &AuthController{authService: authService}
}

func (ctrl *AuthController) Register(c *fiber.Ctx) error {
	var req dto.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return exception.NewValidationError("Dữ liệu JSON không hợp lệ")
	}

	res, appErr := ctrl.authService.Register(c.Context(), &req)
	if appErr != nil {
		return appErr
	}

	return util.SendSuccess(c, fiber.StatusCreated, res)
}

func (ctrl *AuthController) Login(c *fiber.Ctx) error {
	var req dto.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return exception.NewValidationError("Dữ liệu JSON không hợp lệ")
	}

	res, appErr := ctrl.authService.Login(c.Context(), &req)
	if appErr != nil {
		return appErr
	}

	return util.SendSuccess(c, fiber.StatusOK, res)
}

func (ctrl *AuthController) Refresh(c *fiber.Ctx) error {
	var req dto.RefreshRequest
	if err := c.BodyParser(&req); err != nil {
		return exception.NewValidationError("Dữ liệu JSON không hợp lệ")
	}

	res, appErr := ctrl.authService.Refresh(c.Context(), &req)
	if appErr != nil {
		return appErr
	}

	return util.SendSuccess(c, fiber.StatusOK, res)
}

func (ctrl *AuthController) Logout(c *fiber.Ctx) error {
	var req dto.LogoutRequest
	if err := c.BodyParser(&req); err != nil {
		return exception.NewValidationError("Dữ liệu JSON không hợp lệ")
	}

	userID, _ := c.Locals("user_id").(string)
	if userID == "" {
		userID = c.Get("X-User-Id")
	}
	userRole, _ := c.Locals("user_role").(string)
	if userRole == "" {
		userRole = c.Get("X-User-Role")
	}
	authHeader := c.Get("Authorization")

	if appErr := ctrl.authService.Logout(c.Context(), &req, userID, userRole, authHeader); appErr != nil {
		return appErr
	}

	return util.SendSuccess(c, fiber.StatusOK, dto.LogoutResponse{
		Message: "Đăng xuất thành công",
	})
}
