package service

import (
	"context"

	"github.com/google/uuid"

	"dispatch-service/internal/dto"
)

type TripCreateService interface {
	CreateTrip(ctx context.Context, customerID uuid.UUID, req *dto.CreateTripRequest) (*dto.CreateTripResponse, error)
}
