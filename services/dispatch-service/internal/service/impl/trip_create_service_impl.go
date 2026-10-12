package impl

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"dispatch-service/internal/client"
	"dispatch-service/internal/dto"
	"dispatch-service/internal/entity"
	"dispatch-service/internal/exception"
	"dispatch-service/internal/repository"
	"dispatch-service/internal/service"
	"dispatch-service/internal/util"
)

type tripCreateServiceImpl struct {
	tripRepo               repository.TripRepository
	timelineRepo           repository.TimelineRepository
	transactor             repository.Transactor
	eventPublisher         repository.EventPublisher
	pricingClient          client.PricingClient
	paymentClient          client.PaymentClient
	locationClient         client.LocationClient
	userClient             client.UserClient
	offerRadiusKM          float64
	matchingTimeoutSeconds int
	commissionRate         int
}

func NewTripCreateService(
	tripRepo repository.TripRepository,
	timelineRepo repository.TimelineRepository,
	transactor repository.Transactor,
	eventPublisher repository.EventPublisher,
	pricingClient client.PricingClient,
	paymentClient client.PaymentClient,
	locationClient client.LocationClient,
	userClient client.UserClient,
	offerRadiusKM float64,
	matchingTimeoutSeconds int,
	commissionRate int,
) service.TripCreateService {
	return &tripCreateServiceImpl{
		tripRepo:               tripRepo,
		timelineRepo:           timelineRepo,
		transactor:             transactor,
		eventPublisher:         eventPublisher,
		pricingClient:          pricingClient,
		paymentClient:          paymentClient,
		locationClient:         locationClient,
		userClient:             userClient,
		offerRadiusKM:          offerRadiusKM,
		matchingTimeoutSeconds: matchingTimeoutSeconds,
		commissionRate:         commissionRate,
	}
}

