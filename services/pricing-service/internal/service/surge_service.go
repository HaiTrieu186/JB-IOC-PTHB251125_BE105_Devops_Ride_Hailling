package service

import "context"

type SurgeService interface {
	CalculateSurge(ctx context.Context, pickupLat, pickupLng float64, threshold float64) (float64, error)
}
