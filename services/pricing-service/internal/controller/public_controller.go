package controller

import (
	"pricing-service/internal/dto"
	"pricing-service/internal/exception"
	"pricing-service/internal/service"
	"pricing-service/internal/util"

	"github.com/gofiber/fiber/v2"
)

type PublicController struct {
	pricingService service.PricingService
}

func NewPublicController(pricingService service.PricingService) *PublicController {
	return &PublicController{pricingService: pricingService}
}

func (p *PublicController) Estimate(c *fiber.Ctx) error {
	var req dto.EstimateRequest
	if err := c.BodyParser(&req); err != nil {
		return exception.NewValidationError("Payload JSON không hợp lệ hoặc thiếu trường")
	}

	res, err := p.pricingService.EstimatePublic(c.UserContext(), &req)
	if err != nil {
		return err
	}

	return util.SuccessResponse(c, fiber.StatusOK, res)
}
