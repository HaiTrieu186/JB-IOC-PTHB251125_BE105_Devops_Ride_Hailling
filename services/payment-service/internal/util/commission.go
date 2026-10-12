package util

func CalculateCommission(fare int64, commissionRate int) (commission int64, driverIncome int64) {
	commission = (fare*int64(commissionRate) + 50) / 100
	driverIncome = fare - commission
	return commission, driverIncome
}
