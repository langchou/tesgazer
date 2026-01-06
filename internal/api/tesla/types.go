package tesla

import "time"

// Vehicle 车辆基础信息
type Vehicle struct {
	ID          int64    `json:"id"`
	VehicleID   int64    `json:"vehicle_id"`
	VIN         string   `json:"vin"`
	DisplayName string   `json:"display_name"`
	State       string   `json:"state"` // online, asleep, offline
	InService   bool     `json:"in_service"`
	Color       string   `json:"color,omitempty"`
	Tokens      []string `json:"tokens,omitempty"`
}

// VehicleData 车辆完整数据
type VehicleData struct {
	ID            int64          `json:"id"`
	VehicleID     int64          `json:"vehicle_id"`
	VIN           string         `json:"vin"`
	DisplayName   string         `json:"display_name"`
	State         string         `json:"state"`
	ChargeState   *ChargeState   `json:"charge_state,omitempty"`
	ClimateState  *ClimateState  `json:"climate_state,omitempty"`
	DriveState    *DriveState    `json:"drive_state,omitempty"`
	VehicleState  *VehicleState  `json:"vehicle_state,omitempty"`
	VehicleConfig *VehicleConfig `json:"vehicle_config,omitempty"`
}

// ChargeState 充电状态
type ChargeState struct {
	BatteryLevel           int       `json:"battery_level"`
	UsableBatteryLevel     int       `json:"usable_battery_level"`
	BatteryRange           float64   `json:"battery_range"`            // 英里
	EstBatteryRange        float64   `json:"est_battery_range"`        // 英里
	IdealBatteryRange      float64   `json:"ideal_battery_range"`      // 英里
	ChargeLimitSoc         int       `json:"charge_limit_soc"`
	ChargeLimitSocMin      int       `json:"charge_limit_soc_min"`
	ChargeLimitSocMax      int       `json:"charge_limit_soc_max"`
	ChargeLimitSocStd      int       `json:"charge_limit_soc_std"`
	ChargePortDoorOpen     bool      `json:"charge_port_door_open"`
	ChargePortLatch        string    `json:"charge_port_latch"`
	ChargingState          string    `json:"charging_state"` // Disconnected, Stopped, Charging, Complete
	ChargerPower           int       `json:"charger_power"`  // kW
	ChargerVoltage         int       `json:"charger_voltage"`
	ChargerActualCurrent   int       `json:"charger_actual_current"`
	ChargerPilotCurrent    int       `json:"charger_pilot_current"`
	ChargeCurrentRequest   int       `json:"charge_current_request"`
	ChargeCurrentRequestMax int      `json:"charge_current_request_max"`
	ChargeEnergyAdded      float64   `json:"charge_energy_added"` // kWh
	ChargeRateKmPerHour    float64   `json:"charge_rate"`         // 英里/小时
	TimeToFullCharge       float64   `json:"time_to_full_charge"` // 小时
	ScheduledChargingMode  string    `json:"scheduled_charging_mode"`
	ScheduledChargingStartTime *int64 `json:"scheduled_charging_start_time,omitempty"`
	Timestamp              int64     `json:"timestamp"`
}

// ClimateState 空调状态
type ClimateState struct {
	InsideTemp              float64 `json:"inside_temp"`  // 摄氏度
	OutsideTemp             float64 `json:"outside_temp"` // 摄氏度
	DriverTempSetting       float64 `json:"driver_temp_setting"`
	PassengerTempSetting    float64 `json:"passenger_temp_setting"`
	IsAutoConditioningOn    bool    `json:"is_auto_conditioning_on"`
	IsClimateOn             bool    `json:"is_climate_on"`
	IsPreconditioning       bool    `json:"is_preconditioning"`
	IsFrontDefrosterOn      bool    `json:"is_front_defroster_on"`
	IsRearDefrosterOn       bool    `json:"is_rear_defroster_on"`
	FanStatus               int     `json:"fan_status"`
	SeatHeaterLeft          int     `json:"seat_heater_left"`
	SeatHeaterRight         int     `json:"seat_heater_right"`
	SeatHeaterRearLeft      int     `json:"seat_heater_rear_left"`
	SeatHeaterRearRight     int     `json:"seat_heater_rear_right"`
	BatteryHeater           bool    `json:"battery_heater"`
	BatteryHeaterNoPower    *bool   `json:"battery_heater_no_power,omitempty"`
	Timestamp               int64   `json:"timestamp"`
}

