package impl

import (
	"fmt"
	"log/slog"
	"time"

	"user-service/internal/config"
	"user-service/internal/entity"

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

	if err := db.AutoMigrate(&entity.User{}, &entity.Vehicle{}); err != nil {
		return nil, fmt.Errorf("automigrate failed: %w", err)
	}

	rawQueries := []string{
		`CREATE INDEX IF NOT EXISTS idx_users_role_status ON users (role, driver_status);`,
		`DO $$
		BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_users_role') THEN
				ALTER TABLE users ADD CONSTRAINT chk_users_role CHECK (role IN ('CUSTOMER', 'DRIVER', 'ADMIN'));
			END IF;
			IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_users_driver_status') THEN
				ALTER TABLE users ADD CONSTRAINT chk_users_driver_status CHECK (
					(role = 'DRIVER' AND driver_status IN ('ONLINE', 'OFFLINE', 'BUSY')) OR
					(role != 'DRIVER' AND driver_status IS NULL)
				);
			END IF;
			IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_vehicles_driver') THEN
				ALTER TABLE vehicles ADD CONSTRAINT fk_vehicles_driver FOREIGN KEY (driver_id) REFERENCES users(id) ON DELETE CASCADE;
			END IF;
		END $$;`,
	}

	for _, q := range rawQueries {
		if execErr := db.Exec(q).Error; execErr != nil {
			slog.Warn("executing schema constraint query failed", "query", q, "error", execErr)
		}
	}

	return db, nil
}
