package service

import "context"

type TripSweeperService interface {
	SweepExpiredTrips(ctx context.Context) error
}
