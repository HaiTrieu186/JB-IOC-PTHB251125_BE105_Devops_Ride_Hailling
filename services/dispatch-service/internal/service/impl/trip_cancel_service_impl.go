package impl

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"dispatch-service/internal/dto"
	"dispatch-service/internal/entity"
	"dispatch-service/internal/exception"
)

func (s *tripProgressServiceImpl) CancelTrip(ctx context.Context, userID uuid.UUID, role string, tripID uuid.UUID, reason *string) (*dto.StatusResponse, error) {
	var driverIDRet *uuid.UUID
	var customerIDRet *uuid.UUID
	var rowsAffected int64
	var err error

	if role == "CUSTOMER" {
		driverIDRet, customerIDRet, rowsAffected, err = s.tripRepo.CancelByCustomer(ctx, tripID, userID, reason)
	} else if role == "DRIVER" {
		driverIDRet, customerIDRet, rowsAffected, err = s.tripRepo.CancelByDriver(ctx, tripID, userID, reason)
	} else {
		return nil, exception.NewForbiddenError("Vai trò không hợp lệ để hủy chuyến xe")
	}

	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi hủy chuyến xe trong database", err)
	}

	if rowsAffected == 0 {
		trip, err := s.tripRepo.FindByID(ctx, tripID)
		if err != nil {
			return nil, exception.NewServiceUnavailableError("Lỗi truy vấn chuyến xe", err)
		}
		if trip == nil {
			return nil, exception.NewTripNotFoundError("Không tìm thấy chuyến xe")
		}
		if role == "CUSTOMER" && trip.CustomerID != userID {
			return nil, exception.NewForbiddenError("Bạn không phải khách hàng của chuyến xe này")
		}
		if role == "DRIVER" && (trip.DriverID == nil || *trip.DriverID != userID) {
			return nil, exception.NewForbiddenError("Bạn không phải tài xế của chuyến xe này")
		}
		return nil, exception.NewInvalidTripStatusError("Chuyến xe không ở trạng thái hợp lệ để hủy")
	}

	now := time.Now().UTC()
	nowUTC := dto.FormatTimeUTC(now)

	// Best-effort sau commit
	// 1. Ghi status_timeline
	if err := s.timelineRepo.Create(ctx, &entity.StatusTimeline{
		TripID:    tripID,
		Status:    "CANCELLED",
		Note:      reason,
		CreatedAt: now,
	}); err != nil {
		slog.Error("failed to create status_timeline for CANCELLED", "trip_id", tripID, "error", err)
	}

	// 2. XADD stream:trip_events TripCancelled (không có cancelled_by)
	if err := s.eventPublisher.PublishTripCancelled(ctx, tripID.String(), nowUTC); err != nil {
		slog.Error("failed to publish TripCancelled to Redis Stream", "trip_id", tripID, "error", err)
	}

	// 3. PUBLISH ride:trip_updates
	var driverIDStrPtr *string
	if driverIDRet != nil {
		dStr := driverIDRet.String()
		driverIDStrPtr = &dStr
	}
	cidStr := ""
	if customerIDRet != nil {
		cidStr = customerIDRet.String()
	}
	if err := s.eventPublisher.PublishTripUpdate(ctx, &dto.TripUpdateMessage{
		TripID:     tripID.String(),
		CustomerID: cidStr,
		DriverID:   driverIDStrPtr,
		Status:     "CANCELLED",
		UpdatedAt:  nowUTC,
	}); err != nil {
		slog.Error("failed to publish ride:trip_updates for CANCELLED", "trip_id", tripID, "error", err)
	}

	// 4. Nếu có driver_id: DEL trip:link và chuyển BUSY -> ONLINE
	if driverIDRet != nil {
		did := driverIDRet.String()
		_ = s.tripLinkRepo.DeleteLink(ctx, did)
		if err := s.userClient.UpdateDriverStatusWithRetry(did, "BUSY", "ONLINE"); err != nil {
			slog.Error("failed to revert driver BUSY to ONLINE after cancel", "driver_id", did, "error", err)
		}
	}

	return &dto.StatusResponse{ID: tripID, Status: "CANCELLED"}, nil
}

func (s *tripProgressServiceImpl) ForceCancelTrip(ctx context.Context, tripID uuid.UUID, reason string) (*dto.ForceCancelResponse, error) {
	reasonTrim := strings.TrimSpace(reason)
	if reasonTrim == "" {
		return nil, exception.NewValidationError("Lý do hủy bắt buộc đối với quản trị viên")
	}

	driverIDRet, customerIDRet, rowsAffected, err := s.tripRepo.CancelByAdmin(ctx, tripID, reasonTrim)
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi force-cancel chuyến xe trong database", err)
	}

	if rowsAffected == 0 {
		trip, err := s.tripRepo.FindByID(ctx, tripID)
		if err != nil {
			return nil, exception.NewServiceUnavailableError("Lỗi truy vấn chuyến xe", err)
		}
		if trip == nil {
			return nil, exception.NewTripNotFoundError("Không tìm thấy chuyến xe")
		}
		return nil, exception.NewInvalidTripStatusError("Chuyến xe đã kết thúc, không thể hủy cưỡng bức")
	}

	now := time.Now().UTC()
	nowUTC := dto.FormatTimeUTC(now)
	note := "ADMIN_FORCE_CANCEL"

	// 1. Ghi status_timeline
	if err := s.timelineRepo.Create(ctx, &entity.StatusTimeline{
		TripID:    tripID,
		Status:    "CANCELLED",
		Note:      &note,
		CreatedAt: now,
	}); err != nil {
		slog.Error("failed to create status_timeline for ADMIN force-cancel", "trip_id", tripID, "error", err)
	}

	// 2. XADD stream:trip_events TripCancelled
	if err := s.eventPublisher.PublishTripCancelled(ctx, tripID.String(), nowUTC); err != nil {
		slog.Error("failed to publish TripCancelled to Redis Stream", "trip_id", tripID, "error", err)
	}

	// 3. PUBLISH ride:trip_updates
	var driverIDStrPtr *string
	if driverIDRet != nil {
		dStr := driverIDRet.String()
		driverIDStrPtr = &dStr
	}
	cidStr := ""
	if customerIDRet != nil {
		cidStr = customerIDRet.String()
	}
	if err := s.eventPublisher.PublishTripUpdate(ctx, &dto.TripUpdateMessage{
		TripID:     tripID.String(),
		CustomerID: cidStr,
		DriverID:   driverIDStrPtr,
		Status:     "CANCELLED",
		UpdatedAt:  nowUTC,
	}); err != nil {
		slog.Error("failed to publish ride:trip_updates for force-cancel", "trip_id", tripID, "error", err)
	}

	// 4. Nếu có driver_id: DEL trip:link và chuyển BUSY -> ONLINE
	if driverIDRet != nil {
		did := driverIDRet.String()
		_ = s.tripLinkRepo.DeleteLink(ctx, did)
		if err := s.userClient.UpdateDriverStatusWithRetry(did, "BUSY", "ONLINE"); err != nil {
			slog.Error("failed to revert driver BUSY to ONLINE after force-cancel", "driver_id", did, "error", err)
		}
	}

	return &dto.ForceCancelResponse{
		ID:          tripID,
		Status:      "CANCELLED",
		CancelledBy: "ADMIN",
	}, nil
}
