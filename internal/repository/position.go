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
