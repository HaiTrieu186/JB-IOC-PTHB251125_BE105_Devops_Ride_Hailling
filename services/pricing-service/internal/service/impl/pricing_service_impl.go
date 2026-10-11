package impl

import (
	"context"
	"math"

	"pricing-service/internal/dto"
	"pricing-service/internal/exception"
	"pricing-service/internal/service"
	"pricing-service/internal/util"
)

type pricingServiceImpl struct {
	pricingConfigService service.PricingConfigService
	distanceCalculator   service.DistanceCalculator
	surgeService         service.SurgeService
}

func NewPricingService(
	pricingConfigService service.PricingConfigService,
	distanceCalculator service.DistanceCalculator,
	surgeService service.SurgeService,
) service.PricingService {
	return &pricingServiceImpl{
		pricingConfigService: pricingConfigService,
		distanceCalculator:   distanceCalculator,
		surgeService:         surgeService,
	}
}

func (s *pricingServiceImpl) validateCoordinates(req *dto.EstimateRequest) error {
	if req.PickupLat == nil || req.PickupLng == nil || req.DropoffLat == nil || req.DropoffLng == nil {
		return exception.NewValidationError("Tọa độ điểm đón và điểm trả là bắt buộc")
	}

	pLat := *req.PickupLat
	pLng := *req.PickupLng
	dLat := *req.DropoffLat
	dLng := *req.DropoffLng

	// Dải chuẩn LLD-00: vĩ độ [-85.05112878, 85.05112878], kinh độ [-180, 180]
	if pLat < -85.05112878 || pLat > 85.05112878 || dLat < -85.05112878 || dLat > 85.05112878 {
		return exception.NewValidationError("Vĩ độ không hợp lệ, phải thuộc [-85.05112878, 85.05112878]")
	}

	if pLng < -180.0 || pLng > 180.0 || dLng < -180.0 || dLng > 180.0 {
		return exception.NewValidationError("Kinh độ không hợp lệ, phải thuộc [-180, 180]")
	}

	// Đón trùng trả
	if pLat == dLat && pLng == dLng {
		return exception.NewValidationError("Điểm đón và điểm trả không được trùng nhau")
	}

	return nil
}

func (s *pricingServiceImpl) EstimatePublic(ctx context.Context, req *dto.EstimateRequest) (*dto.PublicEstimateResponse, error) {
	if err := s.validateCoordinates(req); err != nil {
		return nil, err
	}

	cfg, err := s.pricingConfigService.GetConfigForPricing(ctx)
	if err != nil {
		return nil, err
	}

	distResult, err := s.distanceCalculator.Calculate(ctx, *req.PickupLat, *req.PickupLng, *req.DropoffLat, *req.DropoffLng)
	if err != nil {
		return nil, err
	}

	surge, err := s.surgeService.CalculateSurge(ctx, *req.PickupLat, *req.PickupLng, cfg.SurgeThreshold)
	if err != nil {
		return nil, err
	}

	totalFare := util.CalculateFare(cfg.BaseFare, cfg.PricePerKm, distResult.DistanceMeters, surge)
	etaMinutes := int(math.Ceil(float64(distResult.ETASeconds) / 60.0))
	distanceKm := float64(distResult.DistanceMeters) / 1000.0

	return &dto.PublicEstimateResponse{
		Distance:        distanceKm,
		ETA:             etaMinutes,
		Source:          distResult.Source,
		SurgeMultiplier: surge,
		BaseFare:        cfg.BaseFare,
		PricePerKm:      cfg.PricePerKm,
		TotalFare:       totalFare,
	}, nil
}

func (s *pricingServiceImpl) EstimateInternal(ctx context.Context, req *dto.EstimateRequest) (*dto.InternalEstimateResponse, error) {
	if err := s.validateCoordinates(req); err != nil {
		return nil, err
	}

	cfg, err := s.pricingConfigService.GetConfigForPricing(ctx)
	if err != nil {
		return nil, err
	}

	distResult, err := s.distanceCalculator.Calculate(ctx, *req.PickupLat, *req.PickupLng, *req.DropoffLat, *req.DropoffLng)
	if err != nil {
		return nil, err
	}

	surge, err := s.surgeService.CalculateSurge(ctx, *req.PickupLat, *req.PickupLng, cfg.SurgeThreshold)
	if err != nil {
		return nil, err
	}

	fare := util.CalculateFare(cfg.BaseFare, cfg.PricePerKm, distResult.DistanceMeters, surge)

	return &dto.InternalEstimateResponse{
		Fare:            fare,
		DistanceMeters:  distResult.DistanceMeters,
		ETASeconds:      distResult.ETASeconds,
		SurgeMultiplier: surge,
	}, nil
}
