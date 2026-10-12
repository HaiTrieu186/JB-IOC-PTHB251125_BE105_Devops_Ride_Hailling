package impl

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"dispatch-service/internal/client"
	"dispatch-service/internal/dto"
	"dispatch-service/internal/entity"
	"dispatch-service/internal/exception"
	"dispatch-service/internal/repository"
	"dispatch-service/internal/service"
)

type tripProgressServiceImpl struct {
	tripRepo       repository.TripRepository
	timelineRepo   repository.TimelineRepository
	tripLinkRepo   repository.TripLinkRepository
	eventPublisher repository.EventPublisher
	userClient     client.UserClient
}

func NewTripProgressService(
	tripRepo repository.TripRepository,
	timelineRepo repository.TimelineRepository,
	tripLinkRepo repository.TripLinkRepository,
	eventPublisher repository.EventPublisher,
	userClient client.UserClient,
) service.TripProgressService {
	return &tripProgressServiceImpl{
		tripRepo:       tripRepo,
		timelineRepo:   timelineRepo,
		tripLinkRepo:   tripLinkRepo,
		eventPublisher: eventPublisher,
		userClient:     userClient,
	}
}

func (s *tripProgressServiceImpl) PickingUp(ctx context.Context, driverID, tripID uuid.UUID) (*dto.StatusResponse, error) {
	customerID, rowsAffected, err := s.tripRepo.UpdateToPickingUp(ctx, tripID, driverID)
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi cập nhật trạng thái đón khách", err)
	}

	if rowsAffected == 0 {
		trip, err := s.tripRepo.FindByID(ctx, tripID)
		if err != nil {
			return nil, exception.NewServiceUnavailableError("Lỗi truy vấn chuyến xe", err)
		}
		if trip == nil {
			return nil, exception.NewTripNotFoundError("Không tìm thấy chuyến xe")
		}
		if trip.DriverID == nil || *trip.DriverID != driverID {
			return nil, exception.NewForbiddenError("Bạn không phải tài xế của chuyến xe này")
		}
		return nil, exception.NewInvalidTripStatusError("Chuyến xe không ở trạng thái ACCEPTED để đón khách")
	}

	now := time.Now().UTC()
	nowUTC := dto.FormatTimeUTC(now)

	// Best-effort sau commit
	if err := s.timelineRepo.Create(ctx, &entity.StatusTimeline{
		TripID:    tripID,
		Status:    "PICKING_UP",
		Note:      nil,
		CreatedAt: now,
	}); err != nil {
		slog.Error("failed to create status_timeline for PICKING_UP", "trip_id", tripID, "error", err)
	}

	driverIDStr := driverID.String()
	if customerID != nil {
		if err := s.eventPublisher.PublishTripUpdate(ctx, &dto.TripUpdateMessage{
			TripID:     tripID.String(),
			CustomerID: customerID.String(),
			DriverID:   &driverIDStr,
			Status:     "PICKING_UP",
			UpdatedAt:  nowUTC,
		}); err != nil {
			slog.Error("failed to publish ride:trip_updates for PICKING_UP", "trip_id", tripID, "error", err)
		}
	}

	return &dto.StatusResponse{ID: tripID, Status: "PICKING_UP"}, nil
}

func (s *tripProgressServiceImpl) StartTrip(ctx context.Context, driverID, tripID uuid.UUID) (*dto.StatusResponse, error) {
	customerID, rowsAffected, err := s.tripRepo.UpdateToInTrip(ctx, tripID, driverID)
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi cập nhật trạng thái bắt đầu chuyến", err)
	}

	if rowsAffected == 0 {
		trip, err := s.tripRepo.FindByID(ctx, tripID)
		if err != nil {
			return nil, exception.NewServiceUnavailableError("Lỗi truy vấn chuyến xe", err)
		}
		if trip == nil {
			return nil, exception.NewTripNotFoundError("Không tìm thấy chuyến xe")
		}
		if trip.DriverID == nil || *trip.DriverID != driverID {
			return nil, exception.NewForbiddenError("Bạn không phải tài xế của chuyến xe này")
		}
		return nil, exception.NewInvalidTripStatusError("Chuyến xe không ở trạng thái PICKING_UP để bắt đầu")
	}

	now := time.Now().UTC()
	nowUTC := dto.FormatTimeUTC(now)

	// Best-effort sau commit
	if err := s.timelineRepo.Create(ctx, &entity.StatusTimeline{
		TripID:    tripID,
		Status:    "IN_TRIP",
		Note:      nil,
		CreatedAt: now,
	}); err != nil {
		slog.Error("failed to create status_timeline for IN_TRIP", "trip_id", tripID, "error", err)
	}

	driverIDStr := driverID.String()
	if customerID != nil {
		if err := s.eventPublisher.PublishTripUpdate(ctx, &dto.TripUpdateMessage{
			TripID:     tripID.String(),
			CustomerID: customerID.String(),
			DriverID:   &driverIDStr,
			Status:     "IN_TRIP",
			UpdatedAt:  nowUTC,
		}); err != nil {
			slog.Error("failed to publish ride:trip_updates for IN_TRIP", "trip_id", tripID, "error", err)
		}
	}

	return &dto.StatusResponse{ID: tripID, Status: "IN_TRIP"}, nil
}

