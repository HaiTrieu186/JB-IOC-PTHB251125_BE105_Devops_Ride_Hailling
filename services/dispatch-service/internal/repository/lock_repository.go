package repository

import "context"

type LockRepository interface {
	AcquireLock(ctx context.Context, tripID, driverID string, ttlSeconds int) (bool, error)
	ReleaseLock(ctx context.Context, tripID, driverID string) error
}
