package controller

import (
	"location-service/internal/util"

	"github.com/gofiber/fiber/v2"
)

type HealthController struct{}

func NewHealthController() *HealthController {
	return &HealthController{}
}

func (h *HealthController) HealthCheck(c *fiber.Ctx) error {
	return util.SendSuccess(c, fiber.StatusOK, fiber.Map{
		"status": "ok",
	})
}
