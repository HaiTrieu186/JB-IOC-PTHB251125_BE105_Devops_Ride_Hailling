package entity

import (
	"time"

	"github.com/google/uuid"
)

type Trip struct {
	ID                uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	CustomerID        uuid.UUID      `gorm:"type:uuid;not null" json:"customer_id"`
	DriverID          *uuid.UUID     `gorm:"type:uuid" json:"driver_id"`
	Status            string         `gorm:"type:varchar(20);not null" json:"status"`
	PickupLat         float64        `gorm:"type:double precision;not null" json:"pickup_lat"`
	PickupLng         float64        `gorm:"type:double precision;not null" json:"pickup_lng"`
	PickupAddress     string         `gorm:"type:text;not null" json:"pickup_address"`
	DropoffLat        float64        `gorm:"type:double precision;not null" json:"dropoff_lat"`
	DropoffLng        float64        `gorm:"type:double precision;not null" json:"dropoff_lng"`
	DropoffAddress    string         `gorm:"type:text;not null" json:"dropoff_address"`
	Fare              int64          `gorm:"type:bigint;not null" json:"fare"`
	DistanceM         int            `gorm:"type:integer;not null" json:"distance_m"`
	SurgeMultiplier   float64        `gorm:"type:numeric(3,2);not null;default:1.0" json:"surge_multiplier"`
	PickupGeohash5    string         `gorm:"type:varchar(5);not null" json:"pickup_geohash5"`
	InvitedDrivers    InvitedDrivers `gorm:"type:jsonb" json:"invited_drivers,omitempty"`
	MatchingExpiresAt time.Time      `gorm:"type:timestamptz;not null" json:"matching_expires_at"`
	CompletedAt       *time.Time     `gorm:"type:timestamptz" json:"completed_at"`
	CancelledBy       *string        `gorm:"type:varchar(10)" json:"cancelled_by"`
	CancelReason      *string        `gorm:"type:text" json:"cancel_reason"`
	CreatedAt         time.Time      `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
	UpdatedAt         time.Time      `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`
}

func (Trip) TableName() string {
	return "trips"
}
