package dto

type DriverStatusRequest struct {
	Status string `json:"status"`
}

type DriverStatusResponse struct {
	DriverID string `json:"driver_id"`
	Status   string `json:"status"`
}

type DriverInternalStatusRequest struct {
	FromStatus string `json:"from_status"`
	ToStatus   string `json:"to_status"`
}

type FilterOnlineRequest struct {
	DriverIDs []string `json:"driver_ids"`
}

type FilterOnlineResponse struct {
	OnlineDriverIDs []string `json:"online_driver_ids"`
}
