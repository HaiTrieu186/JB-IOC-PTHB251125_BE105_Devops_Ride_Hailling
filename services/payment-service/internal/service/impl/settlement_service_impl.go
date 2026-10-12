package impl

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"payment-service/internal/repository"
	"payment-service/internal/service"
	"payment-service/internal/util"
)

type settlementServiceImpl struct {
	transactor      repository.Transactor
	transactionRepo repository.TransactionRepository
	commissionRate  int
}

func NewSettlementService(
	transactor repository.Transactor,
	transactionRepo repository.TransactionRepository,
	commissionRate int,
) service.SettlementService {
	return &settlementServiceImpl{
		transactor:      transactor,
		transactionRepo: transactionRepo,
		commissionRate:  commissionRate,
	}
}

func (s *settlementServiceImpl) ProcessTripCompleted(
	ctx context.Context,
	tripID, customerID, driverID uuid.UUID,
	fare int64,
) error {
	// 1. Idempotency check via transactions table
	exists, err := s.transactionRepo.ExistsByTripID(ctx, tripID)
	if err != nil {
		return err
	}
	if exists {
		slog.Info("TripCompleted event already processed, skipping (idempotent)", "trip_id", tripID)
		return nil
	}

	// 2. Calculate integer commission and driver income
	commission, driverIncome := util.CalculateCommission(fare, s.commissionRate)

	// 3. Execute atomic settlement in DB transaction
	res, err := s.transactor.ExecuteSettlement(ctx, repository.SettlementParams{
		TripID:       tripID,
		CustomerID:   customerID,
		DriverID:     driverID,
		Fare:         fare,
		Commission:   commission,
		DriverIncome: driverIncome,
	})
	if err != nil {
		return err
	}

	if res.Duplicate {
		slog.Warn("TripCompleted duplicate transaction detected via unique constraint", "trip_id", tripID)
		return nil
	}

	if res.InsufficientFund {
		slog.Error("TripCompleted customer insufficient funds on complete, recorded FAILED transaction",
			"trip_id", tripID,
			"customer_id", customerID,
			"fare", fare,
		)
		return nil
	}

	slog.Info("TripCompleted settled successfully",
		"trip_id", tripID,
		"customer_id", customerID,
		"driver_id", driverID,
		"fare", fare,
		"commission", commission,
		"driver_income", driverIncome,
	)
	return nil
}
