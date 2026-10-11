package repository

import (
	"context"
)

type DemandRepository interface {
	AddTripDemand(ctx context.Context, geohash5 string, tripID string, unixCreatedAt int64) error
	GetDemandCount(ctx context.Context, geohash5 string, nowUnix int64) (int64, error)
}
