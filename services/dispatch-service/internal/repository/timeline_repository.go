package repository

import (
	"context"

	"github.com/google/uuid"

	"dispatch-service/internal/entity"
)

type TimelineRepository interface {
	Create(ctx context.Context, timeline *entity.StatusTimeline) error
	FindByTripID(ctx context.Context, tripID uuid.UUID) ([]entity.StatusTimeline, error)
}
