package service

import (
	"context"

	"github.com/google/uuid"
)

type SettlementService interface {
	ProcessTripCompleted(ctx context.Context, tripID, customerID, driverID uuid.UUID, fare int64) error
}
