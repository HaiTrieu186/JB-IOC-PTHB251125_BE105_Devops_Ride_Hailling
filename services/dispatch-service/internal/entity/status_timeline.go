package entity

import (
	"time"

	"github.com/google/uuid"
)

type StatusTimeline struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TripID    uuid.UUID `gorm:"type:uuid;not null;index:idx_timeline_trip_id" json:"trip_id"`
	Status    string    `gorm:"type:varchar(20);not null" json:"status"`
	Note      *string   `gorm:"type:text" json:"note"`
	CreatedAt time.Time `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
}

func (StatusTimeline) TableName() string {
	return "status_timeline"
}
