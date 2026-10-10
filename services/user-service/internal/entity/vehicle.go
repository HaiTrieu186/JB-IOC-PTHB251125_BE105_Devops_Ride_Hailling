package entity

import (
	"time"

	"github.com/google/uuid"
)

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

func (Vehicle) TableName() string {
	return "vehicles"
}