func (s *tripCreateServiceImpl) CreateTrip(ctx context.Context, customerID uuid.UUID, req *dto.CreateTripRequest) (*dto.CreateTripResponse, error) {
	// 1. Validate dữ liệu & Role (Role đã check bởi middleware RequireCustomer)
	if err := validateCoordinates(req); err != nil {
		return nil, err
	}

	// 2. Kiểm tra nhanh chuyến active của khách
	hasActive, err := s.tripRepo.HasActiveTripByCustomer(ctx, customerID)
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi kiểm tra chuyến xe đang hoạt động", err)
	}
	if hasActive {
		return nil, exception.NewActiveTripExistsError("Khách hàng đang có chuyến xe chưa hoàn tất")
	}

	// 3. Lấy báo giá từ pricing-service
	estimate, err := s.pricingClient.Estimate(ctx, req.PickupLat, req.PickupLng, req.DropoffLat, req.DropoffLng)
	if err != nil {
		return nil, err
	}

	// 4. Kiểm tra ví khách hàng từ payment-service
	balanceRes, err := s.paymentClient.CheckBalance(ctx, customerID, estimate.Fare)
	if err != nil {
		return nil, err
	}
	if !balanceRes.Sufficient {
		return nil, exception.NewInsufficientBalanceError("Số dư ví không đủ để thực hiện chuyến đi")
	}

	// 5. Tính Geohash độ dài 5 từ tọa độ đón
	pickupGeohash5 := util.EncodeGeohash5(req.PickupLat, req.PickupLng)

	// 6. Giao dịch DB INSERT chuyến với status = MATCHING
	trip := &entity.Trip{
		CustomerID:      customerID,
		PickupLat:       req.PickupLat,
		PickupLng:       req.PickupLng,
		PickupAddress:   strings.TrimSpace(req.PickupAddress),
		DropoffLat:      req.DropoffLat,
		DropoffLng:      req.DropoffLng,
		DropoffAddress:  strings.TrimSpace(req.DropoffAddress),
		Fare:            estimate.Fare,
		DistanceM:       estimate.DistanceM,
		SurgeMultiplier: estimate.SurgeMultiplier,
		PickupGeohash5:  pickupGeohash5,
	}

	if err := s.transactor.InsertTripWithTimeline(ctx, trip, s.matchingTimeoutSeconds); err != nil {
		return nil, err
	}

	// 7. Phát sự kiện tạo chuyến vào Redis Stream (lỗi chỉ log WARN)
	if err := s.eventPublisher.PublishTripCreated(
		ctx,
		trip.ID.String(),
		trip.CustomerID.String(),
		trip.PickupGeohash5,
		dto.FormatTimeUTC(trip.CreatedAt),
	); err != nil {
		slog.Warn("failed to publish TripCreated to Redis Stream", "trip_id", trip.ID, "error", err)
	}

	// 8. Tìm kiếm & Ghép tài xế
	candidates, err := s.locationClient.GetCandidates(ctx, req.PickupLat, req.PickupLng, s.offerRadiusKM)
	if err != nil {
		slog.Error("failed to get candidates from location-service", "trip_id", trip.ID, "error", err)
		s.expireTripOnDependencyError(ctx, trip)
		return nil, exception.NewServiceUnavailableError("location-service không phản hồi hoặc timeout", err)
	}

	var matchedCandidates []client.DriverCandidate
	if len(candidates) > 0 {
		driverIDs := make([]string, len(candidates))
		for i, c := range candidates {
			driverIDs[i] = c.DriverID
		}

		onlineIDs, err := s.userClient.FilterOnline(ctx, driverIDs)
		if err != nil {
			slog.Error("failed to filter online drivers from user-service", "trip_id", trip.ID, "error", err)
			s.expireTripOnDependencyError(ctx, trip)
			return nil, exception.NewServiceUnavailableError("user-service không phản hồi hoặc timeout", err)
		}

		if len(onlineIDs) > 0 {
			onlineMap := make(map[string]bool, len(onlineIDs))
			for _, id := range onlineIDs {
				onlineMap[id] = true
			}

			for _, c := range candidates {
				if onlineMap[c.DriverID] {
					matchedCandidates = append(matchedCandidates, c)
				}
			}

			// Sắp xếp tăng dần theo khoảng cách distance_m (Quyết định D)
			sort.Slice(matchedCandidates, func(i, j int) bool {
				return matchedCandidates[i].DistanceM < matchedCandidates[j].DistanceM
			})

			// Top-N: min(số tài xế online khớp, 5) (Quyết định A.3)
			if len(matchedCandidates) > 5 {
				matchedCandidates = matchedCandidates[:5]
			}
		}
	}

	// 9. Phân nhánh kết quả
	if len(matchedCandidates) == 0 {
		// Nhánh 9a: Không có tài xế online (0 ONLINE, không lỗi)
		s.expireTripNoDrivers(ctx, trip)
		return &dto.CreateTripResponse{
			ID:                trip.ID,
			Status:            "EXPIRED",
			Fare:              trip.Fare,
			PickupAddress:     trip.PickupAddress,
			DropoffAddress:    trip.DropoffAddress,
			MatchingExpiresAt: dto.FormatTimeUTC(trip.MatchingExpiresAt),
		}, nil
	}

	// Nhánh 9b: Có tài xế (Top 1..5)
	invited := make(entity.InvitedDrivers, len(matchedCandidates))
	targets := make([]dto.TargetDriver, len(matchedCandidates))
	for i, c := range matchedCandidates {
		invited[i] = entity.InvitedDriver{
			DriverID:  c.DriverID,
			DistanceM: c.DistanceM,
		}
		targets[i] = dto.TargetDriver{
			DriverID:  c.DriverID,
			DistanceM: c.DistanceM,
		}
	}

	// Lưu invited_drivers vào trips (Quyết định A.2)
	rowsAffected, err := s.tripRepo.UpdateInvitedDrivers(ctx, trip.ID, invited)
	if err != nil {
		slog.Error("failed to update invited_drivers", "trip_id", trip.ID, "error", err)
		return nil, exception.NewServiceUnavailableError("Lỗi cập nhật danh sách mời xe", err)
	}

	if rowsAffected == 0 {
		// Khách vừa hủy hoặc sweeper vừa hết hạn trong lúc tìm xe (Quyết định A.2)
		latestTrip, err := s.tripRepo.FindByID(ctx, trip.ID)
		realStatus := "MATCHING"
		if err == nil && latestTrip != nil {
			realStatus = latestTrip.Status
		}
		return &dto.CreateTripResponse{
			ID:                trip.ID,
			Status:            realStatus,
			Fare:              trip.Fare,
			PickupAddress:     trip.PickupAddress,
			DropoffAddress:    trip.DropoffAddress,
			MatchingExpiresAt: dto.FormatTimeUTC(trip.MatchingExpiresAt),
		}, nil
	}

	// Phát Redis Pub/Sub: ride:trip_offers
	_, driverFare := util.CalculateDriverFare(trip.Fare, s.commissionRate)
	offerMsg := &dto.TripOfferMessage{
		TripID:     trip.ID.String(),
		Targets:    targets,
		PickupLat:  trip.PickupLat,
		PickupLng:  trip.PickupLng,
		Fare:       trip.Fare,
		DriverFare: driverFare,
		ExpireAt:   dto.FormatTimeUTC(trip.MatchingExpiresAt),
	}

	if err := s.eventPublisher.PublishTripOffer(ctx, offerMsg); err != nil {
		// Quyết định A.4: PUBLISH lỗi -> log ERROR, vẫn trả 201 (sweeper sẽ hết hạn sau 30s)
		slog.Error("failed to publish ride:trip_offers", "trip_id", trip.ID, "error", err)
	}

	return &dto.CreateTripResponse{
		ID:                trip.ID,
		Status:            "MATCHING",
		Fare:              trip.Fare,
		PickupAddress:     trip.PickupAddress,
		DropoffAddress:    trip.DropoffAddress,
		MatchingExpiresAt: dto.FormatTimeUTC(trip.MatchingExpiresAt),
	}, nil
}

