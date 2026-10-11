package impl

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"location-service/internal/repository"

	"github.com/redis/go-redis/v9"
)

const (
	keyDriversGeo        = "drivers:geo"
	prefixDriverLastSeen = "driver:last_seen:"
)

type locationRepositoryImpl struct {
	rdb *redis.Client
}

func NewLocationRepository(rdb *redis.Client) repository.LocationRepository {
	return &locationRepositoryImpl{rdb: rdb}
}

func (r *locationRepositoryImpl) RecordLocation(ctx context.Context, driverID string, lat, lng float64, unixNow int64, ttlSeconds int) error {
	// BẮT BUỘC: SET driver:last_seen:<driver_id> EX 30 TRƯỚC GEOADD (LLD 4.1)
	lastSeenKey := fmt.Sprintf("%s%s", prefixDriverLastSeen, driverID)
	valStr := strconv.FormatInt(unixNow, 10)

	if err := r.rdb.Set(ctx, lastSeenKey, valStr, time.Duration(ttlSeconds)*time.Second).Err(); err != nil {
		return err
	}

	geoLoc := &redis.GeoLocation{
		Name:      driverID,
		Longitude: lng,
		Latitude:  lat,
	}
	if err := r.rdb.GeoAdd(ctx, keyDriversGeo, geoLoc).Err(); err != nil {
		return err
	}

	return nil
}

func (r *locationRepositoryImpl) SearchNearby(ctx context.Context, lng, lat, radiusMeters float64, limit int) ([]repository.GeoLocationResult, error) {
	query := &redis.GeoSearchLocationQuery{
		GeoSearchQuery: redis.GeoSearchQuery{
			Longitude:  lng,
			Latitude:   lat,
			Radius:     radiusMeters,
			RadiusUnit: "m",
			Sort:       "ASC",
			Count:      limit,
		},
		WithDist: true,
	}

	locations, err := r.rdb.GeoSearchLocation(ctx, keyDriversGeo, query).Result()
	if err != nil {
		return nil, err
	}

	results := make([]repository.GeoLocationResult, len(locations))
	for i, loc := range locations {
		results[i] = repository.GeoLocationResult{
			DriverID:  loc.Name,
			Longitude: loc.Longitude,
			Latitude:  loc.Latitude,
			DistanceM: loc.Dist,
		}
	}
	return results, nil
}

func (r *locationRepositoryImpl) SearchByBox(ctx context.Context, centerLng, centerLat, boxWidthKm, boxHeightKm float64, limit int) ([]repository.GeoLocationResult, error) {
	query := &redis.GeoSearchLocationQuery{
		GeoSearchQuery: redis.GeoSearchQuery{
			Longitude: centerLng,
			Latitude:  centerLat,
			BoxWidth:  boxWidthKm,
			BoxHeight: boxHeightKm,
			BoxUnit:   "km",
			Count:     limit,
		},
		WithCoord: true,
	}

	locations, err := r.rdb.GeoSearchLocation(ctx, keyDriversGeo, query).Result()
	if err != nil {
		return nil, err
	}

	results := make([]repository.GeoLocationResult, len(locations))
	for i, loc := range locations {
		results[i] = repository.GeoLocationResult{
			DriverID:  loc.Name,
			Longitude: loc.Longitude,
			Latitude:  loc.Latitude,
			DistanceM: loc.Dist,
		}
	}
	return results, nil
}

func (r *locationRepositoryImpl) GetLastSeenMulti(ctx context.Context, driverIDs []string) (map[string]int64, []string, error) {
	if len(driverIDs) == 0 {
		return map[string]int64{}, []string{}, nil
	}

	keys := make([]string, len(driverIDs))
	for i, id := range driverIDs {
		keys[i] = fmt.Sprintf("%s%s", prefixDriverLastSeen, id)
	}

	vals, err := r.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, nil, err
	}

	foundMap := make(map[string]int64)
	var expiredIDs []string

	for i, val := range vals {
		driverID := driverIDs[i]
		if val == nil {
			expiredIDs = append(expiredIDs, driverID)
			continue
		}

		valStr, ok := val.(string)
		if !ok {
			expiredIDs = append(expiredIDs, driverID)
			continue
		}

		ts, parseErr := strconv.ParseInt(valStr, 10, 64)
		if parseErr != nil {
			expiredIDs = append(expiredIDs, driverID)
			continue
		}

		foundMap[driverID] = ts
	}

	return foundMap, expiredIDs, nil
}

func (r *locationRepositoryImpl) RemoveDriversGeo(ctx context.Context, driverIDs []string) error {
	if len(driverIDs) == 0 {
		return nil
	}

	members := make([]interface{}, len(driverIDs))
	for i, id := range driverIDs {
		members[i] = id
	}

	return r.rdb.ZRem(ctx, keyDriversGeo, members...).Err()
}

func (r *locationRepositoryImpl) GetAllDriverIDs(ctx context.Context) ([]string, error) {
	return r.rdb.ZRange(ctx, keyDriversGeo, 0, -1).Result()
}

func (r *locationRepositoryImpl) GetDriverPositions(ctx context.Context, driverIDs []string) (map[string][2]float64, error) {
	if len(driverIDs) == 0 {
		return map[string][2]float64{}, nil
	}

	positions, err := r.rdb.GeoPos(ctx, keyDriversGeo, driverIDs...).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[string][2]float64)
	for i, pos := range positions {
		if pos != nil {
			result[driverIDs[i]] = [2]float64{pos.Latitude, pos.Longitude}
		}
	}
	return result, nil
}
