package impl

import (
	"context"
	"encoding/json"
	"fmt"
	"ws-gateway/internal/repository"

	"github.com/redis/go-redis/v9"
)

type redisTripLinkRepository struct {
	rdb *redis.Client
}

func NewRedisTripLinkRepository(rdb *redis.Client) repository.TripLinkReader {
	return &redisTripLinkRepository{rdb: rdb}
}

func (r *redisTripLinkRepository) GetTripLink(ctx context.Context, driverID string) (*repository.TripLink, error) {
	val, err := r.rdb.Get(ctx, fmt.Sprintf("trip:link:%s", driverID)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var link repository.TripLink
	if err := json.Unmarshal([]byte(val), &link); err != nil {
		return nil, err
	}
	return &link, nil
}
