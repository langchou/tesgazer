package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/langchou/tesgazer/internal/models"
)

// DriveRepository 行程数据仓库
type DriveRepository struct {
	db *DB
}

// NewDriveRepository 创建行程仓库
func NewDriveRepository(db *DB) *DriveRepository {
	return &DriveRepository{db: db}
}

// Create 创建行程
func (r *DriveRepository) Create(ctx context.Context, drive *models.Drive) error {
	query := `
		INSERT INTO drives (car_id, start_time, start_position_id, start_geofence_id, start_battery_level, start_range_km)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`
	err := r.db.Pool.QueryRow(ctx, query,
		drive.CarID,
		drive.StartTime,
		drive.StartPositionID,
		drive.StartGeofenceID,
		drive.StartBatteryLevel,
		drive.StartRangeKm,
	).Scan(&drive.ID)

	if err != nil {
		return fmt.Errorf("insert drive: %w", err)
	}
	return nil
}

// Complete 完成行程
func (r *DriveRepository) Complete(ctx context.Context, drive *models.Drive) error {
	query := `
		UPDATE drives SET
			end_time = $1,
			end_position_id = $2,
			end_geofence_id = $3,
			distance_km = $4,
			duration_min = $5,
			end_battery_level = $6,
			end_range_km = $7,
			speed_max = $8,
			power_max = $9,
			power_min = $10,
			inside_temp_avg = $11,
			outside_temp_avg = $12
		WHERE id = $13
	`
	_, err := r.db.Pool.Exec(ctx, query,
		drive.EndTime,
		drive.EndPositionID,
		drive.EndGeofenceID,
		drive.DistanceKm,
		drive.DurationMin,
		drive.EndBatteryLevel,
		drive.EndRangeKm,
		drive.SpeedMax,
		drive.PowerMax,
		drive.PowerMin,
		drive.InsideTempAvg,
		drive.OutsideTempAvg,
		drive.ID,
	)
	if err != nil {
		return fmt.Errorf("complete drive: %w", err)
	}
	return nil
}

// GetByID 获取行程
func (r *DriveRepository) GetByID(ctx context.Context, id int64) (*models.Drive, error) {
	query := `
		SELECT id, car_id, start_time, end_time, start_position_id, end_position_id, start_geofence_id, end_geofence_id,
			distance_km, duration_min, start_battery_level, end_battery_level, start_range_km, end_range_km,
			speed_max, power_max, power_min, inside_temp_avg, outside_temp_avg
		FROM drives WHERE id = $1
	`
	drive := &models.Drive{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&drive.ID,
		&drive.CarID,
		&drive.StartTime,
		&drive.EndTime,
		&drive.StartPositionID,
		&drive.EndPositionID,
		&drive.StartGeofenceID,
		&drive.EndGeofenceID,
		&drive.DistanceKm,
		&drive.DurationMin,
		&drive.StartBatteryLevel,
		&drive.EndBatteryLevel,
		&drive.StartRangeKm,
		&drive.EndRangeKm,
		&drive.SpeedMax,
		&drive.PowerMax,
		&drive.PowerMin,
		&drive.InsideTempAvg,
		&drive.OutsideTempAvg,
	)
	if err != nil {
		return nil, fmt.Errorf("get drive by id: %w", err)
	}
	return drive, nil
}

// ListByCarID 获取车辆的行程列表
func (r *DriveRepository) ListByCarID(ctx context.Context, carID int64, limit, offset int) ([]*models.Drive, error) {
	query := `
		SELECT id, car_id, start_time, end_time, start_position_id, end_position_id, start_geofence_id, end_geofence_id,
			distance_km, duration_min, start_battery_level, end_battery_level, start_range_km, end_range_km,
			speed_max, power_max, power_min, inside_temp_avg, outside_temp_avg
		FROM drives WHERE car_id = $1 ORDER BY start_time DESC LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Pool.Query(ctx, query, carID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list drives: %w", err)
	}
	defer rows.Close()

	var drives []*models.Drive
	for rows.Next() {
		drive := &models.Drive{}
		err := rows.Scan(
			&drive.ID,
			&drive.CarID,
			&drive.StartTime,
			&drive.EndTime,
			&drive.StartPositionID,
			&drive.EndPositionID,
			&drive.StartGeofenceID,
			&drive.EndGeofenceID,
			&drive.DistanceKm,
			&drive.DurationMin,
			&drive.StartBatteryLevel,
			&drive.EndBatteryLevel,
			&drive.StartRangeKm,
			&drive.EndRangeKm,
			&drive.SpeedMax,
			&drive.PowerMax,
			&drive.PowerMin,
			&drive.InsideTempAvg,
			&drive.OutsideTempAvg,
		)
		if err != nil {
			return nil, fmt.Errorf("scan drive: %w", err)
		}
		drives = append(drives, drive)
	}

	return drives, nil
}

// CountByCarID 统计车辆行程数
func (r *DriveRepository) CountByCarID(ctx context.Context, carID int64) (int64, error) {
	var count int64
	err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM drives WHERE car_id = $1`, carID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count drives: %w", err)
	}
	return count, nil
}

// GetActiveDrive 获取进行中的行程
func (r *DriveRepository) GetActiveDrive(ctx context.Context, carID int64) (*models.Drive, error) {
	query := `
		SELECT id, car_id, start_time, end_time, start_position_id, end_position_id, start_geofence_id, end_geofence_id,
			distance_km, duration_min, start_battery_level, end_battery_level, start_range_km, end_range_km,
			speed_max, power_max, power_min, inside_temp_avg, outside_temp_avg
		FROM drives WHERE car_id = $1 AND end_time IS NULL ORDER BY start_time DESC LIMIT 1
	`
	drive := &models.Drive{}
	err := r.db.Pool.QueryRow(ctx, query, carID).Scan(
		&drive.ID,
		&drive.CarID,
		&drive.StartTime,
		&drive.EndTime,
		&drive.StartPositionID,
		&drive.EndPositionID,
		&drive.StartGeofenceID,
		&drive.EndGeofenceID,
		&drive.DistanceKm,
		&drive.DurationMin,
		&drive.StartBatteryLevel,
		&drive.EndBatteryLevel,
		&drive.StartRangeKm,
		&drive.EndRangeKm,
		&drive.SpeedMax,
		&drive.PowerMax,
		&drive.PowerMin,
		&drive.InsideTempAvg,
		&drive.OutsideTempAvg,
	)
	if err != nil {
		return nil, err // 可能是没有进行中的行程
	}
	return drive, nil
}

// GetStats 获取行程统计
func (r *DriveRepository) GetStats(ctx context.Context, carID int64, since time.Time) (totalDistance float64, totalDuration float64, count int64, err error) {
	query := `
		SELECT COALESCE(SUM(distance_km), 0), COALESCE(SUM(duration_min), 0), COUNT(*)
		FROM drives WHERE car_id = $1 AND start_time >= $2 AND end_time IS NOT NULL
	`
	err = r.db.Pool.QueryRow(ctx, query, carID, since).Scan(&totalDistance, &totalDuration, &count)
	if err != nil {
		err = fmt.Errorf("get drive stats: %w", err)
	}
	return
}
