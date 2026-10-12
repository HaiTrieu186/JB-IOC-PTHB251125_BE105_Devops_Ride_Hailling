package repository

import (
	"context"
	"time"
)

type TripLinkRepository interface {
	SetLink(ctx context.Context, driverID, tripID, customerID string, ttl time.Duration) error
	DeleteLink(ctx context.Context, driverID string) error
}
