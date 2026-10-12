package impl

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/redis/go-redis/v9"

	"dispatch-service/internal/dto"
	"dispatch-service/internal/repository"
)

type eventPublisherImpl struct {
	rdb *redis.Client
}

func NewEventPublisher(rdb *redis.Client) repository.EventPublisher {
	return &eventPublisherImpl{rdb: rdb}
}

func (p *eventPublisherImpl) PublishTripOffer(ctx context.Context, offer *dto.TripOfferMessage) error {
	payload, err := json.Marshal(offer)
	if err != nil {
		return err
	}
	return p.rdb.Publish(ctx, "ride:trip_offers", string(payload)).Err()
}

func (p *eventPublisherImpl) PublishTripUpdate(ctx context.Context, update *dto.TripUpdateMessage) error {
	payload, err := json.Marshal(update)
	if err != nil {
		return err
	}
	return p.rdb.Publish(ctx, "ride:trip_updates", string(payload)).Err()
}

func (p *eventPublisherImpl) PublishTripCreated(ctx context.Context, tripID, customerID, pickupGeohash5, createdAt string) error {
	args := &redis.XAddArgs{
		Stream: "stream:trip_events",
		MaxLen: 5000,
		Approx: true,
		Values: map[string]interface{}{
			"type":            "TripCreated",
			"version":         "1.0",
			"trip_id":         tripID,
			"customer_id":     customerID,
			"pickup_geohash5": pickupGeohash5,
			"created_at":      createdAt,
		},
	}
	return p.rdb.XAdd(ctx, args).Err()
}

func (p *eventPublisherImpl) PublishTripCompleted(ctx context.Context, tripID, customerID, driverID string, fare int64, completedAt string) error {
	args := &redis.XAddArgs{
		Stream: "stream:trip_events",
		MaxLen: 5000,
		Approx: true,
		Values: map[string]interface{}{
			"type":         "TripCompleted",
			"version":      "1.0",
			"trip_id":      tripID,
			"customer_id":  customerID,
			"driver_id":    driverID,
			"fare":         strconv.FormatInt(fare, 10),
			"completed_at": completedAt,
		},
	}
	return p.rdb.XAdd(ctx, args).Err()
}

func (p *eventPublisherImpl) PublishTripCancelled(ctx context.Context, tripID, cancelledAt string) error {
	args := &redis.XAddArgs{
		Stream: "stream:trip_events",
		MaxLen: 5000,
		Approx: true,
		Values: map[string]interface{}{
			"type":         "TripCancelled",
			"version":      "1.0",
			"trip_id":      tripID,
			"cancelled_at": cancelledAt,
		},
	}
	return p.rdb.XAdd(ctx, args).Err()
}

func (p *eventPublisherImpl) PublishTripExpired(ctx context.Context, tripID, reason, expiredAt string) error {
	args := &redis.XAddArgs{
		Stream: "stream:trip_events",
		MaxLen: 5000,
		Approx: true,
		Values: map[string]interface{}{
			"type":       "TripExpired",
			"version":    "1.0",
			"trip_id":    tripID,
			"reason":     reason,
			"expired_at": expiredAt,
		},
	}
	return p.rdb.XAdd(ctx, args).Err()
}
