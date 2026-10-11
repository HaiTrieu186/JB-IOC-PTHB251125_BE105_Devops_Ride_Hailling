package repository

import "context"

type GeoLocationResult struct {
	DriverID  string
	Longitude float64
	Latitude  float64
	DistanceM float64
}

type LocationRepository interface {
	RecordLocation(ctx context.Context, driverID string, lat, lng float64, unixNow int64, ttlSeconds int) error
	SearchNearby(ctx context.Context, lng, lat, radiusMeters float64, limit int) ([]GeoLocationResult, error)
	SearchByBox(ctx context.Context, centerLng, centerLat, boxWidthKm, boxHeightKm float64, limit int) ([]GeoLocationResult, error)
	GetLastSeenMulti(ctx context.Context, driverIDs []string) (foundMap map[string]int64, expiredIDs []string, err error)
	RemoveDriversGeo(ctx context.Context, driverIDs []string) error
	GetAllDriverIDs(ctx context.Context) ([]string, error)
	GetDriverPositions(ctx context.Context, driverIDs []string) (map[string][2]float64, error)
}
