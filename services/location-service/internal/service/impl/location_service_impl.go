package impl

import (
	"context"
	"math"
	"sort"
	"time"

	"location-service/internal/config"
	"location-service/internal/dto"
	"location-service/internal/exception"
	"location-service/internal/repository"
	"location-service/internal/service"
	"location-service/internal/util"

	"github.com/mmcloughlin/geohash"
)

type locationServiceImpl struct {
	repo repository.LocationRepository
	cfg  *config.Config
}

func NewLocationService(repo repository.LocationRepository, cfg *config.Config) service.LocationService {
	return &locationServiceImpl{
		repo: repo,
		cfg:  cfg,
	}
}

func (s *locationServiceImpl) RecordLocation(ctx context.Context, req *dto.RecordLocationRequest) *exception.AppError {
	if !util.ValidateUUID(req.DriverID) {
		return exception.NewValidationError("driver_id không hợp lệ (phải đúng định dạng UUID)")
	}
	if !util.ValidateCoordinates(req.Latitude, req.Longitude) {
		return exception.NewValidationError("Tọa độ không hợp lệ (latitude [-85.05112878, 85.05112878], longitude [-180, 180])")
	}

	unixNow := time.Now().UTC().Unix()
	// TTL 30 giây cho driver:last_seen
	if err := s.repo.RecordLocation(ctx, req.DriverID, *req.Latitude, *req.Longitude, unixNow, 30); err != nil {
		return exception.NewServiceUnavailableError("Lỗi kết nối Redis khi ghi nhận vị trí")
	}

	return nil
}

func (s *locationServiceImpl) GetCandidates(ctx context.Context, req *dto.CandidatesRequest) (*dto.CandidatesResponse, *exception.AppError) {
	if !util.ValidateCoordinates(req.PickupLat, req.PickupLng) {
		return nil, exception.NewValidationError("Tọa độ điểm đón không hợp lệ")
	}

	radius := s.cfg.DefaultRadiusKM
	if req.RadiusKM != nil {
		if *req.RadiusKM <= 0 || *req.RadiusKM > s.cfg.MaxRadiusKM {
			return nil, exception.NewValidationError("Bán kính tìm kiếm không hợp lệ (phải > 0 và <= MAX_RADIUS_KM)")
		}
		radius = *req.RadiusKM
	}

	radiusMeters := radius * 1000.0
	results, err := s.repo.SearchNearby(ctx, *req.PickupLng, *req.PickupLat, radiusMeters, s.cfg.GeoScanCount)
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi truy vấn Redis GEO")
	}

	if len(results) == 0 {
		return &dto.CandidatesResponse{Candidates: []dto.CandidateDTO{}}, nil
	}

	driverIDs := make([]string, len(results))
	distMap := make(map[string]float64, len(results))
	for i, r := range results {
		driverIDs[i] = r.DriverID
		distMap[r.DriverID] = r.DistanceM
	}

	foundMap, expiredIDs, err := s.repo.GetLastSeenMulti(ctx, driverIDs)
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi truy vấn Redis last_seen")
	}

	// Cơ chế ZREM lười dọn sạch tài xế mất sóng (key last_seen hết hạn)
	if len(expiredIDs) > 0 {
		if err := s.repo.RemoveDriversGeo(ctx, expiredIDs); err != nil {
			return nil, exception.NewServiceUnavailableError("Lỗi kết nối Redis khi dọn dẹp vị trí tài xế")
		}
	}

	now := time.Now().UTC().Unix()
	candidates := make([]dto.CandidateDTO, 0)

	for _, id := range driverIDs {
		lastSeen, exists := foundMap[id]
		if exists && (now-lastSeen) <= s.cfg.GPSFreshnessSeconds {
			distM := int64(math.Round(distMap[id]))
			candidates = append(candidates, dto.CandidateDTO{
				DriverID:  id,
				DistanceM: distM,
			})
		}
	}

	// Sắp xếp tăng dần theo khoảng cách distance_m
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].DistanceM == candidates[j].DistanceM {
			return candidates[i].DriverID < candidates[j].DriverID
		}
		return candidates[i].DistanceM < candidates[j].DistanceM
	})

	// Cắt lấy tối đa 50 ứng viên
	if len(candidates) > 50 {
		candidates = candidates[:50]
	}

	return &dto.CandidatesResponse{Candidates: candidates}, nil
}

