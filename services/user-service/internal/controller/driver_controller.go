package controller

import (
	"user-service/internal/dto"
	"user-service/internal/exception"
	"user-service/internal/service"
	"user-service/internal/util"

	"github.com/gofiber/fiber/v2"
)

type DriverController struct {
	driverService service.DriverService
}

func NewDriverController(driverService service.DriverService) *DriverController {
	return &DriverController{driverService: driverService}
}

func (ctrl *DriverController) UpdateDriverStatus(c *fiber.Ctx) error {
	userRole, _ := c.Locals("user_role").(string)
	if userRole == "" {
		userRole = c.Get("X-User-Role")
	}
	driverID, _ := c.Locals("user_id").(string)
	if driverID == "" {
		driverID = c.Get("X-User-Id")
	}

	var req dto.DriverStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return exception.NewValidationError("Dữ liệu JSON không hợp lệ")
	}

	res, appErr := ctrl.driverService.UpdateStatus(c.Context(), driverID, userRole, &req)
	if appErr != nil {
		return appErr
	}

	return util.SendSuccess(c, fiber.StatusOK, res)
}
