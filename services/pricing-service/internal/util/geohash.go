package util

import (
	"github.com/mmcloughlin/geohash"
)

// EncodeGeohash5 mã hóa tọa độ (lat, lng) thành chuỗi geohash 5 ký tự
func EncodeGeohash5(lat, lng float64) string {
	hash := geohash.Encode(lat, lng)
	if len(hash) >= 5 {
		return hash[:5]
	}
	return hash
}
