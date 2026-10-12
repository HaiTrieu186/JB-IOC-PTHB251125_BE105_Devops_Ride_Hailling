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

type tripAcceptServiceImpl struct {
	tripRepo       repository.TripRepository
	timelineRepo   repository.TimelineRepository
	lockRepo       repository.LockRepository
	tripLinkRepo   repository.TripLinkRepository
	eventPublisher repository.EventPublisher
	userClient     client.UserClient
}

func NewTripAcceptService(
	tripRepo repository.TripRepository,
	timelineRepo repository.TimelineRepository,
	lockRepo repository.LockRepository,
	tripLinkRepo repository.TripLinkRepository,
	eventPublisher repository.EventPublisher,
	userClient client.UserClient,
) service.TripAcceptService {
	return &tripAcceptServiceImpl{
		tripRepo:       tripRepo,
		timelineRepo:   timelineRepo,
		lockRepo:       lockRepo,
		tripLinkRepo:   tripLinkRepo,
		eventPublisher: eventPublisher,
		userClient:     userClient,
	}
}

func (s *tripAcceptServiceImpl) AcceptTrip(ctx context.Context, driverID, tripID uuid.UUID) (*dto.AcceptTripResponse, error) {
	// (1) Kiểm tra nhanh theo thứ tự quy định tại Quyết định B:
	// chuyến không tồn tại -> 404 TRIP_NOT_FOUND
	// status = ACCEPTED và driver_id == người gọi -> 200 idempotent
	// người gọi không thuộc invited_drivers -> 403 NOT_OFFERED
	// status = ACCEPTED bởi người khác -> 409 TRIP_ALREADY_TAKEN
	// status khác MATCHING -> 400 INVALID_TRIP_STATUS
	trip, err := s.tripRepo.FindByID(ctx, tripID)
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi truy vấn chuyến xe từ database", err)
	}
	if trip == nil {
		return nil, exception.NewTripNotFoundError("Không tìm thấy chuyến xe")
	}

	if trip.Status == "ACCEPTED" && trip.DriverID != nil && *trip.DriverID == driverID {
		return &dto.AcceptTripResponse{
			ID:       trip.ID,
			Status:   "ACCEPTED",
			DriverID: driverID,
		}, nil
	}

	isOffered := false
	driverIDStr := driverID.String()
	for _, invited := range trip.InvitedDrivers {
		if invited.DriverID == driverIDStr {
			isOffered = true
			break
		}
	}
	if !isOffered {
		return nil, exception.NewNotOfferedError("Tài xế không nằm trong danh sách được mời nhận chuyến xe này")
	}

	if trip.Status == "ACCEPTED" {
		return nil, exception.NewTripAlreadyTakenError("Chuyến xe đã được tài xế khác nhận")
	}

	if trip.Status != "MATCHING" {
		return nil, exception.NewInvalidTripStatusError("Trạng thái chuyến xe không hợp lệ để nhận cuốc")
	}

	// (2) Khóa phân tán: SET ride:lock:<trip_id> <driver_id> NX EX 10
	ok, err := s.lockRepo.AcquireLock(ctx, tripID.String(), driverID.String(), 10)
	if err != nil {
		// Quyết định B: Lỗi Redis ở SET NX -> trả 503 SERVICE_UNAVAILABLE (chưa có gì để hoàn tác)
		slog.Error("failed to acquire lock in Redis", "trip_id", tripID, "driver_id", driverID, "error", err)
		return nil, exception.NewServiceUnavailableError("Lỗi hệ thống khóa phân tán Redis", err)
	}
	if !ok {
		// Thua lock
		return nil, exception.NewTripAlreadyTakenError("Chuyến xe đã được tài xế khác nhận")
	}

	// (3) Gọi nội bộ: POST /internal/v1/drivers/:id/status {from: ONLINE, to: BUSY}
	if err := s.userClient.UpdateDriverStatus(ctx, driverID.String(), "ONLINE", "BUSY"); err != nil {
		// Hoàn tác: nhả lock Redis bằng context MỚI (Quyết định B)
		s.releaseLockWithNewContext(tripID.String(), driverID.String())

		if appErr, ok := err.(*exception.AppError); ok && appErr.Code == exception.ErrCodeDriverNotAvailable {
			return nil, exception.NewDriverNotAvailableError("Tài xế không ở trạng thái sẵn sàng để nhận chuyến")
		}
		return nil, exception.NewServiceUnavailableError("user-service không phản hồi hoặc timeout khi đổi trạng thái", err)
	}

	// (4) UPDATE trips SET status='ACCEPTED', driver_id=A WHERE id=trip_id AND status='MATCHING' AND matching_expires_at > now()
	rowsAffected, err := s.tripRepo.UpdateToAccepted(ctx, tripID, driverID)
	if err != nil {
		// Quyết định B: Lỗi DB (không phải 0 dòng) ở bước 4: hoàn tác y như trường hợp 0 dòng (BUSY->ONLINE rồi nhả lock bằng Lua), trả 503
		slog.Error("database error during accept update", "trip_id", tripID, "driver_id", driverID, "error", err)
		s.rollbackDriverAndLock(driverID.String(), tripID.String())
		return nil, exception.NewServiceUnavailableError("Lỗi cập nhật nhận chuyến trong database", err)
	}

	if rowsAffected == 0 {
		// 0 dòng affected: xung đột trạng thái / hết hạn
		s.rollbackDriverAndLock(driverID.String(), tripID.String())

		latestTrip, _ := s.tripRepo.FindByID(ctx, tripID)
		if latestTrip != nil && latestTrip.Status == "ACCEPTED" {
			return nil, exception.NewTripAlreadyTakenError("Chuyến xe đã được tài xế khác nhận")
		}
		return nil, exception.NewInvalidTripStatusError("Chuyến xe không còn ở trạng thái MATCHING hoặc đã hết hạn")
	}

	// 1 dòng affected: Thành công! Best-effort sau commit:
	now := time.Now().UTC()
	nowUTC := dto.FormatTimeUTC(now)

	// 1. Ghi status_timeline
	if err := s.timelineRepo.Create(ctx, &entity.StatusTimeline{
		TripID:    tripID,
		Status:    "ACCEPTED",
		Note:      nil,
		CreatedAt: now,
	}); err != nil {
		slog.Error("failed to create status_timeline for ACCEPTED", "trip_id", tripID, "error", err)
	}

	// 2. Ghi trip:link:<driver_id> với TTL 3 giờ (10800s)
	if err := s.tripLinkRepo.SetLink(ctx, driverID.String(), tripID.String(), trip.CustomerID.String(), 3*time.Hour); err != nil {
		slog.Error("failed to set trip:link", "trip_id", tripID, "driver_id", driverID, "error", err)
	}

	// 3. PUBLISH ride:trip_updates
	driverIDVal := driverID.String()
	if err := s.eventPublisher.PublishTripUpdate(ctx, &dto.TripUpdateMessage{
		TripID:     tripID.String(),
		CustomerID: trip.CustomerID.String(),
		DriverID:   &driverIDVal,
		Status:     "ACCEPTED",
		UpdatedAt:  nowUTC,
	}); err != nil {
		slog.Error("failed to publish ride:trip_updates for ACCEPTED", "trip_id", tripID, "error", err)
	}

	return &dto.AcceptTripResponse{
		ID:       tripID,
		Status:   "ACCEPTED",
		DriverID: driverID,
	}, nil
}

func (s *tripAcceptServiceImpl) releaseLockWithNewContext(tripID, driverID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.lockRepo.ReleaseLock(ctx, tripID, driverID); err != nil {
		slog.Error("failed to release lock in cleanup", "trip_id", tripID, "driver_id", driverID, "error", err)
	}
}

func (s *tripAcceptServiceImpl) rollbackDriverAndLock(driverID, tripID string) {
	// 1. BUSY -> ONLINE retry 3 lần
	if err := s.userClient.UpdateDriverStatusWithRetry(driverID, "BUSY", "ONLINE"); err != nil {
		slog.Error("failed to revert driver BUSY to ONLINE during accept rollback", "driver_id", driverID, "error", err)
	}

	// 2. Nhả lock Redis
	s.releaseLockWithNewContext(tripID, driverID)
}
