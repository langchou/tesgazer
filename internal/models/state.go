package models

import "time"

// State 车辆状态记录
type State struct {
	ID        int64      `json:"id" db:"id"`
	CarID     int64      `json:"car_id" db:"car_id"`
	State     string     `json:"state" db:"state"` // online, asleep, charging, driving, updating
	StartTime time.Time  `json:"start_time" db:"start_time"`
	EndTime   *time.Time `json:"end_time,omitempty" db:"end_time"`
}
