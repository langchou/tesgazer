package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/langchou/tesgazer/internal/models"
)

// ChargeRepository 充电数据仓库
type ChargeRepository struct {
	db *DB
}

// NewChargeRepository 创建充电仓库
func NewChargeRepository(db *DB) *ChargeRepository {
	return &ChargeRepository{db: db}
}

// CreateProcess 创建充电过程
func (r *ChargeRepository) CreateProcess(ctx context.Context, cp *models.ChargingProcess) error {
	query := `
		INSERT INTO charging_processes (car_id, position_id, geofence_id, start_time, start_battery_level, start_range_km, address)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`
	err := r.db.Pool.QueryRow(ctx, query,
		cp.CarID,
		cp.PositionID,
		cp.GeofenceID,
		cp.StartTime,
		cp.StartBatteryLevel,
		cp.StartRangeKm,
		cp.Address,
	).Scan(&cp.ID)

	if err != nil {
		return fmt.Errorf("insert charging process: %w", err)
	}
	return nil
}

// CompleteProcess 完成充电过程
func (r *ChargeRepository) CompleteProcess(ctx context.Context, cp *models.ChargingProcess) error {
	query := `
		UPDATE charging_processes SET
			end_time = $1,
			end_battery_level = $2,
			end_range_km = $3,
			charge_energy_added = $4,
			charger_power_max = $5,
			duration_min = $6,
			outside_temp_avg = $7
		WHERE id = $8
	`
	_, err := r.db.Pool.Exec(ctx, query,
		cp.EndTime,
		cp.EndBatteryLevel,
		cp.EndRangeKm,
		cp.ChargeEnergyAdded,
		cp.ChargerPowerMax,
		cp.DurationMin,
		cp.OutsideTempAvg,
		cp.ID,
	)
	if err != nil {
		return fmt.Errorf("complete charging process: %w", err)
	}
	return nil
}

// UpdateSnapshot 更新活跃充电过程的快照信息
func (r *ChargeRepository) UpdateSnapshot(ctx context.Context, cp *models.ChargingProcess) error {
	query := `
		UPDATE charging_processes SET
			end_battery_level = $2,
			end_range_km = $3,
			charge_energy_added = $4,
			charger_power_max = $5,
			outside_temp_avg = $6,
			duration_min = $7
		WHERE id = $1 AND end_time IS NULL
	`
	_, err := r.db.Pool.Exec(ctx, query,
		cp.ID,
		cp.EndBatteryLevel,
		cp.EndRangeKm,
		cp.ChargeEnergyAdded,
		cp.ChargerPowerMax,
		cp.OutsideTempAvg,
		cp.DurationMin,
	)
	if err != nil {
		return fmt.Errorf("update charging snapshot: %w", err)
	}
	return nil
}

// CreateCharge 创建充电详情记录
func (r *ChargeRepository) CreateCharge(ctx context.Context, c *models.Charge) error {
	query := `
		INSERT INTO charges (charging_process_id, battery_level, usable_battery_level, range_km, charger_power, charger_voltage, charger_current, charge_energy_added, outside_temp, recorded_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`
	err := r.db.Pool.QueryRow(ctx, query,
		c.ChargingProcessID,
		c.BatteryLevel,
		c.UsableBatteryLevel,
		c.RangeKm,
		c.ChargerPower,
		c.ChargerVoltage,
		c.ChargerCurrent,
		c.ChargeEnergyAdded,
		c.OutsideTemp,
		c.RecordedAt,
	).Scan(&c.ID)

	if err != nil {
		return fmt.Errorf("insert charge: %w", err)
	}
	return nil
}

// GetProcessByID 获取充电过程
func (r *ChargeRepository) GetProcessByID(ctx context.Context, id int64) (*models.ChargingProcess, error) {
	query := `
		SELECT id, car_id, position_id, geofence_id, start_time, end_time, start_battery_level, end_battery_level,
			start_range_km, end_range_km, charge_energy_added, charger_power_max, duration_min, outside_temp_avg, cost, address
		FROM charging_processes WHERE id = $1
	`
	cp := &models.ChargingProcess{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&cp.ID,
		&cp.CarID,
		&cp.PositionID,
		&cp.GeofenceID,
		&cp.StartTime,
		&cp.EndTime,
		&cp.StartBatteryLevel,
		&cp.EndBatteryLevel,
		&cp.StartRangeKm,
		&cp.EndRangeKm,
		&cp.ChargeEnergyAdded,
		&cp.ChargerPowerMax,
		&cp.DurationMin,
		&cp.OutsideTempAvg,
		&cp.Cost,
		&cp.Address,
	)
	if err != nil {
		return nil, fmt.Errorf("get charging process: %w", err)
	}
	return cp, nil
}

