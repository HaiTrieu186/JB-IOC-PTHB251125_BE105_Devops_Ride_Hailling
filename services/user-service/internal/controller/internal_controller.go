package controller

import (
	"user-service/internal/dto"
	"user-service/internal/exception"
	"user-service/internal/service"
	"user-service/internal/util"

	"github.com/gofiber/fiber/v2"
)

type InternalController struct {
	driverService service.DriverService
}

func NewInternalController(driverService service.DriverService) *InternalController {
	return &InternalController{driverService: driverService}
}

func (ctrl *InternalController) FilterOnline(c *fiber.Ctx) error {
	var req dto.FilterOnlineRequest
	if err := c.BodyParser(&req); err != nil || req.DriverIDs == nil {
		return exception.NewValidationError("Dữ liệu driver_ids không hợp lệ")
	}

	res, appErr := ctrl.driverService.FilterOnline(c.Context(), &req)
	if appErr != nil {
		return appErr
	}

	return util.SendSuccess(c, fiber.StatusOK, res)
}

func (ctrl *InternalController) UpdateDriverInternalStatus(c *fiber.Ctx) error {
	id := c.Params("id")

	var req dto.DriverInternalStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return exception.NewValidationError("Dữ liệu JSON không hợp lệ")
	}

	res, appErr := ctrl.driverService.UpdateInternalStatus(c.Context(), id, &req)
	if appErr != nil {
		return appErr
	}

	return util.SendSuccess(c, fiber.StatusOK, res)
}
