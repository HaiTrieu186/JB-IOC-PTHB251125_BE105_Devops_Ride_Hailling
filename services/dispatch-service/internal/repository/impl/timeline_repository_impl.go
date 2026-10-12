package impl

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"dispatch-service/internal/entity"
	"dispatch-service/internal/repository"
)

type timelineRepositoryImpl struct {
	db *gorm.DB
}

func NewTimelineRepository(db *gorm.DB) repository.TimelineRepository {
	return &timelineRepositoryImpl{db: db}
}

func (r *timelineRepositoryImpl) Create(ctx context.Context, timeline *entity.StatusTimeline) error {
	return r.db.WithContext(ctx).Create(timeline).Error
}

func (r *timelineRepositoryImpl) FindByTripID(ctx context.Context, tripID uuid.UUID) ([]entity.StatusTimeline, error) {
	var timelines []entity.StatusTimeline
	err := r.db.WithContext(ctx).
		Where("trip_id = ?", tripID).
		Order("id ASC").
		Find(&timelines).Error
	if err != nil {
		return nil, err
	}
	return timelines, nil
}
