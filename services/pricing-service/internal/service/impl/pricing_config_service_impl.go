package impl

import (
	"context"
	"log/slog"
	"math"

	"pricing-service/internal/entity"
	"pricing-service/internal/exception"
	"pricing-service/internal/repository"
	"pricing-service/internal/service"
)

type pricingConfigServiceImpl struct {
	configRepo repository.PricingConfigRepository
	cacheRepo  repository.ConfigCacheRepository
}

func NewPricingConfigService(
	configRepo repository.PricingConfigRepository,
	cacheRepo repository.ConfigCacheRepository,
) service.PricingConfigService {
	return &pricingConfigServiceImpl{
		configRepo: configRepo,
		cacheRepo:  cacheRepo,
	}
}

func (s *pricingConfigServiceImpl) GetConfigForPricing(ctx context.Context) (*entity.PricingConfig, error) {
	// 1. Kiểm tra cache Redis
	cached, err := s.cacheRepo.GetCachedConfig(ctx)
	if err == nil && cached != nil {
		return cached, nil
	}

	// 2. Cache miss hoặc Redis lỗi: đọc thẳng từ DB
	cfg, err := s.configRepo.GetConfig(ctx)
	if err != nil {
		slog.Error("failed to get pricing config from DB", "error", err)
		return nil, exception.NewServiceUnavailableError("Không thể tải cấu hình bảng giá từ cơ sở dữ liệu")
	}

	// Thử lưu lại cache (nếu lỗi thì bỏ qua)
	if setErr := s.cacheRepo.SetCachedConfig(ctx, cfg); setErr != nil {
		slog.Warn("failed to set pricing config cache in redis", "error", setErr)
	}

	return cfg, nil
}

func (s *pricingConfigServiceImpl) GetAdminConfig(ctx context.Context) (*entity.PricingConfig, error) {
	// Luôn đọc thẳng từ DB theo Mục 4.1 LLD
	cfg, err := s.configRepo.GetConfig(ctx)
	if err != nil {
		slog.Error("failed to get admin pricing config from DB", "error", err)
		return nil, exception.NewServiceUnavailableError("Không thể tải cấu hình bảng giá từ cơ sở dữ liệu")
	}
	return cfg, nil
}

func (s *pricingConfigServiceImpl) UpdateAdminConfig(ctx context.Context, baseFare, pricePerKm int64, surgeThreshold float64) (*entity.PricingConfig, error) {
	// Validate
	if baseFare <= 0 || pricePerKm <= 0 {
		return nil, exception.NewValidationError("base_fare và price_per_km phải lớn hơn 0")
	}

	if surgeThreshold < 0.01 || surgeThreshold > 99.99 {
		return nil, exception.NewValidationError("surge_threshold phải nằm trong khoảng [0.01, 99.99]")
	}

	if math.Abs(surgeThreshold*100-math.Round(surgeThreshold*100)) > 1e-5 {
		return nil, exception.NewValidationError("surge_threshold chỉ được tối đa 2 chữ số thập phân")
	}

	threshold := math.Round(surgeThreshold*100) / 100

	updated, err := s.configRepo.UpsertConfig(ctx, &entity.PricingConfig{
		BaseFare:       baseFare,
		PricePerKm:     pricePerKm,
		SurgeThreshold: threshold,
	})
	if err != nil {
		slog.Error("failed to update pricing config in DB", "error", err)
		return nil, exception.NewServiceUnavailableError("Không thể cập nhật cấu hình bảng giá")
	}

	// Xóa cache Redis
	if delErr := s.cacheRepo.InvalidateConfig(ctx); delErr != nil {
		slog.Warn("failed to invalidate pricing config cache in redis", "error", delErr)
	}

	return updated, nil
}