func (s *tripProgressServiceImpl) CompleteTrip(ctx context.Context, driverID, tripID uuid.UUID) (*dto.CompleteTripResponse, error) {
	customerID, fare, completedAt, rowsAffected, err := s.tripRepo.UpdateToCompleted(ctx, tripID, driverID)
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi cập nhật hoàn thành chuyến xe", err)
	}

	if rowsAffected == 0 {
		trip, err := s.tripRepo.FindByID(ctx, tripID)
		if err != nil {
			return nil, exception.NewServiceUnavailableError("Lỗi truy vấn chuyến xe", err)
		}
		if trip == nil {
			return nil, exception.NewTripNotFoundError("Không tìm thấy chuyến xe")
		}
		if trip.DriverID == nil || *trip.DriverID != driverID {
			return nil, exception.NewForbiddenError("Bạn không phải tài xế của chuyến xe này")
		}
		return nil, exception.NewInvalidTripStatusError("Chuyến xe không ở trạng thái IN_TRIP để hoàn thành")
	}

	completedAtUTC := dto.FormatTimeUTC(completedAt)

	// Best-effort sau commit:
	// 1. Ghi timeline COMPLETED
	if err := s.timelineRepo.Create(ctx, &entity.StatusTimeline{
		TripID:    tripID,
		Status:    "COMPLETED",
		Note:      nil,
		CreatedAt: completedAt,
	}); err != nil {
		slog.Error("failed to create status_timeline for COMPLETED", "trip_id", tripID, "error", err)
	}

	// 2. XADD stream:trip_events TripCompleted (không có hoa hồng)
	driverIDStr := driverID.String()
	cidStr := ""
	if customerID != nil {
		cidStr = customerID.String()
	}
	if err := s.eventPublisher.PublishTripCompleted(ctx, tripID.String(), cidStr, driverIDStr, fare, completedAtUTC); err != nil {
		slog.Error("failed to publish TripCompleted to Redis Stream", "trip_id", tripID, "error", err)
	}

	// 3. DEL trip:link:<driver_id>
	if err := s.tripLinkRepo.DeleteLink(ctx, driverIDStr); err != nil {
		slog.Error("failed to delete trip:link", "driver_id", driverIDStr, "error", err)
	}

	// 4. PUBLISH ride:trip_updates
	if err := s.eventPublisher.PublishTripUpdate(ctx, &dto.TripUpdateMessage{
		TripID:     tripID.String(),
		CustomerID: cidStr,
		DriverID:   &driverIDStr,
		Status:     "COMPLETED",
		UpdatedAt:  completedAtUTC,
	}); err != nil {
		slog.Error("failed to publish ride:trip_updates for COMPLETED", "trip_id", tripID, "error", err)
	}

	// 5. Giải phóng tài xế: BUSY -> ONLINE retry 3 lần
	if err := s.userClient.UpdateDriverStatusWithRetry(driverIDStr, "BUSY", "ONLINE"); err != nil {
		slog.Error("failed to revert driver BUSY to ONLINE after complete", "driver_id", driverIDStr, "error", err)
	}

	return &dto.CompleteTripResponse{
		ID:          tripID,
		Status:      "COMPLETED",
		CompletedAt: completedAtUTC,
	}, nil
}
