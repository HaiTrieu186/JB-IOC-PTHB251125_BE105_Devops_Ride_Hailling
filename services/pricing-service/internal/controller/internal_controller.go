package controller

import (
	"pricing-service/internal/dto"
	"pricing-service/internal/exception"
	"pricing-service/internal/service"
	"pricing-service/internal/util"

	"github.com/gofiber/fiber/v2"
)

type InternalController struct {
	pricingService service.PricingService
}

func NewInternalController(pricingService service.PricingService) *InternalController {
	return &InternalController{pricingService: pricingService}
}

func (i *InternalController) Estimate(c *fiber.Ctx) error {
	var req dto.EstimateRequest
	if err := c.BodyParser(&req); err != nil {
		return exception.NewValidationError("Payload JSON không hợp lệ hoặc thiếu trường")
	}

	res, err := i.pricingService.EstimateInternal(c.UserContext(), &req)
	if err != nil {
		return err
	}

	return util.SuccessResponse(c, fiber.StatusOK, res)
}
