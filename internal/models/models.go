package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// Address 结构化地址信息（用于逆地理编码结果）
type Address struct {
	FormattedAddress string `json:"formatted_address,omitempty"` // 完整格式化地址
	Country          string `json:"country,omitempty"`           // 国家
	Province         string `json:"province,omitempty"`          // 省
	City             string `json:"city,omitempty"`              // 市
	District         string `json:"district,omitempty"`          // 区/县
	Township         string `json:"township,omitempty"`          // 乡镇/街道
	Street           string `json:"street,omitempty"`            // 道路
	StreetNumber     string `json:"street_number,omitempty"`     // 门牌号
}

// Value 实现 driver.Valuer 接口，用于存储到数据库
func (a Address) Value() (driver.Value, error) {
	return json.Marshal(a)
}

// Scan 实现 sql.Scanner 接口，用于从数据库读取
func (a *Address) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, a)
}

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
	// TPMS 胎压数据 (bar)
	TpmsPressureFL *float64 `json:"tpms_pressure_fl,omitempty" db:"tpms_pressure_fl"` // 左前
	TpmsPressureFR *float64 `json:"tpms_pressure_fr,omitempty" db:"tpms_pressure_fr"` // 右前
	TpmsPressureRL *float64 `json:"tpms_pressure_rl,omitempty" db:"tpms_pressure_rl"` // 左后
	TpmsPressureRR *float64 `json:"tpms_pressure_rr,omitempty" db:"tpms_pressure_rr"` // 右后
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
	StartOdometerKm   float64    `json:"start_odometer_km" db:"start_odometer_km"`   // 起始里程表 (km)
	EndOdometerKm     *float64   `json:"end_odometer_km,omitempty" db:"end_odometer_km"` // 结束里程表 (km)
	SpeedMax          *int       `json:"speed_max,omitempty" db:"speed_max"`             // 最高速度 (km/h)
	PowerMax          *int       `json:"power_max,omitempty" db:"power_max"`             // 最大功率 (kW)
	PowerMin          *int       `json:"power_min,omitempty" db:"power_min"`             // 最小功率 (kW，负值=回收)
	InsideTempAvg     *float64   `json:"inside_temp_avg,omitempty" db:"inside_temp_avg"` // 平均车内温度
	OutsideTempAvg    *float64   `json:"outside_temp_avg,omitempty" db:"outside_temp_avg"` // 平均车外温度
	EnergyUsedKwh     *float64   `json:"energy_used_kwh,omitempty" db:"energy_used_kwh"`   // 总耗电量 (kWh)
	EnergyRegenKwh    *float64   `json:"energy_regen_kwh,omitempty" db:"energy_regen_kwh"` // 动能回收电量 (kWh)
	// 起止地址 (逆地理编码，结构化数据)
	StartAddress      *Address   `json:"start_address,omitempty" db:"start_address"`       // 起始地址
	EndAddress        *Address   `json:"end_address,omitempty" db:"end_address"`           // 结束地址
	// 起止经纬度 (用于前端展示和逆地理编码)
	StartLatitude     *float64   `json:"start_latitude,omitempty" db:"start_latitude"`     // 起始纬度
	StartLongitude    *float64   `json:"start_longitude,omitempty" db:"start_longitude"`   // 起始经度
	EndLatitude       *float64   `json:"end_latitude,omitempty" db:"end_latitude"`         // 结束纬度
	EndLongitude      *float64   `json:"end_longitude,omitempty" db:"end_longitude"`       // 结束经度
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
	Latitude  float64 `json:"latitude" db:"latitude"`
	Longitude float64 `json:"longitude" db:"longitude"`

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
