package impl

import (
	"context"
	"log/slog"
	"strings"
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

func (s *tripProgressServiceImpl) GetCurrentTrip(ctx context.Context, userID uuid.UUID, role string) (*dto.TripResponse, error) {
	var trip *entity.Trip
	var err error

	if role == "CUSTOMER" {
		trip, err = s.tripRepo.FindActiveByCustomer(ctx, userID)
	} else if role == "DRIVER" {
		trip, err = s.tripRepo.FindActiveByDriver(ctx, userID)
	} else {
		return nil, exception.NewForbiddenError("Vai trò không hợp lệ")
	}

	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi truy vấn chuyến xe hiện tại", err)
	}
	if trip == nil {
		return nil, nil
	}

	res := mapTripToDTO(trip)
	return &res, nil
}

func (s *tripProgressServiceImpl) GetTripDetail(ctx context.Context, userID uuid.UUID, role string, tripID uuid.UUID) (*dto.TripDetailResponse, error) {
	trip, err := s.tripRepo.FindByID(ctx, tripID)
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi truy vấn chi tiết chuyến xe", err)
	}
	if trip == nil {
		return nil, exception.NewTripNotFoundError("Không tìm thấy chuyến xe")
	}

	if role == "CUSTOMER" && trip.CustomerID != userID {
		return nil, exception.NewForbiddenError("Bạn không có quyền xem thông tin chuyến xe này")
	}
	if role == "DRIVER" && (trip.DriverID == nil || *trip.DriverID != userID) {
		return nil, exception.NewForbiddenError("Bạn không có quyền xem thông tin chuyến xe này")
	}

	timelines, err := s.timelineRepo.FindByTripID(ctx, tripID)
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi truy vấn lịch sử trạng thái chuyến xe", err)
	}

	timelineDTOs := make([]dto.TimelineResponse, len(timelines))
	for i, tl := range timelines {
		timelineDTOs[i] = dto.TimelineResponse{
			ID:        tl.ID,
			TripID:    tl.TripID.String(),
			Status:    tl.Status,
			Note:      tl.Note,
			CreatedAt: dto.FormatTimeUTC(tl.CreatedAt),
		}
	}

	return &dto.TripDetailResponse{
		Trip:           mapTripToDTO(trip),
		StatusTimeline: timelineDTOs,
	}, nil
}

func (s *tripProgressServiceImpl) ListTripsAdmin(ctx context.Context, status string, page, limit int) (*dto.AdminTripsResponse, error) {
	validStatuses := map[string]bool{
		"CREATED":    true,
		"MATCHING":   true,
		"ACCEPTED":   true,
		"PICKING_UP": true,
		"IN_TRIP":    true,
		"COMPLETED":  true,
		"CANCELLED":  true,
		"EXPIRED":    true,
	}

	if status != "" && !validStatuses[status] {
		return nil, exception.NewValidationError("Trạng thái chuyến xe cần lọc không hợp lệ")
	}

	if page < 1 {
		return nil, exception.NewValidationError("Page phải lớn hơn hoặc bằng 1")
	}
	if limit < 1 || limit > 100 {
		return nil, exception.NewValidationError("Limit phải nằm trong khoảng từ 1 đến 100")
	}

	trips, total, err := s.tripRepo.ListTripsAdmin(ctx, status, page, limit)
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi truy vấn danh sách chuyến xe của quản trị viên", err)
	}

	tripDTOs := make([]dto.TripResponse, len(trips))
	for i, t := range trips {
		tripDTOs[i] = mapTripToDTO(&t)
	}

	return &dto.AdminTripsResponse{
		Trips: tripDTOs,
		Total: total,
		Page:  page,
	}, nil
}

func mapTripToDTO(trip *entity.Trip) dto.TripResponse {
	return dto.TripResponse{
		ID:                trip.ID,
		CustomerID:        trip.CustomerID,
		DriverID:          trip.DriverID,
		Status:            trip.Status,
		PickupLat:         trip.PickupLat,
		PickupLng:         trip.PickupLng,
		PickupAddress:     trip.PickupAddress,
		DropoffLat:        trip.DropoffLat,
		DropoffLng:        trip.DropoffLng,
		DropoffAddress:    trip.DropoffAddress,
		Fare:              trip.Fare,
		DistanceM:         trip.DistanceM,
		SurgeMultiplier:   trip.SurgeMultiplier,
		PickupGeohash5:    trip.PickupGeohash5,
		MatchingExpiresAt: dto.FormatTimeUTC(trip.MatchingExpiresAt),
		CompletedAt:       dto.FormatTimeUTCPtr(trip.CompletedAt),
		CancelledBy:       trip.CancelledBy,
		CancelReason:      trip.CancelReason,
		CreatedAt:         dto.FormatTimeUTC(trip.CreatedAt),
		UpdatedAt:         dto.FormatTimeUTC(trip.UpdatedAt),
	}
}