// ListProcessesByCarID 获取车辆充电记录列表
func (r *ChargeRepository) ListProcessesByCarID(ctx context.Context, carID int64, limit, offset int) ([]*models.ChargingProcess, error) {
	query := `
		SELECT id, car_id, position_id, geofence_id, start_time, end_time, start_battery_level, end_battery_level,
			start_range_km, end_range_km, charge_energy_added, charger_power_max, duration_min, outside_temp_avg, cost, address
		FROM charging_processes WHERE car_id = $1 ORDER BY start_time DESC LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Pool.Query(ctx, query, carID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list charging processes: %w", err)
	}
	defer rows.Close()

	var processes []*models.ChargingProcess
	for rows.Next() {
		cp := &models.ChargingProcess{}
		err := rows.Scan(
			&cp.ID,
			&cp.CarID,
			&cp.PositionID,
			&cp.GeofenceID,
			&cp.StartTime,
			&cp.EndTime,
			&cp.StartBatteryLevel,
			&cp.EndBatteryLevel,
			&cp.StartRangeKm,
			&cp.EndRangeKm,
			&cp.ChargeEnergyAdded,
			&cp.ChargerPowerMax,
			&cp.DurationMin,
			&cp.OutsideTempAvg,
			&cp.Cost,
			&cp.Address,
		)
		if err != nil {
			return nil, fmt.Errorf("scan charging process: %w", err)
		}
		processes = append(processes, cp)
	}

	return processes, nil
}

// GetActiveProcess 获取进行中的充电
func (r *ChargeRepository) GetActiveProcess(ctx context.Context, carID int64) (*models.ChargingProcess, error) {
	query := `
		SELECT id, car_id, position_id, geofence_id, start_time, end_time, start_battery_level, end_battery_level,
			start_range_km, end_range_km, charge_energy_added, charger_power_max, duration_min, outside_temp_avg, cost, address
		FROM charging_processes WHERE car_id = $1 AND end_time IS NULL ORDER BY start_time DESC LIMIT 1
	`
	cp := &models.ChargingProcess{}
	err := r.db.Pool.QueryRow(ctx, query, carID).Scan(
		&cp.ID,
		&cp.CarID,
		&cp.PositionID,
		&cp.GeofenceID,
		&cp.StartTime,
		&cp.EndTime,
		&cp.StartBatteryLevel,
		&cp.EndBatteryLevel,
		&cp.StartRangeKm,
		&cp.EndRangeKm,
		&cp.ChargeEnergyAdded,
		&cp.ChargerPowerMax,
		&cp.DurationMin,
		&cp.OutsideTempAvg,
		&cp.Cost,
		&cp.Address,
	)
	if err != nil {
		return nil, err
	}
	return cp, nil
}

// ListChargesByProcessID 获取充电详情列表
func (r *ChargeRepository) ListChargesByProcessID(ctx context.Context, processID int64) ([]*models.Charge, error) {
	query := `
		SELECT id, charging_process_id, battery_level, usable_battery_level, range_km, charger_power, charger_voltage, charger_current, charge_energy_added, outside_temp, recorded_at
		FROM charges WHERE charging_process_id = $1 ORDER BY recorded_at
	`
	rows, err := r.db.Pool.Query(ctx, query, processID)
	if err != nil {
		return nil, fmt.Errorf("list charges: %w", err)
	}
	defer rows.Close()

	var charges []*models.Charge
	for rows.Next() {
		c := &models.Charge{}
		err := rows.Scan(
			&c.ID,
			&c.ChargingProcessID,
			&c.BatteryLevel,
			&c.UsableBatteryLevel,
			&c.RangeKm,
			&c.ChargerPower,
			&c.ChargerVoltage,
			&c.ChargerCurrent,
			&c.ChargeEnergyAdded,
			&c.OutsideTemp,
			&c.RecordedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan charge: %w", err)
		}
		charges = append(charges, c)
	}

	return charges, nil
}

// CountProcessesByCarID 统计车辆充电次数
func (r *ChargeRepository) CountProcessesByCarID(ctx context.Context, carID int64) (int64, error) {
	var count int64
	err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM charging_processes WHERE car_id = $1 AND end_time IS NOT NULL`, carID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count charging processes: %w", err)
	}
	return count, nil
}

// GetStats 获取充电统计
func (r *ChargeRepository) GetStats(ctx context.Context, carID int64, since time.Time) (totalEnergy float64, count int64, err error) {
	query := `
		SELECT COALESCE(SUM(charge_energy_added), 0), COUNT(*)
		FROM charging_processes WHERE car_id = $1 AND start_time >= $2 AND end_time IS NOT NULL
	`
	err = r.db.Pool.QueryRow(ctx, query, carID, since).Scan(&totalEnergy, &count)
	if err != nil {
		err = fmt.Errorf("get charge stats: %w", err)
	}
	return
}
