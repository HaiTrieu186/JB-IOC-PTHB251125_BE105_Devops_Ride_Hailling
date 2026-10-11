package dto

type RecordLocationRequest struct {
	DriverID  string   `json:"driver_id"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

type CandidatesRequest struct {
	PickupLat *float64 `json:"pickup_lat"`
	PickupLng *float64 `json:"pickup_lng"`
	RadiusKM  *float64 `json:"radius_km"`
}

type CandidateDTO struct {
	DriverID  string `json:"driver_id"`
	DistanceM int64  `json:"distance_m"`
}

type CandidatesResponse struct {
	Candidates []CandidateDTO `json:"candidates"`
}

type GeohashDriversRequest struct {
	Geohash5 string `json:"geohash5"`
}

type GeohashDriversResponse struct {
	DriverIDs []string `json:"driver_ids"`
}

type ActiveDriverDTO struct {
	DriverID  string  `json:"driver_id"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	LastSeen  string  `json:"last_seen"`
}

type ActiveDriversResponse struct {
	Drivers []ActiveDriverDTO `json:"drivers"`
}
