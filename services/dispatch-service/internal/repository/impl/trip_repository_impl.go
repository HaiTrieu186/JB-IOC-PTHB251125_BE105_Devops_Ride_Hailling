package impl

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"dispatch-service/internal/entity"
	"dispatch-service/internal/repository"
)

type tripRepositoryImpl struct {
	db *gorm.DB
}

func NewTripRepository(db *gorm.DB) repository.TripRepository {
	return &tripRepositoryImpl{db: db}
}

func (r *tripRepositoryImpl) HasActiveTripByCustomer(ctx context.Context, customerID uuid.UUID) (bool, error) {
	var count int64
	query := `
		SELECT COUNT(1) FROM trips
		WHERE customer_id = ? AND status IN ('CREATED', 'MATCHING', 'ACCEPTED', 'PICKING_UP', 'IN_TRIP')
		LIMIT 1;
	`
	err := r.db.WithContext(ctx).Raw(query, customerID).Scan(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *tripRepositoryImpl) FindByID(ctx context.Context, tripID uuid.UUID) (*entity.Trip, error) {
	var trip entity.Trip
	err := r.db.WithContext(ctx).First(&trip, "id = ?", tripID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &trip, nil
}

func (r *tripRepositoryImpl) FindActiveByCustomer(ctx context.Context, customerID uuid.UUID) (*entity.Trip, error) {
	var trip entity.Trip
	err := r.db.WithContext(ctx).
		Where("customer_id = ? AND status IN ('CREATED', 'MATCHING', 'ACCEPTED', 'PICKING_UP', 'IN_TRIP')", customerID).
		Order("created_at DESC").
		First(&trip).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &trip, nil
}

func (r *tripRepositoryImpl) FindActiveByDriver(ctx context.Context, driverID uuid.UUID) (*entity.Trip, error) {
	var trip entity.Trip
	err := r.db.WithContext(ctx).
		Where("driver_id = ? AND status IN ('ACCEPTED', 'PICKING_UP', 'IN_TRIP')", driverID).
		Order("created_at DESC").
		First(&trip).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &trip, nil
}

func (r *tripRepositoryImpl) UpdateInvitedDrivers(ctx context.Context, tripID uuid.UUID, drivers entity.InvitedDrivers) (int64, error) {
	query := `
		UPDATE trips
		SET invited_drivers = ?, updated_at = now()
		WHERE id = ? AND status = 'MATCHING';
	`
	res := r.db.WithContext(ctx).Exec(query, drivers, tripID)
	return res.RowsAffected, res.Error
}

func (r *tripRepositoryImpl) UpdateToAccepted(ctx context.Context, tripID, driverID uuid.UUID) (int64, error) {
	query := `
		UPDATE trips
		SET status = 'ACCEPTED', driver_id = ?, updated_at = now()
		WHERE id = ? AND status = 'MATCHING' AND matching_expires_at > now();
	`
	res := r.db.WithContext(ctx).Exec(query, driverID, tripID)
	return res.RowsAffected, res.Error
}

type pickingUpResult struct {
	CustomerID uuid.UUID `gorm:"column:customer_id"`
}

func (r *tripRepositoryImpl) UpdateToPickingUp(ctx context.Context, tripID, driverID uuid.UUID) (*uuid.UUID, int64, error) {
	query := `
		UPDATE trips
		SET status = 'PICKING_UP', updated_at = now()
		WHERE id = ? AND driver_id = ? AND status = 'ACCEPTED'
		RETURNING customer_id;
	`
	var results []pickingUpResult
	err := r.db.WithContext(ctx).Raw(query, tripID, driverID).Scan(&results).Error
	if err != nil {
		return nil, 0, err
	}
	if len(results) == 0 {
		return nil, 0, nil
	}
	return &results[0].CustomerID, int64(len(results)), nil
}

type inTripResult struct {
	CustomerID uuid.UUID `gorm:"column:customer_id"`
}

func (r *tripRepositoryImpl) UpdateToInTrip(ctx context.Context, tripID, driverID uuid.UUID) (*uuid.UUID, int64, error) {
	query := `
		UPDATE trips
		SET status = 'IN_TRIP', updated_at = now()
		WHERE id = ? AND driver_id = ? AND status = 'PICKING_UP'
		RETURNING customer_id;
	`
	var results []inTripResult
	err := r.db.WithContext(ctx).Raw(query, tripID, driverID).Scan(&results).Error
	if err != nil {
		return nil, 0, err
	}
	if len(results) == 0 {
		return nil, 0, nil
	}
	return &results[0].CustomerID, int64(len(results)), nil
}

type completedResult struct {
	CustomerID  uuid.UUID `gorm:"column:customer_id"`
	Fare        int64     `gorm:"column:fare"`
	CompletedAt time.Time `gorm:"column:completed_at"`
}

func (r *tripRepositoryImpl) UpdateToCompleted(ctx context.Context, tripID, driverID uuid.UUID) (*uuid.UUID, int64, time.Time, int64, error) {
	query := `
		UPDATE trips
		SET status = 'COMPLETED', completed_at = now(), updated_at = now()
		WHERE id = ? AND driver_id = ? AND status = 'IN_TRIP'
		RETURNING customer_id, fare, completed_at;
	`
	var results []completedResult
	err := r.db.WithContext(ctx).Raw(query, tripID, driverID).Scan(&results).Error
	if err != nil {
		return nil, 0, time.Time{}, 0, err
	}
	if len(results) == 0 {
		return nil, 0, time.Time{}, 0, nil
	}
	res := results[0]
	return &res.CustomerID, res.Fare, res.CompletedAt, int64(len(results)), nil
}

type cancelResult struct {
	DriverID   *uuid.UUID `gorm:"column:driver_id"`
	CustomerID *uuid.UUID `gorm:"column:customer_id"`
}

func (r *tripRepositoryImpl) CancelByCustomer(ctx context.Context, tripID, customerID uuid.UUID, reason *string) (*uuid.UUID, *uuid.UUID, int64, error) {
	query := `
		UPDATE trips
		SET status = 'CANCELLED', cancelled_by = 'CUSTOMER', cancel_reason = ?, updated_at = now()
		WHERE id = ? AND customer_id = ? AND status IN ('MATCHING', 'ACCEPTED')
		RETURNING driver_id, customer_id;
	`
	var results []cancelResult
	err := r.db.WithContext(ctx).Raw(query, reason, tripID, customerID).Scan(&results).Error
	if err != nil {
		return nil, nil, 0, err
	}
	if len(results) == 0 {
		return nil, nil, 0, nil
	}
	res := results[0]
	return res.DriverID, res.CustomerID, int64(len(results)), nil
}

func (r *tripRepositoryImpl) CancelByDriver(ctx context.Context, tripID, driverID uuid.UUID, reason *string) (*uuid.UUID, *uuid.UUID, int64, error) {
	query := `
		UPDATE trips
		SET status = 'CANCELLED', cancelled_by = 'DRIVER', cancel_reason = ?, updated_at = now()
		WHERE id = ? AND driver_id = ? AND status IN ('ACCEPTED', 'PICKING_UP')
		RETURNING driver_id, customer_id;
	`
	var results []cancelResult
	err := r.db.WithContext(ctx).Raw(query, reason, tripID, driverID).Scan(&results).Error
	if err != nil {
		return nil, nil, 0, err
	}
	if len(results) == 0 {
		return nil, nil, 0, nil
	}
	res := results[0]
	return res.DriverID, res.CustomerID, int64(len(results)), nil
}

func (r *tripRepositoryImpl) CancelByAdmin(ctx context.Context, tripID uuid.UUID, reason string) (*uuid.UUID, *uuid.UUID, int64, error) {
	query := `
		UPDATE trips
		SET status = 'CANCELLED', cancelled_by = 'ADMIN', cancel_reason = ?, updated_at = now()
		WHERE id = ? AND status IN ('CREATED', 'MATCHING', 'ACCEPTED', 'PICKING_UP', 'IN_TRIP')
		RETURNING driver_id, customer_id;
	`
	var results []cancelResult
	err := r.db.WithContext(ctx).Raw(query, reason, tripID).Scan(&results).Error
	if err != nil {
		return nil, nil, 0, err
	}
	if len(results) == 0 {
		return nil, nil, 0, nil
	}
	res := results[0]
	return res.DriverID, res.CustomerID, int64(len(results)), nil
}

func (r *tripRepositoryImpl) ExpireTrip(ctx context.Context, tripID uuid.UUID) (int64, error) {
	query := `
		UPDATE trips
		SET status = 'EXPIRED', updated_at = now()
		WHERE id = ? AND status = 'MATCHING';
	`
	res := r.db.WithContext(ctx).Exec(query, tripID)
	return res.RowsAffected, res.Error
}

func (r *tripRepositoryImpl) ExpireStaleTrips(ctx context.Context) ([]repository.ExpiredTripResult, error) {
	query := `
		UPDATE trips
		SET status = 'EXPIRED', updated_at = now()
		WHERE status = 'MATCHING' AND matching_expires_at <= now()
		RETURNING id, customer_id;
	`
	var results []repository.ExpiredTripResult
	err := r.db.WithContext(ctx).Raw(query).Scan(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (r *tripRepositoryImpl) ListTripsAdmin(ctx context.Context, status string, page, limit int) ([]entity.Trip, int64, error) {
	var trips []entity.Trip
	var total int64

	db := r.db.WithContext(ctx).Model(&entity.Trip{})
	if status != "" {
		db = db.Where("status = ?", status)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&trips).Error
	if err != nil {
		return nil, 0, err
	}

	return trips, total, nil
}
