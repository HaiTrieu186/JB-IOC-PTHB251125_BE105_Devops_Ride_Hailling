package entity

import "time"

type PricingConfig struct {
	ID             int       `gorm:"primaryKey;type:integer;default:1;autoIncrement:false" json:"id"`
	BaseFare       int64     `gorm:"column:base_fare;not null" json:"base_fare"`
	PricePerKm     int64     `gorm:"column:price_per_km;not null" json:"price_per_km"`
	SurgeThreshold float64   `gorm:"column:surge_threshold;type:numeric(4,2);not null" json:"surge_threshold"`
	UpdatedAt      time.Time `gorm:"column:updated_at;not null;default:now()" json:"updated_at"`
}

func (PricingConfig) TableName() string {
	return "pricing_configs"
}
