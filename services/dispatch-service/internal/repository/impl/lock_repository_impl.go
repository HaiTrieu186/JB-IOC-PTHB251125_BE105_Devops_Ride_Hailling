package impl

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"dispatch-service/internal/repository"
)

type lockRepositoryImpl struct {
	rdb *redis.Client
}

func NewLockRepository(rdb *redis.Client) repository.LockRepository {
	return &lockRepositoryImpl{rdb: rdb}
}

func (r *lockRepositoryImpl) AcquireLock(ctx context.Context, tripID, driverID string, ttlSeconds int) (bool, error) {
	key := fmt.Sprintf("ride:lock:%s", tripID)
	ok, err := r.rdb.SetNX(ctx, key, driverID, time.Duration(ttlSeconds)*time.Second).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}

// ReleaseLock giải phóng distributed lock an toàn bằng Lua script compare-and-delete
// Chỉ xóa khóa khi driver_id khớp (LLD-00 Mục 5(i) và LLD Mục 6.1)
func (r *lockRepositoryImpl) ReleaseLock(ctx context.Context, tripID, driverID string) error {
	key := fmt.Sprintf("ride:lock:%s", tripID)
	luaScript := `
		if redis.call('get', KEYS[1]) == ARGV[1] then
			return redis.call('del', KEYS[1])
		else
			return 0
		end
	`
	return r.rdb.Eval(ctx, luaScript, []string{key}, driverID).Err()
}
