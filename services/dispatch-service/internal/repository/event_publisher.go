package repository

import (
	"context"

	"dispatch-service/internal/dto"
)

type EventPublisher interface {
	PublishTripOffer(ctx context.Context, offer *dto.TripOfferMessage) error
	PublishTripUpdate(ctx context.Context, update *dto.TripUpdateMessage) error

	PublishTripCreated(ctx context.Context, tripID, customerID, pickupGeohash5, createdAt string) error
	PublishTripCompleted(ctx context.Context, tripID, customerID, driverID string, fare int64, completedAt string) error
	PublishTripCancelled(ctx context.Context, tripID, cancelledAt string) error
	PublishTripExpired(ctx context.Context, tripID, reason, expiredAt string) error
}
