package models

import (
	"time"
)

// Car 车辆信息
type Car struct {
	ID           int64     `json:"id" db:"id"`
	TeslaID      int64     `json:"tesla_id" db:"tesla_id"`
	TeslaVehicleID int64   `json:"tesla_vehicle_id" db:"tesla_vehicle_id"`
	VIN          string    `json:"vin" db:"vin"`
	Name         string    `json:"name" db:"name"`
	Model        string    `json:"model" db:"model"`
	TrimBadging  string    `json:"trim_badging" db:"trim_badging"`
	ExteriorColor string   `json:"exterior_color" db:"exterior_color"`
	WheelType    string    `json:"wheel_type" db:"wheel_type"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// Position 位置记录
type Position struct {
	ID          int64     `json:"id" db:"id"`
	CarID       int64     `json:"car_id" db:"car_id"`
	DriveID     *int64    `json:"drive_id,omitempty" db:"drive_id"`
	Latitude    float64   `json:"latitude" db:"latitude"`
	Longitude   float64   `json:"longitude" db:"longitude"`
	Heading     int       `json:"heading" db:"heading"`
	Speed       *int      `json:"speed,omitempty" db:"speed"`           // km/h
	Power       int       `json:"power" db:"power"`                     // kW
	Odometer    float64   `json:"odometer" db:"odometer"`               // km
	BatteryLevel int      `json:"battery_level" db:"battery_level"`
	RangeKm     float64   `json:"range_km" db:"range_km"`
	InsideTemp  *float64  `json:"inside_temp,omitempty" db:"inside_temp"`
	OutsideTemp *float64  `json:"outside_temp,omitempty" db:"outside_temp"`
	Elevation   *int      `json:"elevation,omitempty" db:"elevation"`   // 海拔 (米)
	RecordedAt  time.Time `json:"recorded_at" db:"recorded_at"`
}

// Drive 行程记录
type Drive struct {
	ID                int64      `json:"id" db:"id"`
	CarID             int64      `json:"car_id" db:"car_id"`
	StartTime         time.Time  `json:"start_time" db:"start_time"`
	EndTime           *time.Time `json:"end_time,omitempty" db:"end_time"`
	StartPositionID   *int64     `json:"start_position_id,omitempty" db:"start_position_id"`
	EndPositionID     *int64     `json:"end_position_id,omitempty" db:"end_position_id"`
	StartGeofenceID   *int64     `json:"start_geofence_id,omitempty" db:"start_geofence_id"`
	EndGeofenceID     *int64     `json:"end_geofence_id,omitempty" db:"end_geofence_id"`
	DistanceKm        float64    `json:"distance_km" db:"distance_km"`
	DurationMin       float64    `json:"duration_min" db:"duration_min"`
	StartBatteryLevel int        `json:"start_battery_level" db:"start_battery_level"`
	EndBatteryLevel   *int       `json:"end_battery_level,omitempty" db:"end_battery_level"`
	StartRangeKm      float64    `json:"start_range_km" db:"start_range_km"`
	EndRangeKm        *float64   `json:"end_range_km,omitempty" db:"end_range_km"`
	SpeedMax          *int       `json:"speed_max,omitempty" db:"speed_max"`
	PowerMax          *int       `json:"power_max,omitempty" db:"power_max"`
	PowerMin          *int       `json:"power_min,omitempty" db:"power_min"`
	InsideTempAvg     *float64   `json:"inside_temp_avg,omitempty" db:"inside_temp_avg"`
	OutsideTempAvg    *float64   `json:"outside_temp_avg,omitempty" db:"outside_temp_avg"`
}

// ChargingProcess 充电记录
type ChargingProcess struct {
	ID                 int64      `json:"id" db:"id"`
	CarID              int64      `json:"car_id" db:"car_id"`
	PositionID         *int64     `json:"position_id,omitempty" db:"position_id"`
	GeofenceID         *int64     `json:"geofence_id,omitempty" db:"geofence_id"`
	StartTime          time.Time  `json:"start_time" db:"start_time"`
	EndTime            *time.Time `json:"end_time,omitempty" db:"end_time"`
	StartBatteryLevel  int        `json:"start_battery_level" db:"start_battery_level"`
	EndBatteryLevel    *int       `json:"end_battery_level,omitempty" db:"end_battery_level"`
	StartRangeKm       float64    `json:"start_range_km" db:"start_range_km"`
	EndRangeKm         *float64   `json:"end_range_km,omitempty" db:"end_range_km"`
	ChargeEnergyAdded  float64    `json:"charge_energy_added" db:"charge_energy_added"` // kWh
	ChargerPowerMax    *int       `json:"charger_power_max,omitempty" db:"charger_power_max"`
	DurationMin        float64    `json:"duration_min" db:"duration_min"`
	OutsideTempAvg     *float64   `json:"outside_temp_avg,omitempty" db:"outside_temp_avg"`
	Cost               *float64   `json:"cost,omitempty" db:"cost"`
}

// Charge 充电详情 (每分钟记录)
type Charge struct {
	ID                int64     `json:"id" db:"id"`
	ChargingProcessID int64     `json:"charging_process_id" db:"charging_process_id"`
	BatteryLevel      int       `json:"battery_level" db:"battery_level"`
	UsableBatteryLevel int      `json:"usable_battery_level" db:"usable_battery_level"`
	RangeKm           float64   `json:"range_km" db:"range_km"`
	ChargerPower      int       `json:"charger_power" db:"charger_power"`
	ChargerVoltage    int       `json:"charger_voltage" db:"charger_voltage"`
	ChargerCurrent    int       `json:"charger_current" db:"charger_current"`
	ChargeEnergyAdded float64   `json:"charge_energy_added" db:"charge_energy_added"`
	OutsideTemp       *float64  `json:"outside_temp,omitempty" db:"outside_temp"`
	RecordedAt        time.Time `json:"recorded_at" db:"recorded_at"`
}

// State 车辆状态记录
type State struct {
	ID        int64     `json:"id" db:"id"`
	CarID     int64     `json:"car_id" db:"car_id"`
	State     string    `json:"state" db:"state"` // online, asleep, charging, driving, updating
	StartTime time.Time `json:"start_time" db:"start_time"`
	EndTime   *time.Time `json:"end_time,omitempty" db:"end_time"`
}

// Geofence 地理围栏
type Geofence struct {
	ID        int64   `json:"id" db:"id"`
	Name      string  `json:"name" db:"name"`
	Latitude  float64 `json:"latitude" db:"latitude"`
	Longitude float64 `json:"longitude" db:"longitude"`
	Radius    int     `json:"radius" db:"radius"` // 米
}

// Settings 设置
type Settings struct {
	ID    int64  `json:"id" db:"id"`
	CarID int64  `json:"car_id" db:"car_id"`
	Key   string `json:"key" db:"key"`
	Value string `json:"value" db:"value"`
}

// Token 存储的认证令牌
type Token struct {
	ID           int64     `json:"id" db:"id"`
	AccessToken  string    `json:"access_token" db:"access_token"`
	RefreshToken string    `json:"refresh_token" db:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}
