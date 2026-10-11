package repository

import (
	"context"

	"pricing-service/internal/entity"
)

type ConfigCacheRepository interface {
	GetCachedConfig(ctx context.Context) (*entity.PricingConfig, error)
	SetCachedConfig(ctx context.Context, config *entity.PricingConfig) error
	InvalidateConfig(ctx context.Context) error
}
