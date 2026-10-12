package repository

import (
	"context"

	"dispatch-service/internal/entity"
)

type Transactor interface {
	InsertTripWithTimeline(ctx context.Context, trip *entity.Trip, matchingTimeoutSeconds int) error
}