func (s *tripCreateServiceImpl) expireTripNoDrivers(ctx context.Context, trip *entity.Trip) {
	now := time.Now().UTC()
	nowUTC := dto.FormatTimeUTC(now)
	note := "NO_DRIVERS_AVAILABLE"

	_, _ = s.tripRepo.ExpireTrip(ctx, trip.ID)
	_ = s.timelineRepo.Create(ctx, &entity.StatusTimeline{
		TripID:    trip.ID,
		Status:    "EXPIRED",
		Note:      &note,
		CreatedAt: now,
	})
	_ = s.eventPublisher.PublishTripExpired(ctx, trip.ID.String(), note, nowUTC)
	_ = s.eventPublisher.PublishTripUpdate(ctx, &dto.TripUpdateMessage{
		TripID:     trip.ID.String(),
		CustomerID: trip.CustomerID.String(),
		DriverID:   nil,
		Status:     "EXPIRED",
		UpdatedAt:  nowUTC,
	})
}

func (s *tripCreateServiceImpl) expireTripOnDependencyError(ctx context.Context, trip *entity.Trip) {
	now := time.Now().UTC()
	nowUTC := dto.FormatTimeUTC(now)
	note := "NO_DRIVERS_AVAILABLE"

	_, _ = s.tripRepo.ExpireTrip(ctx, trip.ID)
	_ = s.timelineRepo.Create(ctx, &entity.StatusTimeline{
		TripID:    trip.ID,
		Status:    "EXPIRED",
		Note:      &note,
		CreatedAt: now,
	})
	_ = s.eventPublisher.PublishTripExpired(ctx, trip.ID.String(), note, nowUTC)
	_ = s.eventPublisher.PublishTripUpdate(ctx, &dto.TripUpdateMessage{
		TripID:     trip.ID.String(),
		CustomerID: trip.CustomerID.String(),
		DriverID:   nil,
		Status:     "EXPIRED",
		UpdatedAt:  nowUTC,
	})
}

func validateCoordinates(req *dto.CreateTripRequest) error {
	// Dải chuẩn LLD-00 Mục 4: vĩ độ [-85.05112878, 85.05112878], kinh độ [-180, 180]
	const minLat = -85.05112878
	const maxLat = 85.05112878
	const minLng = -180.0
	const maxLng = 180.0

	if req.PickupLat < minLat || req.PickupLat > maxLat || req.PickupLng < minLng || req.PickupLng > maxLng {
		return exception.NewValidationError("Tọa độ điểm đón không hợp lệ")
	}
	if req.DropoffLat < minLat || req.DropoffLat > maxLat || req.DropoffLng < minLng || req.DropoffLng > maxLng {
		return exception.NewValidationError("Tọa độ điểm trả không hợp lệ")
	}

	if req.PickupLat == req.DropoffLat && req.PickupLng == req.DropoffLng {
		return exception.NewValidationError("Điểm đón và điểm trả không được trùng nhau")
	}

	if strings.TrimSpace(req.PickupAddress) == "" {
		return exception.NewValidationError("Địa chỉ đón không được để trống")
	}
	if strings.TrimSpace(req.DropoffAddress) == "" {
		return exception.NewValidationError("Địa chỉ trả không được để trống")
	}

	return nil
}
