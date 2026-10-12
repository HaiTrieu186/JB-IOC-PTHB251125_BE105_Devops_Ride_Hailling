package impl

import (
	"context"

	"github.com/google/uuid"

	"dispatch-service/internal/dto"
	"dispatch-service/internal/entity"
	"dispatch-service/internal/exception"
)

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
