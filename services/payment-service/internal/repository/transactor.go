package repository

import (
	"context"

	"github.com/google/uuid"
)

type SettlementParams struct {
	TripID       uuid.UUID
	CustomerID   uuid.UUID
	DriverID     uuid.UUID
	Fare         int64
	Commission   int64
	DriverIncome int64
}

type SettlementResult struct {
	Success          bool
	InsufficientFund bool
	Duplicate        bool
}

type Transactor interface {
	ExecuteSettlement(ctx context.Context, params SettlementParams) (*SettlementResult, error)
}
