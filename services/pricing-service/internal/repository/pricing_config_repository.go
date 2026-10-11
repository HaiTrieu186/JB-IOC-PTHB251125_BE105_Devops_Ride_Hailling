package repository

import (
	"context"

	"pricing-service/internal/entity"
)

type PricingConfigRepository interface {
	GetConfig(ctx context.Context) (*entity.PricingConfig, error)
	UpsertConfig(ctx context.Context, config *entity.PricingConfig) (*entity.PricingConfig, error)
	Count(ctx context.Context) (int64, error)
	SeedIfNotExists(ctx context.Context, defaultCfg *entity.PricingConfig) error
}
