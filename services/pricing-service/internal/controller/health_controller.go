package controller

import (
	"pricing-service/internal/dto"
	"pricing-service/internal/util"

	"github.com/gofiber/fiber/v2"
)

type HealthController struct{}

func NewHealthController() *HealthController {
	return &HealthController{}
}

func (h *HealthController) Health(c *fiber.Ctx) error {
	return util.SuccessResponse(c, fiber.StatusOK, dto.HealthResponse{
		Status: "ok",
	})
}
