package impl

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"dispatch-service/internal/entity"
	"dispatch-service/internal/exception"
	"dispatch-service/internal/repository"
)

type transactorImpl struct {
	db *gorm.DB
}

func NewTransactor(db *gorm.DB) repository.Transactor {
	return &transactorImpl{db: db}
}

type insertTripResult struct {
	ID                uuid.UUID `gorm:"column:id"`
	MatchingExpiresAt time.Time `gorm:"column:matching_expires_at"`
	CreatedAt         time.Time `gorm:"column:created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at"`
}

func (t *transactorImpl) InsertTripWithTimeline(ctx context.Context, trip *entity.Trip, matchingTimeoutSeconds int) error {
	tx := t.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return exception.NewServiceUnavailableError("Không thể mở giao dịch database", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	// 1. INSERT trực tiếp trips với status = 'MATCHING' (Quyết định A.1)
	insertSQL := `
		INSERT INTO trips (
			customer_id, status, pickup_lat, pickup_lng, pickup_address,
			dropoff_lat, dropoff_lng, dropoff_address, fare, distance_m,
			surge_multiplier, pickup_geohash5, matching_expires_at,
			created_at, updated_at
		) VALUES (
			?, 'MATCHING', ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, now() + (? * interval '1 second'),
			now(), now()
		)
		RETURNING id, matching_expires_at, created_at, updated_at;
	`

	var res insertTripResult
	err := tx.Raw(insertSQL,
		trip.CustomerID,
		trip.PickupLat,
		trip.PickupLng,
		trip.PickupAddress,
		trip.DropoffLat,
		trip.DropoffLng,
		trip.DropoffAddress,
		trip.Fare,
		trip.DistanceM,
		trip.SurgeMultiplier,
		trip.PickupGeohash5,
		matchingTimeoutSeconds,
	).Scan(&res).Error

	if err != nil {
		tx.Rollback()
		errStr := err.Error()
		if strings.Contains(errStr, "23505") || strings.Contains(errStr, "idx_trips_active_customer") {
			return exception.NewActiveTripExistsError("Khách hàng đang có chuyến xe chưa hoàn tất")
		}
		return exception.NewServiceUnavailableError("Lỗi chèn chuyến xe vào database", err)
	}

	trip.ID = res.ID
	trip.Status = "MATCHING"
	trip.MatchingExpiresAt = res.MatchingExpiresAt
	trip.CreatedAt = res.CreatedAt
	trip.UpdatedAt = res.UpdatedAt

	// 2. Chèn 2 dòng vào status_timeline: CREATED và MATCHING (Quyết định A.1)
	timelineSQL := `
		INSERT INTO status_timeline (trip_id, status, note, created_at)
		VALUES (?, 'CREATED', NULL, ?), (?, 'MATCHING', NULL, ?);
	`
	now := trip.CreatedAt
	if err := tx.Exec(timelineSQL, trip.ID, now, trip.ID, now).Error; err != nil {
		tx.Rollback()
		return exception.NewServiceUnavailableError("Lỗi ghi lịch sử trạng thái chuyến xe", err)
	}

	if err := tx.Commit().Error; err != nil {
		return exception.NewServiceUnavailableError("Không thể commit giao dịch database", err)
	}

	return nil
}
