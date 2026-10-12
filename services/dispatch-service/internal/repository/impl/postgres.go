package impl

import (
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"dispatch-service/internal/entity"
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
					// Cấu hình Connection Pool theo LLD-00 Mục 5(h) và LLD Mục 7.2
					// MaxOpenConns = 10, MaxIdleConns = 3, ConnMaxLifetime = 30m
					sqlDB.SetMaxOpenConns(10)
					sqlDB.SetMaxIdleConns(3)
					sqlDB.SetConnMaxLifetime(30 * time.Minute)

					slog.Info("connected to PostgreSQL dispatchdb", "attempt", attempt)
					break
				}
				err = pingErr
			} else {
				err = dbErr
			}
		}

		slog.Warn("failed to connect to PostgreSQL dispatchdb, retrying...", "attempt", attempt, "error", err)
		if attempt < 10 {
			time.Sleep(3 * time.Second)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("could not connect to PostgreSQL dispatchdb after 10 attempts: %w", err)
	}

	// AutoMigrate bảng trips và status_timeline
	if err := db.AutoMigrate(&entity.Trip{}, &entity.StatusTimeline{}); err != nil {
		return nil, fmt.Errorf("failed to auto migrate dispatchdb: %w", err)
	}

	// Raw SQL tạo chỉ mục & ràng buộc theo Mục 5(l) LLD-00 và LLD dispatch-service Mục 2
	rawSQL := `
	DO $$
	BEGIN
		-- Chỉ mục partial unique chống 2 chuyến active đồng thời của 1 khách
		IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_trips_active_customer') THEN
			CREATE UNIQUE INDEX idx_trips_active_customer ON trips (customer_id)
			WHERE status IN ('CREATED', 'MATCHING', 'ACCEPTED', 'PICKING_UP', 'IN_TRIP');
		END IF;

		-- Chỉ mục tối ưu cho worker quét quá hạn 2s/lần
		IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_trips_matching_timeout') THEN
			CREATE INDEX idx_trips_matching_timeout ON trips (matching_expires_at)
			WHERE status = 'MATCHING';
		END IF;

		-- Chỉ mục tra cứu chuyến hiện tại của khách
		IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_trips_customer_lookup') THEN
			CREATE INDEX idx_trips_customer_lookup ON trips (customer_id, status);
		END IF;

		-- Chỉ mục chuyến active của tài xế
		IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_trips_driver_active') THEN
			CREATE INDEX idx_trips_driver_active ON trips (driver_id, status)
			WHERE status IN ('ACCEPTED', 'PICKING_UP', 'IN_TRIP');
		END IF;

		-- Chỉ mục phân trang danh sách chuyến cho Admin
		IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_trips_admin_list') THEN
			CREATE INDEX idx_trips_admin_list ON trips (created_at DESC);
		END IF;

		-- Chỉ mục lịch sử dòng trạng thái
		IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_timeline_trip_id') THEN
			CREATE INDEX idx_timeline_trip_id ON status_timeline (trip_id, id ASC);
		END IF;
	END $$;
	`

	if err := db.Exec(rawSQL).Error; err != nil {
		return nil, fmt.Errorf("failed to execute raw SQL schema migration on dispatchdb: %w", err)
	}

	slog.Info("PostgreSQL dispatchdb migrations applied successfully")
	return db, nil
}
