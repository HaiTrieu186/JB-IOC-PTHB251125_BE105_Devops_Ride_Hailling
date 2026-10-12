package entity

import (
	"time"

	"github.com/google/uuid"
)

const (
	TxTypeTripPayment = "TRIP_PAYMENT"
	TxTypeTripIncome  = "TRIP_INCOME"
	TxTypeCommission  = "COMMISSION"
	TxTypeTopup       = "TOPUP"

	TxStatusSuccess = "SUCCESS"
	TxStatusFailed  = "FAILED"
)

type Transaction struct {
	ID        int64      `gorm:"primaryKey;autoIncrement"`
	WalletID  *uuid.UUID `gorm:"type:uuid"`
	UserID    *uuid.UUID `gorm:"type:uuid"`
	TripID    *uuid.UUID `gorm:"type:uuid"`
	Type      string     `gorm:"type:varchar(20);not null"`
	Amount    int64      `gorm:"type:bigint;not null"`
	Status    string     `gorm:"type:varchar(10);not null"`
	Note      *string    `gorm:"type:text"`
	CreatedAt time.Time  `gorm:"type:timestamptz;not null;default:now()"`
}

func (Transaction) TableName() string {
	return "transactions"
}
