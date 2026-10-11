package impl

import (
	"context"
	"encoding/json"
	"time"

	"pricing-service/internal/entity"
	"pricing-service/internal/repository"

	"github.com/redis/go-redis/v9"
)

const configCacheKey = "pricing:config"
const configCacheTTL = 3600 * time.Second

type cachedConfigData struct {
	BaseFare       int64   `json:"base_fare"`
	PricePerKm     int64   `json:"price_per_km"`
	SurgeThreshold float64 `json:"surge_threshold"`
}

type configCacheRepositoryImpl struct {
	rdb *redis.Client
}

func NewConfigCacheRepository(rdb *redis.Client) repository.ConfigCacheRepository {
	return &configCacheRepositoryImpl{rdb: rdb}
}

func (r *configCacheRepositoryImpl) GetCachedConfig(ctx context.Context) (*entity.PricingConfig, error) {
	val, err := r.rdb.Get(ctx, configCacheKey).Result()
	if err != nil {
		return nil, err
	}

	var data cachedConfigData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, err
	}

	return &entity.PricingConfig{
		ID:             1,
		BaseFare:       data.BaseFare,
		PricePerKm:     data.PricePerKm,
		SurgeThreshold: data.SurgeThreshold,
	}, nil
}

func (r *configCacheRepositoryImpl) SetCachedConfig(ctx context.Context, cfg *entity.PricingConfig) error {
	data := cachedConfigData{
		BaseFare:       cfg.BaseFare,
		PricePerKm:     cfg.PricePerKm,
		SurgeThreshold: cfg.SurgeThreshold,
	}

	b, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return r.rdb.Set(ctx, configCacheKey, string(b), configCacheTTL).Err()
}

func (r *configCacheRepositoryImpl) InvalidateConfig(ctx context.Context) error {
	return r.rdb.Del(ctx, configCacheKey).Err()
}
