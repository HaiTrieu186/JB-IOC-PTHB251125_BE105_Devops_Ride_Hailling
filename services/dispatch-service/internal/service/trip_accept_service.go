package service

import (
	"context"

	"github.com/google/uuid"

	"dispatch-service/internal/dto"
)

type TripAcceptService interface {
	AcceptTrip(ctx context.Context, driverID, tripID uuid.UUID) (*dto.AcceptTripResponse, error)
}
