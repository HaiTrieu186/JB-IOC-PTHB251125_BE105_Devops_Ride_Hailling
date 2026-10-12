package entity

import (
	"time"

	"github.com/google/uuid"
)

type Wallet struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;uniqueIndex"`
	Balance   int64     `gorm:"type:bigint;not null;default:0"`
	CreatedAt time.Time `gorm:"type:timestamptz;not null;default:now()"`
	UpdatedAt time.Time `gorm:"type:timestamptz;not null;default:now()"`
}

func (Wallet) TableName() string {
	return "wallets"
}
