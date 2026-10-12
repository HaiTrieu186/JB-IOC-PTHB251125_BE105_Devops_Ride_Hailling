package impl

import (
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"payment-service/internal/entity"
)

func NewPostgresDB(databaseURL string) (*gorm.DB, error) {
	var db *gorm.DB
	var err error

	for attempt := 1; attempt <= 10; attempt++ {
		db, err = gorm.Open(postgres.Open(databaseURL), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err == nil {
			sqlDB, dbErr := db.DB()
			if dbErr == nil {
				pingErr := sqlDB.Ping()
				if pingErr == nil {
					// Connection pool configuration per LLD-00 Section 5(h)
					sqlDB.SetMaxOpenConns(10)
					sqlDB.SetMaxIdleConns(3)
					sqlDB.SetConnMaxLifetime(30 * time.Minute)

					slog.Info("connected to PostgreSQL paymentdb", "attempt", attempt)
					break
				}
				err = pingErr
			} else {
				err = dbErr
			}
		}

		slog.Warn("failed to connect to PostgreSQL paymentdb, retrying...", "attempt", attempt, "error", err)
		if attempt < 10 {
			time.Sleep(3 * time.Second)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("could not connect to PostgreSQL paymentdb after 10 attempts: %w", err)
	}

	// AutoMigrate tables
	if err := db.AutoMigrate(&entity.Wallet{}, &entity.Transaction{}); err != nil {
		return nil, fmt.Errorf("failed to auto migrate paymentdb: %w", err)
	}

	// Raw SQL for constraints and indexes (LLD-00 Section 5(l) and LLD payment-service Section 2)
	rawSQL := `
	DO $$
	BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'check_balance_non_negative') THEN
			ALTER TABLE wallets ADD CONSTRAINT check_balance_non_negative CHECK (balance >= 0);
		END IF;
		IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'check_transaction_type') THEN
			ALTER TABLE transactions ADD CONSTRAINT check_transaction_type CHECK (type IN ('TRIP_PAYMENT', 'TRIP_INCOME', 'COMMISSION', 'TOPUP'));
		END IF;
		IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'check_transaction_status') THEN
			ALTER TABLE transactions ADD CONSTRAINT check_transaction_status CHECK (status IN ('SUCCESS', 'FAILED'));
		END IF;
	END $$;

	CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_trip_type ON transactions (trip_id, type) WHERE trip_id IS NOT NULL;
	CREATE INDEX IF NOT EXISTS idx_transactions_user_created ON transactions (user_id, created_at DESC);
	`

	if err := db.Exec(rawSQL).Error; err != nil {
		return nil, fmt.Errorf("failed to execute raw SQL schema migration on paymentdb: %w", err)
	}

	slog.Info("PostgreSQL paymentdb migrations applied successfully")
	return db, nil
}
