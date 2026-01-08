package models

import "time"

// ChargingProcess 充电记录
type ChargingProcess struct {
	ID                int64      `json:"id" db:"id"`
	CarID             int64      `json:"car_id" db:"car_id"`
	PositionID        *int64     `json:"position_id,omitempty" db:"position_id"`
	GeofenceID        *int64     `json:"geofence_id,omitempty" db:"geofence_id"`
	Address           *Address   `json:"address,omitempty" db:"address"` // 结构化地址
	StartTime         time.Time  `json:"start_time" db:"start_time"`
	EndTime           *time.Time `json:"end_time,omitempty" db:"end_time"`
	StartBatteryLevel int        `json:"start_battery_level" db:"start_battery_level"`
	EndBatteryLevel   *int       `json:"end_battery_level,omitempty" db:"end_battery_level"`
	StartRangeKm      float64    `json:"start_range_km" db:"start_range_km"`
	EndRangeKm        *float64   `json:"end_range_km,omitempty" db:"end_range_km"`
	ChargeEnergyAdded float64    `json:"charge_energy_added" db:"charge_energy_added"` // kWh
	ChargerPowerMax   *int       `json:"charger_power_max,omitempty" db:"charger_power_max"`
	DurationMin       float64    `json:"duration_min" db:"duration_min"`
	OutsideTempAvg    *float64   `json:"outside_temp_avg,omitempty" db:"outside_temp_avg"`
	Cost              *float64   `json:"cost,omitempty" db:"cost"`
}

// Charge 充电详情 (每分钟记录)
type Charge struct {
	ID                 int64     `json:"id" db:"id"`
	ChargingProcessID  int64     `json:"charging_process_id" db:"charging_process_id"`
	BatteryLevel       int       `json:"battery_level" db:"battery_level"`
	UsableBatteryLevel int       `json:"usable_battery_level" db:"usable_battery_level"`
	RangeKm            float64   `json:"range_km" db:"range_km"`
	ChargerPower       int       `json:"charger_power" db:"charger_power"`
	ChargerVoltage     int       `json:"charger_voltage" db:"charger_voltage"`
	ChargerCurrent     int       `json:"charger_current" db:"charger_current"`
	ChargeEnergyAdded  float64   `json:"charge_energy_added" db:"charge_energy_added"`
	OutsideTemp        *float64  `json:"outside_temp,omitempty" db:"outside_temp"`
	RecordedAt         time.Time `json:"recorded_at" db:"recorded_at"`
}
