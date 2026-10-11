package controller

import (
	"location-service/internal/service"
	"location-service/internal/util"

	"github.com/gofiber/fiber/v2"
)

type PublicController struct {
	locationService service.LocationService
}

func NewPublicController(locationService service.LocationService) *PublicController {
	return &PublicController{locationService: locationService}
}

func (p *PublicController) GetActiveDrivers(c *fiber.Ctx) error {
	res, appErr := p.locationService.GetActiveDrivers(c.Context())
	if appErr != nil {
		return appErr
	}
	return util.SendSuccess(c, fiber.StatusOK, res)
}
