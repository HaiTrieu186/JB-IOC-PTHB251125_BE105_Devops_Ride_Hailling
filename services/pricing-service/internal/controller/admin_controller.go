package controller

import (
	"pricing-service/internal/dto"
	"pricing-service/internal/exception"
	"pricing-service/internal/service"
	"pricing-service/internal/util"

	"github.com/gofiber/fiber/v2"
)

type AdminController struct {
	configService service.PricingConfigService
}

func NewAdminController(configService service.PricingConfigService) *AdminController {
	return &AdminController{configService: configService}
}

func (a *AdminController) GetConfig(c *fiber.Ctx) error {
	cfg, err := a.configService.GetAdminConfig(c.UserContext())
	if err != nil {
		return err
	}

	return util.SuccessResponse(c, fiber.StatusOK, dto.AdminPricingConfigResponse{
		BaseFare:       cfg.BaseFare,
		PricePerKm:     cfg.PricePerKm,
		SurgeThreshold: cfg.SurgeThreshold,
		UpdatedAt:      cfg.UpdatedAt,
	})
}

func (a *AdminController) UpdateConfig(c *fiber.Ctx) error {
	var req dto.UpdatePricingConfigRequest
	if err := c.BodyParser(&req); err != nil {
		return exception.NewValidationError("Payload JSON không hợp lệ")
	}

	if req.BaseFare == nil || req.PricePerKm == nil || req.SurgeThreshold == nil {
		return exception.NewValidationError("Các trường base_fare, price_per_km và surge_threshold là bắt buộc")
	}

	cfg, err := a.configService.UpdateAdminConfig(c.UserContext(), *req.BaseFare, *req.PricePerKm, *req.SurgeThreshold)
	if err != nil {
		return err
	}

	return util.SuccessResponse(c, fiber.StatusOK, dto.AdminPricingConfigResponse{
		BaseFare:       cfg.BaseFare,
		PricePerKm:     cfg.PricePerKm,
		SurgeThreshold: cfg.SurgeThreshold,
		UpdatedAt:      cfg.UpdatedAt,
	})
}
