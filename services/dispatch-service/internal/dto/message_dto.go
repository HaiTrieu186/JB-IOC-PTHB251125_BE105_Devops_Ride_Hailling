package dto

// TargetDriver biểu diễn ứng viên tài xế trong bản tin ride:trip_offers
type TargetDriver struct {
	DriverID  string `json:"driver_id"`
	DistanceM int    `json:"distance_m"`
}

// TripOfferMessage là payload phát vào kênh Pub/Sub ride:trip_offers
type TripOfferMessage struct {
	TripID     string         `json:"trip_id"`
	Targets    []TargetDriver `json:"targets"`
	PickupLat  float64        `json:"pickup_lat"`
	PickupLng  float64        `json:"pickup_lng"`
	Fare       int64          `json:"fare"`
	DriverFare int64          `json:"driver_fare"`
	ExpireAt   string         `json:"expire_at"`
}

// TripUpdateMessage là payload phát vào kênh Pub/Sub ride:trip_updates
// DriverID bắt buộc KHÔNG có omitempty để render null thật ra JSON khi chưa có tài xế (LLD-00 Mục 2 & Quyết định E)
type TripUpdateMessage struct {
	TripID     string  `json:"trip_id"`
	CustomerID string  `json:"customer_id"`
	DriverID   *string `json:"driver_id"`
	Status     string  `json:"status"`
	UpdatedAt  string  `json:"updated_at"`
}

// TripLinkData lưu trữ tại Redis key trip:link:<driver_id>
type TripLinkData struct {
	TripID     string `json:"trip_id"`
	CustomerID string `json:"customer_id"`
}
