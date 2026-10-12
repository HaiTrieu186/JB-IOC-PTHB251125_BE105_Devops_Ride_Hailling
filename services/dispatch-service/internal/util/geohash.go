package util

import (
	"github.com/mmcloughlin/geohash"
)

// EncodeGeohash5 tính chuỗi Geohash chuẩn độ dài 5 từ tọa độ đón (lat, lng)
func EncodeGeohash5(lat, lng float64) string {
	return geohash.EncodeWithPrecision(lat, lng, 5)
}
