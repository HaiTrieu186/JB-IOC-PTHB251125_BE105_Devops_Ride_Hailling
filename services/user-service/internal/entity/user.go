package entity

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

func (User) TableName() string {
	return "users"
}
