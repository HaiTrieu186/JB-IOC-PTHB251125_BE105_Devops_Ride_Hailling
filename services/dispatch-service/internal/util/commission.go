package util

// CalculateDriverFare tính cước cho tài xế dựa trên giá cước và tỷ lệ hoa hồng
// commission = (fare * COMMISSION_RATE + 50) / 100
// driverFare = fare - commission
func CalculateDriverFare(fare int64, commissionRate int) (commission int64, driverFare int64) {
	commission = (fare*int64(commissionRate) + 50) / 100
	driverFare = fare - commission
	return commission, driverFare
}
