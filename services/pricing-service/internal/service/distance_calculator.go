package service

import "context"

type DistanceResult struct {
	DistanceMeters int
	ETASeconds     int
	Source         string // "OSRM" | "HAVERSINE"
}

type DistanceCalculator interface {
	Calculate(ctx context.Context, pickupLat, pickupLng, dropoffLat, dropoffLng float64) (DistanceResult, error)
}
