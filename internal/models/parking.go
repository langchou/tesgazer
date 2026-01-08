package models

import "time"

// Parking 停车记录
type Parking struct {
	ID          int64      `json:"id" db:"id"`
	CarID       int64      `json:"car_id" db:"car_id"`
	PositionID  *int64     `json:"position_id,omitempty" db:"position_id"`
	GeofenceID  *int64     `json:"geofence_id,omitempty" db:"geofence_id"`
	StartTime   time.Time  `json:"start_time" db:"start_time"`
	EndTime     *time.Time `json:"end_time,omitempty" db:"end_time"`
	DurationMin float64    `json:"duration_min" db:"duration_min"`

	// 位置
	Latitude  float64  `json:"latitude" db:"latitude"`
	Longitude float64  `json:"longitude" db:"longitude"`
	Address   *Address `json:"address,omitempty" db:"address"` // 结构化地址 (JSONB)

	// 电量变化
	StartBatteryLevel int      `json:"start_battery_level" db:"start_battery_level"`
	EndBatteryLevel   *int     `json:"end_battery_level,omitempty" db:"end_battery_level"`
	StartRangeKm      float64  `json:"start_range_km" db:"start_range_km"`
	EndRangeKm        *float64 `json:"end_range_km,omitempty" db:"end_range_km"`
	StartOdometer     float64  `json:"start_odometer" db:"start_odometer"` // km
	EndOdometer       *float64 `json:"end_odometer,omitempty" db:"end_odometer"`

	// 吸血鬼功耗 (vampire drain)
	EnergyUsedKwh *float64 `json:"energy_used_kwh,omitempty" db:"energy_used_kwh"` // 停车期间消耗的电量

	// 温度
	StartInsideTemp  *float64 `json:"start_inside_temp,omitempty" db:"start_inside_temp"`
	EndInsideTemp    *float64 `json:"end_inside_temp,omitempty" db:"end_inside_temp"`
	StartOutsideTemp *float64 `json:"start_outside_temp,omitempty" db:"start_outside_temp"`
	EndOutsideTemp   *float64 `json:"end_outside_temp,omitempty" db:"end_outside_temp"`
	InsideTempAvg    *float64 `json:"inside_temp_avg,omitempty" db:"inside_temp_avg"`
	OutsideTempAvg   *float64 `json:"outside_temp_avg,omitempty" db:"outside_temp_avg"`

	// 空调使用情况
	ClimateUsedMin *float64 `json:"climate_used_min,omitempty" db:"climate_used_min"` // 空调使用时长 (分钟)

	// 哨兵模式
	SentryModeUsedMin *float64 `json:"sentry_mode_used_min,omitempty" db:"sentry_mode_used_min"` // 哨兵模式使用时长 (分钟)

	// 起始状态快照
	StartLocked        bool `json:"start_locked" db:"start_locked"`
	StartSentryMode    bool `json:"start_sentry_mode" db:"start_sentry_mode"`
	StartDoorsOpen     bool `json:"start_doors_open" db:"start_doors_open"`
	StartWindowsOpen   bool `json:"start_windows_open" db:"start_windows_open"`
	StartFrunkOpen     bool `json:"start_frunk_open" db:"start_frunk_open"`
	StartTrunkOpen     bool `json:"start_trunk_open" db:"start_trunk_open"`
	StartIsClimateOn   bool `json:"start_is_climate_on" db:"start_is_climate_on"`
	StartIsUserPresent bool `json:"start_is_user_present" db:"start_is_user_present"`

	// 结束状态快照
	EndLocked        *bool `json:"end_locked,omitempty" db:"end_locked"`
	EndSentryMode    *bool `json:"end_sentry_mode,omitempty" db:"end_sentry_mode"`
	EndDoorsOpen     *bool `json:"end_doors_open,omitempty" db:"end_doors_open"`
	EndWindowsOpen   *bool `json:"end_windows_open,omitempty" db:"end_windows_open"`
	EndFrunkOpen     *bool `json:"end_frunk_open,omitempty" db:"end_frunk_open"`
	EndTrunkOpen     *bool `json:"end_trunk_open,omitempty" db:"end_trunk_open"`
	EndIsClimateOn   *bool `json:"end_is_climate_on,omitempty" db:"end_is_climate_on"`
	EndIsUserPresent *bool `json:"end_is_user_present,omitempty" db:"end_is_user_present"`

	// 胎压 (开始)
	StartTpmsPressureFL *float64 `json:"start_tpms_pressure_fl,omitempty" db:"start_tpms_pressure_fl"`
	StartTpmsPressureFR *float64 `json:"start_tpms_pressure_fr,omitempty" db:"start_tpms_pressure_fr"`
	StartTpmsPressureRL *float64 `json:"start_tpms_pressure_rl,omitempty" db:"start_tpms_pressure_rl"`
	StartTpmsPressureRR *float64 `json:"start_tpms_pressure_rr,omitempty" db:"start_tpms_pressure_rr"`

	// 胎压 (结束)
	EndTpmsPressureFL *float64 `json:"end_tpms_pressure_fl,omitempty" db:"end_tpms_pressure_fl"`
	EndTpmsPressureFR *float64 `json:"end_tpms_pressure_fr,omitempty" db:"end_tpms_pressure_fr"`
	EndTpmsPressureRL *float64 `json:"end_tpms_pressure_rl,omitempty" db:"end_tpms_pressure_rl"`
	EndTpmsPressureRR *float64 `json:"end_tpms_pressure_rr,omitempty" db:"end_tpms_pressure_rr"`

	// 软件版本
	CarVersion string `json:"car_version,omitempty" db:"car_version"`
}
