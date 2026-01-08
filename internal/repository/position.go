package repository

import (
	"context"
	"fmt"

	"github.com/langchou/tesgazer/internal/models"
)

// PositionRepository 位置数据仓库
type PositionRepository struct {
	db *DB
}

// NewPositionRepository 创建位置仓库
func NewPositionRepository(db *DB) *PositionRepository {
	return &PositionRepository{db: db}
}

// Create 创建位置记录
func (r *PositionRepository) Create(ctx context.Context, pos *models.Position) error {
	query := `
		INSERT INTO positions (car_id, drive_id, latitude, longitude, heading, speed, power, odometer, battery_level, range_km, inside_temp, outside_temp, elevation, tpms_pressure_fl, tpms_pressure_fr, tpms_pressure_rl, tpms_pressure_rr, recorded_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		RETURNING id
	`
	err := r.db.Pool.QueryRow(ctx, query,
		pos.CarID,
		pos.DriveID,
		pos.Latitude,
		pos.Longitude,
		pos.Heading,
		pos.Speed,
		pos.Power,
		pos.Odometer,
		pos.BatteryLevel,
		pos.RangeKm,
		pos.InsideTemp,
		pos.OutsideTemp,
		pos.Elevation,
		pos.TpmsPressureFL,
		pos.TpmsPressureFR,
		pos.TpmsPressureRL,
		pos.TpmsPressureRR,
		pos.RecordedAt,
	).Scan(&pos.ID)

	if err != nil {
		return fmt.Errorf("insert position: %w", err)
	}
	return nil
}

// GetLatestByCarID 获取车辆最新位置
func (r *PositionRepository) GetLatestByCarID(ctx context.Context, carID int64) (*models.Position, error) {
	query := `
		SELECT id, car_id, drive_id, latitude, longitude, heading, speed, power, odometer, battery_level, range_km, inside_temp, outside_temp, elevation, tpms_pressure_fl, tpms_pressure_fr, tpms_pressure_rl, tpms_pressure_rr, recorded_at
		FROM positions WHERE car_id = $1 ORDER BY recorded_at DESC LIMIT 1
	`
	pos := &models.Position{}
	err := r.db.Pool.QueryRow(ctx, query, carID).Scan(
		&pos.ID,
		&pos.CarID,
		&pos.DriveID,
		&pos.Latitude,
		&pos.Longitude,
		&pos.Heading,
		&pos.Speed,
		&pos.Power,
		&pos.Odometer,
		&pos.BatteryLevel,
		&pos.RangeKm,
		&pos.InsideTemp,
		&pos.OutsideTemp,
		&pos.Elevation,
		&pos.TpmsPressureFL,
		&pos.TpmsPressureFR,
		&pos.TpmsPressureRL,
		&pos.TpmsPressureRR,
		&pos.RecordedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get latest position: %w", err)
	}
	return pos, nil
}

// ListByDriveID 获取行程的所有位置
func (r *PositionRepository) ListByDriveID(ctx context.Context, driveID int64) ([]*models.Position, error) {
	query := `
		SELECT id, car_id, drive_id, latitude, longitude, heading, speed, power, odometer, battery_level, range_km, inside_temp, outside_temp, elevation, tpms_pressure_fl, tpms_pressure_fr, tpms_pressure_rl, tpms_pressure_rr, recorded_at
		FROM positions WHERE drive_id = $1 ORDER BY recorded_at
	`
	rows, err := r.db.Pool.Query(ctx, query, driveID)
	if err != nil {
		return nil, fmt.Errorf("list positions by drive: %w", err)
	}
	defer rows.Close()

	var positions []*models.Position
	for rows.Next() {
		pos := &models.Position{}
		err := rows.Scan(
			&pos.ID,
			&pos.CarID,
			&pos.DriveID,
			&pos.Latitude,
			&pos.Longitude,
			&pos.Heading,
			&pos.Speed,
			&pos.Power,
			&pos.Odometer,
			&pos.BatteryLevel,
			&pos.RangeKm,
			&pos.InsideTemp,
			&pos.OutsideTemp,
			&pos.Elevation,
			&pos.TpmsPressureFL,
			&pos.TpmsPressureFR,
			&pos.TpmsPressureRL,
			&pos.TpmsPressureRR,
			&pos.RecordedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan position: %w", err)
		}
		positions = append(positions, pos)
	}

	return positions, nil
}

// UpdateDriveID 更新位置的行程 ID
func (r *PositionRepository) UpdateDriveID(ctx context.Context, positionID, driveID int64) error {
	query := `UPDATE positions SET drive_id = $1 WHERE id = $2`
	_, err := r.db.Pool.Exec(ctx, query, driveID, positionID)
	if err != nil {
		return fmt.Errorf("update position drive_id: %w", err)
	}
	return nil
}

// DriveStats 行程统计数据
type DriveStats struct {
	SpeedMax       *int     // 最高速度 (km/h)
	PowerMax       *int     // 最大功率 (kW，正值=耗电)
	PowerMin       *int     // 最小功率 (kW，负值=回收)
	InsideTempAvg  *float64 // 平均车内温度
	OutsideTempAvg *float64 // 平均车外温度
	EnergyUsedKwh  *float64 // 总耗电量 (kWh)
	EnergyRegenKwh *float64 // 总回收电量 (kWh)
}

// GetDriveStats 获取行程统计数据
func (r *PositionRepository) GetDriveStats(ctx context.Context, driveID int64) (*DriveStats, error) {
	query := `
		SELECT
			MAX(speed) as speed_max,
			MAX(power) as power_max,
			MIN(power) as power_min,
			AVG(inside_temp) as inside_temp_avg,
			AVG(outside_temp) as outside_temp_avg
		FROM positions
		WHERE drive_id = $1
	`
	stats := &DriveStats{}
	err := r.db.Pool.QueryRow(ctx, query, driveID).Scan(
		&stats.SpeedMax,
		&stats.PowerMax,
		&stats.PowerMin,
		&stats.InsideTempAvg,
		&stats.OutsideTempAvg,
	)
	if err != nil {
		return nil, fmt.Errorf("get drive stats: %w", err)
	}

	// 计算能量消耗和回收（基于功率和时间间隔）
	// power 单位是 kW，时间间隔约 3 秒
	// 能量 = 功率 * 时间 = kW * (3/3600) h = kWh
	energyQuery := `
		WITH intervals AS (
			SELECT
				power,
				EXTRACT(EPOCH FROM (
					LEAD(recorded_at) OVER (ORDER BY recorded_at) - recorded_at
				)) as interval_seconds
			FROM positions
			WHERE drive_id = $1 AND power IS NOT NULL
		)
		SELECT
			COALESCE(SUM(CASE WHEN power > 0 THEN power * interval_seconds / 3600.0 ELSE 0 END), 0) as energy_used,
			COALESCE(SUM(CASE WHEN power < 0 THEN ABS(power) * interval_seconds / 3600.0 ELSE 0 END), 0) as energy_regen
		FROM intervals
		WHERE interval_seconds IS NOT NULL AND interval_seconds < 60
	`
	var energyUsed, energyRegen float64
	err = r.db.Pool.QueryRow(ctx, energyQuery, driveID).Scan(&energyUsed, &energyRegen)
	if err == nil {
		if energyUsed > 0 {
			stats.EnergyUsedKwh = &energyUsed
		}
		if energyRegen > 0 {
			stats.EnergyRegenKwh = &energyRegen
		}
	}

	return stats, nil
}
