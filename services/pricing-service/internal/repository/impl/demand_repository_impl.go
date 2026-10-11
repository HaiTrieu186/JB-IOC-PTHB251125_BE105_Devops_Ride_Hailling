package impl

import (
	"context"
	"fmt"
	"time"

	"pricing-service/internal/repository"

	"github.com/redis/go-redis/v9"
)

type demandRepositoryImpl struct {
	rdb *redis.Client
}

func NewDemandRepository(rdb *redis.Client) repository.DemandRepository {
	return &demandRepositoryImpl{rdb: rdb}
}

func (r *demandRepositoryImpl) AddTripDemand(ctx context.Context, geohash5 string, tripID string, unixCreatedAt int64) error {
	key := fmt.Sprintf("demand:geo:%s", geohash5)

	pipe := r.rdb.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{
		Score:  float64(unixCreatedAt),
		Member: tripID,
	})
	pipe.Expire(ctx, key, 300*time.Second)

	_, err := pipe.Exec(ctx)
	return err
}

func (r *demandRepositoryImpl) GetDemandCount(ctx context.Context, geohash5 string, nowUnix int64) (int64, error) {
	key := fmt.Sprintf("demand:geo:%s", geohash5)

	// Xóa các entry cũ hơn 5 phút (nowUnix - 300)
	maxScore := fmt.Sprintf("(%d", nowUnix-300)
	if err := r.rdb.ZRemRangeByScore(ctx, key, "-inf", maxScore).Err(); err != nil {
		return 0, err
	}

	count, err := r.rdb.ZCard(ctx, key).Result()
	if err != nil {
		return 0, err
	}

	return count, nil
}
