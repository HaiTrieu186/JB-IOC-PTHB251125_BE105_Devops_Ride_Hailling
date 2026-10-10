package main

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	PhoneNumber  *string    `gorm:"type:varchar(15);uniqueIndex" json:"phone_number"`
	Email        *string    `gorm:"type:varchar(255);uniqueIndex" json:"email"`
	PasswordHash string     `gorm:"type:varchar(255);not null" json:"-"`
	FullName     string     `gorm:"type:varchar(100);not null" json:"full_name"`
	Role         string     `gorm:"type:varchar(20);not null" json:"role"`
	DriverStatus *string    `gorm:"type:varchar(20)" json:"driver_status"`
	CreatedAt    time.Time  `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`
}

type Vehicle struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	DriverID     uuid.UUID `gorm:"type:uuid;uniqueIndex;not null" json:"driver_id"`
	LicensePlate string    `gorm:"type:varchar(20);uniqueIndex;not null" json:"license_plate"`
	VehicleType  string    `gorm:"type:varchar(20);not null" json:"vehicle_type"`
	Brand        *string   `gorm:"type:varchar(50)" json:"brand"`
	Model        *string   `gorm:"type:varchar(50)" json:"model"`
	Color        *string   `gorm:"type:varchar(30)" json:"color"`
	UpdatedAt    time.Time `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`
}
