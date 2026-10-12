package impl

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"dispatch-service/internal/dto"
	"dispatch-service/internal/repository"
)

type tripLinkRepositoryImpl struct {
	rdb *redis.Client
}

func NewTripLinkRepository(rdb *redis.Client) repository.TripLinkRepository {
	return &tripLinkRepositoryImpl{rdb: rdb}
}

func (r *tripLinkRepositoryImpl) SetLink(ctx context.Context, driverID, tripID, customerID string, ttl time.Duration) error {
	key := fmt.Sprintf("trip:link:%s", driverID)
	data := dto.TripLinkData{
		TripID:     tripID,
		CustomerID: customerID,
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return r.rdb.Set(ctx, key, string(bytes), ttl).Err()
}

func (r *tripLinkRepositoryImpl) DeleteLink(ctx context.Context, driverID string) error {
	key := fmt.Sprintf("trip:link:%s", driverID)
	return r.rdb.Del(ctx, key).Err()
}