func (s *locationServiceImpl) GetGeohashDrivers(ctx context.Context, req *dto.GeohashDriversRequest) (*dto.GeohashDriversResponse, *exception.AppError) {
	if !util.ValidateGeohash5(req.Geohash5) {
		return nil, exception.NewValidationError("Mã geohash5 không hợp lệ (phải đúng 5 ký tự)")
	}

	centerLat, centerLng := geohash.DecodeCenter(req.Geohash5)
	results, err := s.repo.SearchByBox(ctx, centerLng, centerLat, 5.5, 5.5, s.cfg.GeoScanCount)
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi truy vấn Redis GEO theo bounding box")
	}

	if len(results) == 0 {
		return &dto.GeohashDriversResponse{DriverIDs: []string{}}, nil
	}

	var matchedIDs []string
	for _, r := range results {
		enc := geohash.EncodeWithPrecision(r.Latitude, r.Longitude, 5)
		if enc == req.Geohash5 {
			matchedIDs = append(matchedIDs, r.DriverID)
		}
	}

	if len(matchedIDs) == 0 {
		return &dto.GeohashDriversResponse{DriverIDs: []string{}}, nil
	}

	foundMap, expiredIDs, err := s.repo.GetLastSeenMulti(ctx, matchedIDs)
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi truy vấn Redis last_seen")
	}

	// Cơ chế ZREM lười
	if len(expiredIDs) > 0 {
		if err := s.repo.RemoveDriversGeo(ctx, expiredIDs); err != nil {
			return nil, exception.NewServiceUnavailableError("Lỗi kết nối Redis khi dọn dẹp vị trí tài xế")
		}
	}

	now := time.Now().UTC().Unix()
	validIDs := make([]string, 0)

	for _, id := range matchedIDs {
		lastSeen, exists := foundMap[id]
		if exists && (now-lastSeen) <= s.cfg.GPSFreshnessSeconds {
			validIDs = append(validIDs, id)
		}
	}

	return &dto.GeohashDriversResponse{DriverIDs: validIDs}, nil
}

func (s *locationServiceImpl) GetActiveDrivers(ctx context.Context) (*dto.ActiveDriversResponse, *exception.AppError) {
	allIDs, err := s.repo.GetAllDriverIDs(ctx)
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi truy vấn danh sách tài xế từ Redis")
	}

	if len(allIDs) == 0 {
		return &dto.ActiveDriversResponse{Drivers: []dto.ActiveDriverDTO{}}, nil
	}

	foundMap, expiredIDs, err := s.repo.GetLastSeenMulti(ctx, allIDs)
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi truy vấn Redis last_seen")
	}

	// Cơ chế ZREM lười
	if len(expiredIDs) > 0 {
		if err := s.repo.RemoveDriversGeo(ctx, expiredIDs); err != nil {
			return nil, exception.NewServiceUnavailableError("Lỗi kết nối Redis khi dọn dẹp vị trí tài xế")
		}
	}

	now := time.Now().UTC().Unix()
	var validIDs []string
	for _, id := range allIDs {
		lastSeen, exists := foundMap[id]
		if exists && (now-lastSeen) <= s.cfg.GPSFreshnessSeconds {
			validIDs = append(validIDs, id)
		}
	}

	if len(validIDs) == 0 {
		return &dto.ActiveDriversResponse{Drivers: []dto.ActiveDriverDTO{}}, nil
	}

	posMap, err := s.repo.GetDriverPositions(ctx, validIDs)
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi truy vấn tọa độ tài xế từ Redis")
	}

	drivers := make([]dto.ActiveDriverDTO, 0, len(validIDs))
	for _, id := range validIDs {
		if coords, ok := posMap[id]; ok {
			lastSeen := foundMap[id]
			drivers = append(drivers, dto.ActiveDriverDTO{
				DriverID:  id,
				Latitude:  coords[0],
				Longitude: coords[1],
				LastSeen:  time.Unix(lastSeen, 0).UTC().Format(time.RFC3339),
			})
		}
	}

	return &dto.ActiveDriversResponse{Drivers: drivers}, nil
}
