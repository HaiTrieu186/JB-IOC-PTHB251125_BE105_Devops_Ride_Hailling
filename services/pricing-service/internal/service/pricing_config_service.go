package service

import (
	"context"

	"pricing-service/internal/entity"
)

type PricingConfigService interface {
	GetConfigForPricing(ctx context.Context) (*entity.PricingConfig, error)
	GetAdminConfig(ctx context.Context) (*entity.PricingConfig, error)
	UpdateAdminConfig(ctx context.Context, baseFare, pricePerKm int64, surgeThreshold float64) (*entity.PricingConfig, error)
}
