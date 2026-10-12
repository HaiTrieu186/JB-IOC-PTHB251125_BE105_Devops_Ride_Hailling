package impl

import (
	"context"
	"log/slog"
	"time"

	"dispatch-service/internal/dto"
	"dispatch-service/internal/entity"
	"dispatch-service/internal/repository"
	"dispatch-service/internal/service"
)

type tripSweeperServiceImpl struct {
	tripRepo       repository.TripRepository
	timelineRepo   repository.TimelineRepository
	eventPublisher repository.EventPublisher
}

func NewTripSweeperService(
	tripRepo repository.TripRepository,
	timelineRepo repository.TimelineRepository,
	eventPublisher repository.EventPublisher,
) service.TripSweeperService {
	return &tripSweeperServiceImpl{
		tripRepo:       tripRepo,
		timelineRepo:   timelineRepo,
		eventPublisher: eventPublisher,
	}
}

func (s *tripSweeperServiceImpl) SweepExpiredTrips(ctx context.Context) error {
	expiredList, err := s.tripRepo.ExpireStaleTrips(ctx)
	if err != nil {
		slog.Error("failed to sweep expired trips in DB", "error", err)
		return err
	}

	if len(expiredList) == 0 {
		return nil
	}

	slog.Info("sweeper found and expired trips", "count", len(expiredList))
	now := time.Now().UTC()
	nowUTC := dto.FormatTimeUTC(now)
	note := "TIMEOUT_30S"

	for _, item := range expiredList {
		// 1. Ghi nhận status_timeline
		if err := s.timelineRepo.Create(ctx, &entity.StatusTimeline{
			TripID:    item.ID,
			Status:    "EXPIRED",
			Note:      &note,
			CreatedAt: now,
		}); err != nil {
			slog.Error("sweeper failed to create status_timeline", "trip_id", item.ID, "error", err)
		}

		// 2. XADD stream:trip_events
		if err := s.eventPublisher.PublishTripExpired(ctx, item.ID.String(), note, nowUTC); err != nil {
			slog.Error("sweeper failed to publish TripExpired event", "trip_id", item.ID, "error", err)
		}

		// 3. PUBLISH ride:trip_updates
		if err := s.eventPublisher.PublishTripUpdate(ctx, &dto.TripUpdateMessage{
			TripID:     item.ID.String(),
			CustomerID: item.CustomerID.String(),
			DriverID:   nil,
			Status:     "EXPIRED",
			UpdatedAt:  nowUTC,
		}); err != nil {
			slog.Error("sweeper failed to publish ride:trip_updates", "trip_id", item.ID, "error", err)
		}
	}

	return nil
}
