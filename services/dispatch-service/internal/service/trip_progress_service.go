package service

import (
	"context"

	"github.com/google/uuid"

	"dispatch-service/internal/dto"
)

type TripProgressService interface {
	PickingUp(ctx context.Context, driverID, tripID uuid.UUID) (*dto.StatusResponse, error)
	StartTrip(ctx context.Context, driverID, tripID uuid.UUID) (*dto.StatusResponse, error)
	CompleteTrip(ctx context.Context, driverID, tripID uuid.UUID) (*dto.CompleteTripResponse, error)

	CancelTrip(ctx context.Context, userID uuid.UUID, role string, tripID uuid.UUID, reason *string) (*dto.StatusResponse, error)
	ForceCancelTrip(ctx context.Context, tripID uuid.UUID, reason string) (*dto.ForceCancelResponse, error)

	GetCurrentTrip(ctx context.Context, userID uuid.UUID, role string) (*dto.TripResponse, error)
	GetTripDetail(ctx context.Context, userID uuid.UUID, role string, tripID uuid.UUID) (*dto.TripDetailResponse, error)
	ListTripsAdmin(ctx context.Context, status string, page, limit int) (*dto.AdminTripsResponse, error)
}
