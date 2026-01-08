package models

import "time"

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
	StartOdometerKm   float64    `json:"start_odometer_km" db:"start_odometer_km"`         // 起始里程表 (km)
	EndOdometerKm     *float64   `json:"end_odometer_km,omitempty" db:"end_odometer_km"`   // 结束里程表 (km)
	SpeedMax          *int       `json:"speed_max,omitempty" db:"speed_max"`               // 最高速度 (km/h)
	PowerMax          *int       `json:"power_max,omitempty" db:"power_max"`               // 最大功率 (kW)
	PowerMin          *int       `json:"power_min,omitempty" db:"power_min"`               // 最小功率 (kW，负值=回收)
	InsideTempAvg     *float64   `json:"inside_temp_avg,omitempty" db:"inside_temp_avg"`   // 平均车内温度
	OutsideTempAvg    *float64   `json:"outside_temp_avg,omitempty" db:"outside_temp_avg"` // 平均车外温度
	EnergyUsedKwh     *float64   `json:"energy_used_kwh,omitempty" db:"energy_used_kwh"`   // 总耗电量 (kWh)
	EnergyRegenKwh    *float64   `json:"energy_regen_kwh,omitempty" db:"energy_regen_kwh"` // 动能回收电量 (kWh)
	// 起止地址 (逆地理编码，结构化数据)
	StartAddress *Address `json:"start_address,omitempty" db:"start_address"` // 起始地址
	EndAddress   *Address `json:"end_address,omitempty" db:"end_address"`     // 结束地址
	// 起止经纬度 (用于前端展示和逆地理编码)
	StartLatitude  *float64 `json:"start_latitude,omitempty" db:"start_latitude"`   // 起始纬度
	StartLongitude *float64 `json:"start_longitude,omitempty" db:"start_longitude"` // 起始经度
	EndLatitude    *float64 `json:"end_latitude,omitempty" db:"end_latitude"`       // 结束纬度
	EndLongitude   *float64 `json:"end_longitude,omitempty" db:"end_longitude"`     // 结束经度
}

// Position 位置记录
type Position struct {
	ID           int64    `json:"id" db:"id"`
	CarID        int64    `json:"car_id" db:"car_id"`
	DriveID      *int64   `json:"drive_id,omitempty" db:"drive_id"`
	Latitude     float64  `json:"latitude" db:"latitude"`
	Longitude    float64  `json:"longitude" db:"longitude"`
	Heading      int      `json:"heading" db:"heading"`
	Speed        *int     `json:"speed,omitempty" db:"speed"` // km/h
	Power        int      `json:"power" db:"power"`           // kW
	Odometer     float64  `json:"odometer" db:"odometer"`     // km
	BatteryLevel int      `json:"battery_level" db:"battery_level"`
	RangeKm      float64  `json:"range_km" db:"range_km"`
	InsideTemp   *float64 `json:"inside_temp,omitempty" db:"inside_temp"`
	OutsideTemp  *float64 `json:"outside_temp,omitempty" db:"outside_temp"`
	Elevation    *int     `json:"elevation,omitempty" db:"elevation"` // 海拔 (米)
	// TPMS 胎压数据 (bar)
	TpmsPressureFL *float64  `json:"tpms_pressure_fl,omitempty" db:"tpms_pressure_fl"` // 左前
	TpmsPressureFR *float64  `json:"tpms_pressure_fr,omitempty" db:"tpms_pressure_fr"` // 右前
	TpmsPressureRL *float64  `json:"tpms_pressure_rl,omitempty" db:"tpms_pressure_rl"` // 左后
	TpmsPressureRR *float64  `json:"tpms_pressure_rr,omitempty" db:"tpms_pressure_rr"` // 右后
	RecordedAt     time.Time `json:"recorded_at" db:"recorded_at"`
}
