package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"dispatch-service/internal/entity"
)

type ExpiredTripResult struct {
	ID         uuid.UUID `gorm:"column:id"`
	CustomerID uuid.UUID `gorm:"column:customer_id"`
}

type TripRepository interface {
	HasActiveTripByCustomer(ctx context.Context, customerID uuid.UUID) (bool, error)
	FindByID(ctx context.Context, tripID uuid.UUID) (*entity.Trip, error)
	FindActiveByCustomer(ctx context.Context, customerID uuid.UUID) (*entity.Trip, error)
	FindActiveByDriver(ctx context.Context, driverID uuid.UUID) (*entity.Trip, error)

	UpdateInvitedDrivers(ctx context.Context, tripID uuid.UUID, drivers entity.InvitedDrivers) (int64, error)
	UpdateToAccepted(ctx context.Context, tripID, driverID uuid.UUID) (int64, error)
	UpdateToPickingUp(ctx context.Context, tripID, driverID uuid.UUID) (customerID *uuid.UUID, rowsAffected int64, err error)
	UpdateToInTrip(ctx context.Context, tripID, driverID uuid.UUID) (customerID *uuid.UUID, rowsAffected int64, err error)
	UpdateToCompleted(ctx context.Context, tripID, driverID uuid.UUID) (customerID *uuid.UUID, fare int64, completedAt time.Time, rowsAffected int64, err error)

	CancelByCustomer(ctx context.Context, tripID, customerID uuid.UUID, reason *string) (driverID *uuid.UUID, customerIDRet *uuid.UUID, rowsAffected int64, err error)
	CancelByDriver(ctx context.Context, tripID, driverID uuid.UUID, reason *string) (driverIDRet *uuid.UUID, customerIDRet *uuid.UUID, rowsAffected int64, err error)
	CancelByAdmin(ctx context.Context, tripID uuid.UUID, reason string) (driverID *uuid.UUID, customerID *uuid.UUID, rowsAffected int64, err error)

	ExpireTrip(ctx context.Context, tripID uuid.UUID) (int64, error)
	ExpireStaleTrips(ctx context.Context) ([]ExpiredTripResult, error)

	ListTripsAdmin(ctx context.Context, status string, page, limit int) ([]entity.Trip, int64, error)
}
