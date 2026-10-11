package util

import (
	"math"
)

// CalculateFare tính toán cước chuyến đi và làm tròn tới 1.000 VND gần nhất (nửa lên)
// D_km = distanceMeters / 1000.0
// RawFare = (baseFare + D_km * pricePerKm) * surgeMultiplier
// fare = floor((RawFare + 500) / 1000) * 1000
func CalculateFare(baseFare, pricePerKm int64, distanceMeters int, surgeMultiplier float64) int64 {
	dKm := float64(distanceMeters) / 1000.0
	rawFare := (float64(baseFare) + dKm*float64(pricePerKm)) * surgeMultiplier
	fare := int64(math.Floor((rawFare+500.0)/1000.0)) * 1000
	return fare
}
