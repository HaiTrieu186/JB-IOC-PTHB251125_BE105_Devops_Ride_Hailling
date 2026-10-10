package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func initDB(cfg *Config) (*gorm.DB, error) {
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

	// AutoMigrate tables
	if err := db.AutoMigrate(&User{}, &Vehicle{}); err != nil {
		return nil, fmt.Errorf("automigrate failed: %w", err)
	}

	// Create index and constraints idempotently
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

func initRedis(cfg *Config) (*redis.Client, error) {
	var rdb *redis.Client
	var err error

	for attempt := 1; attempt <= 10; attempt++ {
		rdb = redis.NewClient(&redis.Options{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
		})
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err = rdb.Ping(ctx).Err()
		cancel()

		if err == nil {
			slog.Info("connected to redis successfully")
			return rdb, nil
		}
		slog.Warn("failed to connect to redis, retrying...", "attempt", attempt, "max", 10, "error", err)
		time.Sleep(3 * time.Second)
	}

	return nil, fmt.Errorf("unable to connect to redis after 10 attempts: %w", err)
}

func seedAdmin(db *gorm.DB, cfg *Config) error {
	if cfg.AdminEmail == "" || cfg.AdminPassword == "" {
		return nil
	}

	var existing User
	err := db.Where("email = ?", cfg.AdminEmail).First(&existing).Error
	if err == nil {
		slog.Info("admin account already exists, skipping seed", "email", cfg.AdminEmail)
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), 10)
	if err != nil {
		return fmt.Errorf("failed to hash admin password: %w", err)
	}

	admin := User{
		ID:           uuid.New(),
		PhoneNumber:  nil,
		Email:        &cfg.AdminEmail,
		PasswordHash: string(hash),
		FullName:     "Administrator",
		Role:         "ADMIN",
		DriverStatus: nil,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if createErr := db.Create(&admin).Error; createErr != nil {
		return fmt.Errorf("failed to insert admin user: %w", createErr)
	}

	slog.Info("seeded admin account successfully", "email", cfg.AdminEmail, "user_id", admin.ID)
	return nil
}
