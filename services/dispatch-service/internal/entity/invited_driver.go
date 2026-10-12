package entity

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

type InvitedDriver struct {
	DriverID  string `json:"driver_id"`
	DistanceM int    `json:"distance_m"`
}

type InvitedDrivers []InvitedDriver

func (a InvitedDrivers) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	return json.Marshal(a)
}

func (a *InvitedDrivers) Scan(value interface{}) error {
	if value == nil {
		*a = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("failed to unmarshal JSONB value: %v", value)
		}
		bytes = []byte(str)
	}
	return json.Unmarshal(bytes, a)
}