// DriveState 驾驶状态
type DriveState struct {
	Latitude       float64 `json:"latitude"`
	Longitude      float64 `json:"longitude"`
	Heading        int     `json:"heading"`
	GpsAsOf        int64   `json:"gps_as_of"`
	NativeLatitude float64 `json:"native_latitude"`
	NativeLongitude float64 `json:"native_longitude"`
	NativeType     string  `json:"native_type"`
	Speed          *int    `json:"speed,omitempty"` // 英里/小时, nil 表示停止
	Power          int     `json:"power"`           // kW
	ShiftState     *string `json:"shift_state,omitempty"` // D, R, P, N
	Timestamp      int64   `json:"timestamp"`
}

// VehicleState 车辆状态
type VehicleState struct {
	APIVersion              int     `json:"api_version"`
	Odometer                float64 `json:"odometer"` // 英里
	Locked                  bool    `json:"locked"`
	SentryMode              bool    `json:"sentry_mode"`
	SentryModeAvailable     bool    `json:"sentry_mode_available"`
	ValetMode               bool    `json:"valet_mode"`
	SoftwareUpdate          *SoftwareUpdate `json:"software_update,omitempty"`
	SpeedLimitMode          *SpeedLimitMode `json:"speed_limit_mode,omitempty"`
	CenterDisplayState      int     `json:"center_display_state"`
	DriverDoorOpen          bool    `json:"df"` // driver front
	PassengerDoorOpen       bool    `json:"pf"` // passenger front
	DriverRearDoorOpen      bool    `json:"dr"` // driver rear
	PassengerRearDoorOpen   bool    `json:"pr"` // passenger rear
	FrunkOpen               bool    `json:"ft"` // front trunk
	TrunkOpen               bool    `json:"rt"` // rear trunk
	DriverWindowOpen        int     `json:"fd_window"`
	PassengerWindowOpen     int     `json:"fp_window"`
	DriverRearWindowOpen    int     `json:"rd_window"`
	PassengerRearWindowOpen int     `json:"rp_window"`
	IsUserPresent           bool    `json:"is_user_present"`
	VehicleName             string  `json:"vehicle_name"`
	Timestamp               int64   `json:"timestamp"`
}

// SoftwareUpdate 软件更新信息
type SoftwareUpdate struct {
	DownloadPerc        int    `json:"download_perc"`
	ExpectedDurationSec int    `json:"expected_duration_sec"`
	InstallPerc         int    `json:"install_perc"`
	Status              string `json:"status"`
	Version             string `json:"version"`
}

// SpeedLimitMode 限速模式
type SpeedLimitMode struct {
	Active          bool    `json:"active"`
	CurrentLimitMph float64 `json:"current_limit_mph"`
	MaxLimitMph     float64 `json:"max_limit_mph"`
	MinLimitMph     float64 `json:"min_limit_mph"`
	PinCodeSet      bool    `json:"pin_code_set"`
}

// VehicleConfig 车辆配置
type VehicleConfig struct {
	CarType             string `json:"car_type"`
	ExteriorColor       string `json:"exterior_color"`
	HasAirSuspension    bool   `json:"has_air_suspension"`
	HasLudicrousMode    bool   `json:"has_ludicrous_mode"`
	MotorizedChargePort bool   `json:"motorized_charge_port"`
	Plg                 bool   `json:"plg"` // performance legacy
	RearSeatHeaters     int    `json:"rear_seat_heaters"`
	RearSeatType        int    `json:"rear_seat_type"`
	Rhd                 bool   `json:"rhd"` // right hand drive
	RoofColor           string `json:"roof_color"`
	SeatType            int    `json:"seat_type"`
	SpoilerType         string `json:"spoiler_type"`
	SunRoofInstalled    int    `json:"sun_roof_installed"`
	ThirdRowSeats       string `json:"third_row_seats"`
	Timestamp           int64  `json:"timestamp"`
	TrimBadging         string `json:"trim_badging"`
	WheelType           string `json:"wheel_type"`
}

// Helper functions

// MilesToKm 英里转公里
func MilesToKm(miles float64) float64 {
	return miles * 1.60934
}

// KmToMiles 公里转英里
func KmToMiles(km float64) float64 {
	return km / 1.60934
}

// ParseTimestamp 解析 Tesla API 时间戳 (毫秒)
func ParseTimestamp(ts int64) time.Time {
	return time.UnixMilli(ts)
}
