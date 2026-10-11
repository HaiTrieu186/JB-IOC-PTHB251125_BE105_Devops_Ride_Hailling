package controller

import (
	"location-service/internal/dto"
	"location-service/internal/exception"
	"location-service/internal/service"
	"location-service/internal/util"

	"github.com/gofiber/fiber/v2"
)

type InternalController struct {
	locationService service.LocationService
}

func NewInternalController(locationService service.LocationService) *InternalController {
	return &InternalController{locationService: locationService}
}

func (ctrl *InternalController) RecordLocation(c *fiber.Ctx) error {
	var req dto.RecordLocationRequest
	if err := c.BodyParser(&req); err != nil {
		return exception.NewValidationError("Payload JSON không hợp lệ")
	}

	if appErr := ctrl.locationService.RecordLocation(c.Context(), &req); appErr != nil {
		return appErr
	}

	return util.SendSuccess(c, fiber.StatusOK, fiber.Map{
		"success": true,
	})
}

func (ctrl *InternalController) GetCandidates(c *fiber.Ctx) error {
	var req dto.CandidatesRequest
	if err := c.BodyParser(&req); err != nil {
		return exception.NewValidationError("Payload JSON không hợp lệ")
	}

	res, appErr := ctrl.locationService.GetCandidates(c.Context(), &req)
	if appErr != nil {
		return appErr
	}

	return util.SendSuccess(c, fiber.StatusOK, res)
}

func (ctrl *InternalController) GetGeohashDrivers(c *fiber.Ctx) error {
	var req dto.GeohashDriversRequest
	if err := c.BodyParser(&req); err != nil {
		return exception.NewValidationError("Payload JSON không hợp lệ")
	}

	res, appErr := ctrl.locationService.GetGeohashDrivers(c.Context(), &req)
	if appErr != nil {
		return appErr
	}

	return util.SendSuccess(c, fiber.StatusOK, res)
}
