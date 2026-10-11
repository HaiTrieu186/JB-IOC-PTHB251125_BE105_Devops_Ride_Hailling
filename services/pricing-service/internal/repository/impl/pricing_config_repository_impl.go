package impl

import (
	"context"

	"pricing-service/internal/entity"
	"pricing-service/internal/repository"

	"gorm.io/gorm"
)

type pricingConfigRepositoryImpl struct {
	db *gorm.DB
}

func NewPricingConfigRepository(db *gorm.DB) repository.PricingConfigRepository {
	return &pricingConfigRepositoryImpl{db: db}
}

func (r *pricingConfigRepositoryImpl) GetConfig(ctx context.Context) (*entity.PricingConfig, error) {
	var cfg entity.PricingConfig
	err := r.db.WithContext(ctx).First(&cfg, 1).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *pricingConfigRepositoryImpl) UpsertConfig(ctx context.Context, config *entity.PricingConfig) (*entity.PricingConfig, error) {
	var updated entity.PricingConfig
	query := `INSERT INTO pricing_configs (id, base_fare, price_per_km, surge_threshold, updated_at)
		VALUES (1, ?, ?, ?, now())
		ON CONFLICT (id) DO UPDATE SET
			base_fare = EXCLUDED.base_fare,
			price_per_km = EXCLUDED.price_per_km,
			surge_threshold = EXCLUDED.surge_threshold,
			updated_at = now()
		RETURNING id, base_fare, price_per_km, surge_threshold, updated_at`

	err := r.db.WithContext(ctx).Raw(query, config.BaseFare, config.PricePerKm, config.SurgeThreshold).Scan(&updated).Error
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func (r *pricingConfigRepositoryImpl) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&entity.PricingConfig{}).Count(&count).Error
	return count, err
}

func (r *pricingConfigRepositoryImpl) SeedIfNotExists(ctx context.Context, defaultCfg *entity.PricingConfig) error {
	query := `INSERT INTO pricing_configs (id, base_fare, price_per_km, surge_threshold, updated_at)
		VALUES (1, ?, ?, ?, now())
		ON CONFLICT (id) DO NOTHING`
	return r.db.WithContext(ctx).Exec(query, defaultCfg.BaseFare, defaultCfg.PricePerKm, defaultCfg.SurgeThreshold).Error
}
