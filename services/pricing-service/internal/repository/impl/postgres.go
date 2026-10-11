package impl

import (
	"fmt"
	"log/slog"
	"time"

	"pricing-service/internal/config"
	"pricing-service/internal/entity"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func InitDB(cfg *config.Config) (*gorm.DB, error) {
	var db *gorm.DB
	var err error

	for attempt := 1; attempt <= 10; attempt++ {
		db, err = gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err == nil {
			sqlDB, dbErr := db.DB()
			if dbErr == nil {
				if pingErr := sqlDB.Ping(); pingErr == nil {
					sqlDB.SetMaxOpenConns(5)
					sqlDB.SetMaxIdleConns(2)
					sqlDB.SetConnMaxLifetime(30 * time.Minute)
					slog.Info("connected to postgres database successfully")
					break
				}
			}
		}
		slog.Warn("failed to connect to postgres, retrying...", "attempt", attempt, "max", 10, "error", err)
		time.Sleep(3 * time.Second)
	}

	if db == nil {
		return nil, fmt.Errorf("unable to connect to postgres after 10 attempts: %w", err)
	}

	if err := db.AutoMigrate(&entity.PricingConfig{}); err != nil {
		return nil, fmt.Errorf("automigrate failed: %w", err)
	}

	rawConstraintQuery := `DO $$
	BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_pricing_configs_id') THEN
			ALTER TABLE pricing_configs ADD CONSTRAINT chk_pricing_configs_id CHECK (id = 1);
		END IF;
		IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_pricing_configs_valid') THEN
			ALTER TABLE pricing_configs ADD CONSTRAINT chk_pricing_configs_valid CHECK (base_fare > 0 AND price_per_km > 0 AND surge_threshold >= 0.01 AND surge_threshold <= 99.99);
		END IF;
	END $$;`

	if execErr := db.Exec(rawConstraintQuery).Error; execErr != nil {
		slog.Warn("executing pricing_configs schema constraint query failed", "error", execErr)
	}

	return db, nil
}
