package models

import "time"

// ParkingEventType 停车事件类型
type ParkingEventType string

const (
	// 车门事件
	EventDoorsOpened  ParkingEventType = "doors_opened"
	EventDoorsClosed  ParkingEventType = "doors_closed"

	// 车窗事件
	EventWindowsOpened ParkingEventType = "windows_opened"
	EventWindowsClosed ParkingEventType = "windows_closed"

	// 后备箱事件
	EventTrunkOpened ParkingEventType = "trunk_opened"
	EventTrunkClosed ParkingEventType = "trunk_closed"

	// 前备箱事件
	EventFrunkOpened ParkingEventType = "frunk_opened"
	EventFrunkClosed ParkingEventType = "frunk_closed"

	// 锁车事件
	EventUnlocked ParkingEventType = "unlocked"
	EventLocked   ParkingEventType = "locked"

	// 哨兵模式事件
	EventSentryEnabled  ParkingEventType = "sentry_enabled"
	EventSentryDisabled ParkingEventType = "sentry_disabled"

	// 空调事件
	EventClimateOn  ParkingEventType = "climate_on"
	EventClimateOff ParkingEventType = "climate_off"

	// 用户在车内事件
	EventUserPresent ParkingEventType = "user_present"
	EventUserLeft    ParkingEventType = "user_left"
)

// ParkingEvent 停车事件
type ParkingEvent struct {
	ID        int64                  `json:"id" db:"id"`
	ParkingID int64                  `json:"parking_id" db:"parking_id"`
	EventType ParkingEventType       `json:"event_type" db:"event_type"`
	EventTime time.Time              `json:"event_time" db:"event_time"`
	Details   map[string]interface{} `json:"details,omitempty" db:"details"`
}
