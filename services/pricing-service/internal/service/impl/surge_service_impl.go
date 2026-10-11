package impl

import (
	"context"
	"log/slog"
	"math"
	"time"

	"pricing-service/internal/client"
	"pricing-service/internal/repository"
	"pricing-service/internal/service"
	"pricing-service/internal/util"
)

type surgeServiceImpl struct {
	demandRepo     repository.DemandRepository
	locationClient client.LocationClient
	userClient     client.UserClient
}

func NewSurgeService(
	demandRepo repository.DemandRepository,
	locationClient client.LocationClient,
	userClient client.UserClient,
) service.SurgeService {
	return &surgeServiceImpl{
		demandRepo:     demandRepo,
		locationClient: locationClient,
		userClient:     userClient,
	}
}

func (s *surgeServiceImpl) CalculateSurge(ctx context.Context, pickupLat, pickupLng float64, threshold float64) (float64, error) {
	geohash5 := util.EncodeGeohash5(pickupLat, pickupLng)

	// 1. Tính Demand
	nowUnix := time.Now().Unix()
	demand, err := s.demandRepo.GetDemandCount(ctx, geohash5, nowUnix)
	if err != nil {
		slog.Warn("failed to get demand from redis, falling back surge to 1.0", "geohash5", geohash5, "error", err)
		return 1.0, nil
	}

	// 2. Tính Supply
	driverIDs, err := s.locationClient.GetGeohashDrivers(ctx, geohash5)
	if err != nil {
		slog.Warn("failed to get geohash drivers from location-service, falling back surge to 1.0", "geohash5", geohash5, "error", err)
		return 1.0, nil
	}

	var supply int
	if len(driverIDs) > 0 {
		onlineDriverIDs, err := s.userClient.FilterOnlineDrivers(ctx, driverIDs)
		if err != nil {
			slog.Warn("failed to filter online drivers from user-service, falling back surge to 1.0", "error", err)
			return 1.0, nil
		}
		supply = len(onlineDriverIDs)
	}

	slog.Info("surge metrics calculated", "geohash5", geohash5, "demand", demand, "supply", supply, "threshold", threshold)

	// 3. Quy tắc tính Surge và Fallback (Mục 4.3 LLD & 5(j) LLD-00)
	if demand == 0 {
		return 1.0, nil
	}

	if supply == 0 && demand > 0 {
		return 1.5, nil
	}

	ratio := (float64(demand) / float64(supply)) / threshold
	rawSurge := math.Max(1.0, math.Min(1.5, ratio))
	surgeMultiplier := math.Round(rawSurge*100) / 100

	return surgeMultiplier, nil
}
