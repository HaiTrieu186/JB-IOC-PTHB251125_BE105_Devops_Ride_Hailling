package util

import (
	"github.com/google/uuid"
	"github.com/mmcloughlin/geohash"
)

const (
	MinLatitude  = -85.05112878
	MaxLatitude  = 85.05112878
	MinLongitude = -180.0
	MaxLongitude = 180.0
)

func ValidateCoordinates(lat, lng *float64) bool {
	if lat == nil || lng == nil {
		return false
	}
	if *lat < MinLatitude || *lat > MaxLatitude {
		return false
	}
	if *lng < MinLongitude || *lng > MaxLongitude {
		return false
	}
	return true
}

func ValidateUUID(id string) bool {
	if id == "" {
		return false
	}
	_, err := uuid.Parse(id)
	return err == nil
}

func ValidateGeohash5(hash string) bool {
	if len(hash) != 5 {
		return false
	}
	return geohash.Validate(hash) == nil
}
