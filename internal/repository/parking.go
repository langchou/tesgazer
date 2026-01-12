package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/langchou/tesgazer/internal/models"
)

// ParkingRepository 停车数据仓库
type ParkingRepository struct {
	db *DB
}

// NewParkingRepository 创建停车仓库
func NewParkingRepository(db *DB) *ParkingRepository {
	return &ParkingRepository{db: db}
}

// Create 创建停车记录
func (r *ParkingRepository) Create(ctx context.Context, parking *models.Parking) error {
	query := `
		INSERT INTO parkings (
			car_id, position_id, geofence_id, start_time, latitude, longitude,
			start_battery_level, start_range_km, start_odometer,
			start_inside_temp, start_outside_temp,
			start_locked, start_sentry_mode, start_doors_open, start_windows_open,
			start_frunk_open, start_trunk_open, start_is_climate_on, start_is_user_present,
			start_tpms_pressure_fl, start_tpms_pressure_fr, start_tpms_pressure_rl, start_tpms_pressure_rr,
			car_version, address
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25)
		RETURNING id
	`
	err := r.db.Pool.QueryRow(ctx, query,
		parking.CarID,
		parking.PositionID,
		parking.GeofenceID,
		parking.StartTime,
		parking.Latitude,
		parking.Longitude,
		parking.StartBatteryLevel,
		parking.StartRangeKm,
		parking.StartOdometer,
		parking.StartInsideTemp,
		parking.StartOutsideTemp,
		parking.StartLocked,
		parking.StartSentryMode,
		parking.StartDoorsOpen,
		parking.StartWindowsOpen,
		parking.StartFrunkOpen,
		parking.StartTrunkOpen,
		parking.StartIsClimateOn,
		parking.StartIsUserPresent,
		parking.StartTpmsPressureFL,
		parking.StartTpmsPressureFR,
		parking.StartTpmsPressureRL,
		parking.StartTpmsPressureRR,
		parking.CarVersion,
		parking.Address,
	).Scan(&parking.ID)

	if err != nil {
		return fmt.Errorf("insert parking: %w", err)
	}
	return nil
}

// Complete 完成停车记录
func (r *ParkingRepository) Complete(ctx context.Context, parking *models.Parking) error {
	query := `
		UPDATE parkings SET
			end_time = $1,
			duration_min = $2,
			end_battery_level = $3,
			end_range_km = $4,
			end_odometer = $5,
			energy_used_kwh = $6,
			end_inside_temp = $7,
			end_outside_temp = $8,
			inside_temp_avg = $9,
			outside_temp_avg = $10,
			climate_used_min = $11,
			sentry_mode_used_min = $12,
			end_locked = $13,
			end_sentry_mode = $14,
			end_doors_open = $15,
			end_windows_open = $16,
			end_frunk_open = $17,
			end_trunk_open = $18,
			end_is_climate_on = $19,
			end_is_user_present = $20,
			end_tpms_pressure_fl = $21,
			end_tpms_pressure_fr = $22,
			end_tpms_pressure_rl = $23,
			end_tpms_pressure_rr = $24
		WHERE id = $25
	`
	_, err := r.db.Pool.Exec(ctx, query,
		parking.EndTime,
		parking.DurationMin,
		parking.EndBatteryLevel,
		parking.EndRangeKm,
		parking.EndOdometer,
		parking.EnergyUsedKwh,
		parking.EndInsideTemp,
		parking.EndOutsideTemp,
		parking.InsideTempAvg,
		parking.OutsideTempAvg,
		parking.ClimateUsedMin,
		parking.SentryModeUsedMin,
		parking.EndLocked,
		parking.EndSentryMode,
		parking.EndDoorsOpen,
		parking.EndWindowsOpen,
		parking.EndFrunkOpen,
		parking.EndTrunkOpen,
		parking.EndIsClimateOn,
		parking.EndIsUserPresent,
		parking.EndTpmsPressureFL,
		parking.EndTpmsPressureFR,
		parking.EndTpmsPressureRL,
		parking.EndTpmsPressureRR,
		parking.ID,
	)
	if err != nil {
		return fmt.Errorf("complete parking: %w", err)
	}
	return nil
}

// UpdateSnapshot 更新活跃停车记录的快照信息
func (r *ParkingRepository) UpdateSnapshot(ctx context.Context, parking *models.Parking) error {
	query := `
		UPDATE parkings SET
			end_battery_level = $2,
			end_range_km = $3,
			end_odometer = $4,
			end_inside_temp = $5,
			end_outside_temp = $6,
			end_sentry_mode = $7,
			end_locked = $8,
			end_doors_open = $9,
			end_windows_open = $10,
			end_frunk_open = $11,
			end_trunk_open = $12,
			end_is_climate_on = $13,
			climate_used_min = $14,
			sentry_mode_used_min = $15
		WHERE id = $1 AND end_time IS NULL
	`
	_, err := r.db.Pool.Exec(ctx, query,
		parking.ID,
		parking.EndBatteryLevel,
		parking.EndRangeKm,
		parking.EndOdometer,
		parking.EndInsideTemp,
		parking.EndOutsideTemp,
		parking.EndSentryMode,
		parking.EndLocked,
		parking.EndDoorsOpen,
		parking.EndWindowsOpen,
		parking.EndFrunkOpen,
		parking.EndTrunkOpen,
		parking.EndIsClimateOn,
		parking.ClimateUsedMin,
		parking.SentryModeUsedMin,
	)
	if err != nil {
		return fmt.Errorf("update parking snapshot: %w", err)
	}
	return nil
}

// GetByID 获取停车记录
func (r *ParkingRepository) GetByID(ctx context.Context, id int64) (*models.Parking, error) {
	query := `
		SELECT id, car_id, position_id, geofence_id, start_time, end_time, duration_min,
			latitude, longitude,
			start_battery_level, end_battery_level, start_range_km, end_range_km,
			start_odometer, end_odometer, energy_used_kwh,
			start_inside_temp, end_inside_temp, start_outside_temp, end_outside_temp,
			inside_temp_avg, outside_temp_avg,
			climate_used_min, sentry_mode_used_min,
			start_locked, start_sentry_mode, start_doors_open, start_windows_open,
			start_frunk_open, start_trunk_open, start_is_climate_on, start_is_user_present,
			end_locked, end_sentry_mode, end_doors_open, end_windows_open,
			end_frunk_open, end_trunk_open, end_is_climate_on, end_is_user_present,
			start_tpms_pressure_fl, start_tpms_pressure_fr, start_tpms_pressure_rl, start_tpms_pressure_rr,
			end_tpms_pressure_fl, end_tpms_pressure_fr, end_tpms_pressure_rl, end_tpms_pressure_rr,
			car_version, address
		FROM parkings WHERE id = $1
	`
	parking := &models.Parking{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&parking.ID,
		&parking.CarID,
		&parking.PositionID,
		&parking.GeofenceID,
		&parking.StartTime,
		&parking.EndTime,
		&parking.DurationMin,
		&parking.Latitude,
		&parking.Longitude,
		&parking.StartBatteryLevel,
		&parking.EndBatteryLevel,
		&parking.StartRangeKm,
		&parking.EndRangeKm,
		&parking.StartOdometer,
		&parking.EndOdometer,
		&parking.EnergyUsedKwh,
		&parking.StartInsideTemp,
		&parking.EndInsideTemp,
		&parking.StartOutsideTemp,
		&parking.EndOutsideTemp,
		&parking.InsideTempAvg,
		&parking.OutsideTempAvg,
		&parking.ClimateUsedMin,
		&parking.SentryModeUsedMin,
		&parking.StartLocked,
		&parking.StartSentryMode,
		&parking.StartDoorsOpen,
		&parking.StartWindowsOpen,
		&parking.StartFrunkOpen,
		&parking.StartTrunkOpen,
		&parking.StartIsClimateOn,
		&parking.StartIsUserPresent,
		&parking.EndLocked,
		&parking.EndSentryMode,
		&parking.EndDoorsOpen,
		&parking.EndWindowsOpen,
		&parking.EndFrunkOpen,
		&parking.EndTrunkOpen,
		&parking.EndIsClimateOn,
		&parking.EndIsUserPresent,
		&parking.StartTpmsPressureFL,
		&parking.StartTpmsPressureFR,
		&parking.StartTpmsPressureRL,
		&parking.StartTpmsPressureRR,
		&parking.EndTpmsPressureFL,
		&parking.EndTpmsPressureFR,
		&parking.EndTpmsPressureRL,
		&parking.EndTpmsPressureRR,
		&parking.CarVersion,
		&parking.Address,
	)
	if err != nil {
		return nil, fmt.Errorf("get parking by id: %w", err)
	}
	return parking, nil
}

// ListByCarID 获取车辆的停车列表
func (r *ParkingRepository) ListByCarID(ctx context.Context, carID int64, limit, offset int) ([]*models.Parking, error) {
	query := `
		SELECT id, car_id, position_id, geofence_id, start_time, end_time, duration_min,
			latitude, longitude,
			start_battery_level, end_battery_level, start_range_km, end_range_km,
			start_odometer, end_odometer, energy_used_kwh,
			start_inside_temp, end_inside_temp, start_outside_temp, end_outside_temp,
			inside_temp_avg, outside_temp_avg,
			climate_used_min, sentry_mode_used_min,
			start_locked, start_sentry_mode, start_doors_open, start_windows_open,
			start_frunk_open, start_trunk_open, start_is_climate_on, start_is_user_present,
			end_locked, end_sentry_mode, end_doors_open, end_windows_open,
			end_frunk_open, end_trunk_open, end_is_climate_on, end_is_user_present,
			start_tpms_pressure_fl, start_tpms_pressure_fr, start_tpms_pressure_rl, start_tpms_pressure_rr,
			end_tpms_pressure_fl, end_tpms_pressure_fr, end_tpms_pressure_rl, end_tpms_pressure_rr,
			car_version, address
		FROM parkings WHERE car_id = $1 ORDER BY start_time DESC LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Pool.Query(ctx, query, carID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list parkings: %w", err)
	}
	defer rows.Close()

	var parkings []*models.Parking
	for rows.Next() {
		parking := &models.Parking{}
		err := rows.Scan(
			&parking.ID,
			&parking.CarID,
			&parking.PositionID,
			&parking.GeofenceID,
			&parking.StartTime,
			&parking.EndTime,
			&parking.DurationMin,
			&parking.Latitude,
			&parking.Longitude,
			&parking.StartBatteryLevel,
			&parking.EndBatteryLevel,
			&parking.StartRangeKm,
			&parking.EndRangeKm,
			&parking.StartOdometer,
			&parking.EndOdometer,
			&parking.EnergyUsedKwh,
			&parking.StartInsideTemp,
			&parking.EndInsideTemp,
			&parking.StartOutsideTemp,
			&parking.EndOutsideTemp,
			&parking.InsideTempAvg,
			&parking.OutsideTempAvg,
			&parking.ClimateUsedMin,
			&parking.SentryModeUsedMin,
			&parking.StartLocked,
			&parking.StartSentryMode,
			&parking.StartDoorsOpen,
			&parking.StartWindowsOpen,
			&parking.StartFrunkOpen,
			&parking.StartTrunkOpen,
			&parking.StartIsClimateOn,
			&parking.StartIsUserPresent,
			&parking.EndLocked,
			&parking.EndSentryMode,
			&parking.EndDoorsOpen,
			&parking.EndWindowsOpen,
			&parking.EndFrunkOpen,
			&parking.EndTrunkOpen,
			&parking.EndIsClimateOn,
			&parking.EndIsUserPresent,
			&parking.StartTpmsPressureFL,
			&parking.StartTpmsPressureFR,
			&parking.StartTpmsPressureRL,
			&parking.StartTpmsPressureRR,
			&parking.EndTpmsPressureFL,
			&parking.EndTpmsPressureFR,
			&parking.EndTpmsPressureRL,
			&parking.EndTpmsPressureRR,
			&parking.CarVersion,
			&parking.Address,
		)
		if err != nil {
			return nil, fmt.Errorf("scan parking: %w", err)
		}
		parkings = append(parkings, parking)
	}

	return parkings, nil
}

// CountByCarID 统计车辆停车数
func (r *ParkingRepository) CountByCarID(ctx context.Context, carID int64) (int64, error) {
	var count int64
	err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM parkings WHERE car_id = $1`, carID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count parkings: %w", err)
	}
	return count, nil
}

// GetActiveParking 获取进行中的停车记录
func (r *ParkingRepository) GetActiveParking(ctx context.Context, carID int64) (*models.Parking, error) {
	query := `
		SELECT id, car_id, position_id, geofence_id, start_time, end_time, duration_min,
			latitude, longitude,
			start_battery_level, end_battery_level, start_range_km, end_range_km,
			start_odometer, end_odometer, energy_used_kwh,
			start_inside_temp, end_inside_temp, start_outside_temp, end_outside_temp,
			inside_temp_avg, outside_temp_avg,
			climate_used_min, sentry_mode_used_min,
			start_locked, start_sentry_mode, start_doors_open, start_windows_open,
			start_frunk_open, start_trunk_open, start_is_climate_on, start_is_user_present,
			end_locked, end_sentry_mode, end_doors_open, end_windows_open,
			end_frunk_open, end_trunk_open, end_is_climate_on, end_is_user_present,
			start_tpms_pressure_fl, start_tpms_pressure_fr, start_tpms_pressure_rl, start_tpms_pressure_rr,
			end_tpms_pressure_fl, end_tpms_pressure_fr, end_tpms_pressure_rl, end_tpms_pressure_rr,
			car_version, address
		FROM parkings WHERE car_id = $1 AND end_time IS NULL ORDER BY start_time DESC LIMIT 1
	`
	parking := &models.Parking{}
	err := r.db.Pool.QueryRow(ctx, query, carID).Scan(
		&parking.ID,
		&parking.CarID,
		&parking.PositionID,
		&parking.GeofenceID,
		&parking.StartTime,
		&parking.EndTime,
		&parking.DurationMin,
		&parking.Latitude,
		&parking.Longitude,
		&parking.StartBatteryLevel,
		&parking.EndBatteryLevel,
		&parking.StartRangeKm,
		&parking.EndRangeKm,
		&parking.StartOdometer,
		&parking.EndOdometer,
		&parking.EnergyUsedKwh,
		&parking.StartInsideTemp,
		&parking.EndInsideTemp,
		&parking.StartOutsideTemp,
		&parking.EndOutsideTemp,
		&parking.InsideTempAvg,
		&parking.OutsideTempAvg,
		&parking.ClimateUsedMin,
		&parking.SentryModeUsedMin,
		&parking.StartLocked,
		&parking.StartSentryMode,
		&parking.StartDoorsOpen,
		&parking.StartWindowsOpen,
		&parking.StartFrunkOpen,
		&parking.StartTrunkOpen,
		&parking.StartIsClimateOn,
		&parking.StartIsUserPresent,
		&parking.EndLocked,
		&parking.EndSentryMode,
		&parking.EndDoorsOpen,
		&parking.EndWindowsOpen,
		&parking.EndFrunkOpen,
		&parking.EndTrunkOpen,
		&parking.EndIsClimateOn,
		&parking.EndIsUserPresent,
		&parking.StartTpmsPressureFL,
		&parking.StartTpmsPressureFR,
		&parking.StartTpmsPressureRL,
		&parking.StartTpmsPressureRR,
		&parking.EndTpmsPressureFL,
		&parking.EndTpmsPressureFR,
		&parking.EndTpmsPressureRL,
		&parking.EndTpmsPressureRR,
		&parking.CarVersion,
		&parking.Address,
	)
	if err != nil {
		return nil, err // 可能是没有进行中的停车
	}
	return parking, nil
}

// ForceCloseOpenParkings 强制关闭指定车辆的所有未结束停车记录
func (r *ParkingRepository) ForceCloseOpenParkings(ctx context.Context, carID int64, endTime time.Time) error {
	query := `
		UPDATE parkings 
		SET end_time = $1, duration_min = EXTRACT(EPOCH FROM ($1 - start_time))/60
		WHERE car_id = $2 AND end_time IS NULL
	`
	_, err := r.db.Pool.Exec(ctx, query, endTime, carID)
	return err
}

// GetStats 获取停车统计
func (r *ParkingRepository) GetStats(ctx context.Context, carID int64, since time.Time) (totalDuration float64, totalEnergyUsed float64, count int64, err error) {
	query := `
		SELECT COALESCE(SUM(duration_min), 0), COALESCE(SUM(energy_used_kwh), 0), COUNT(*)
		FROM parkings WHERE car_id = $1 AND start_time >= $2 AND end_time IS NOT NULL
	`
	err = r.db.Pool.QueryRow(ctx, query, carID, since).Scan(&totalDuration, &totalEnergyUsed, &count)
	if err != nil {
		err = fmt.Errorf("get parking stats: %w", err)
	}
	return
}

// CreateEvent 创建停车事件
func (r *ParkingRepository) CreateEvent(ctx context.Context, event *models.ParkingEvent) error {
	query := `
		INSERT INTO parking_events (parking_id, event_type, event_time, details)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`
	err := r.db.Pool.QueryRow(ctx, query,
		event.ParkingID,
		event.EventType,
		event.EventTime,
		event.Details,
	).Scan(&event.ID)
	if err != nil {
		return fmt.Errorf("create parking event: %w", err)
	}
	return nil
}

// ListEventsByParkingID 获取停车事件列表
func (r *ParkingRepository) ListEventsByParkingID(ctx context.Context, parkingID int64) ([]*models.ParkingEvent, error) {
	query := `
		SELECT id, parking_id, event_type, event_time, details
		FROM parking_events
		WHERE parking_id = $1
		ORDER BY event_time ASC
	`
	rows, err := r.db.Pool.Query(ctx, query, parkingID)
	if err != nil {
		return nil, fmt.Errorf("list parking events: %w", err)
	}
	defer rows.Close()

	var events []*models.ParkingEvent
	for rows.Next() {
		event := &models.ParkingEvent{}
		err := rows.Scan(
			&event.ID,
			&event.ParkingID,
			&event.EventType,
			&event.EventTime,
			&event.Details,
		)
		if err != nil {
			return nil, fmt.Errorf("scan parking event: %w", err)
		}
		events = append(events, event)
	}

	return events, nil
}

// DeleteEventsByParkingID 删除停车事件（用于停车记录删除时级联删除）
func (r *ParkingRepository) DeleteEventsByParkingID(ctx context.Context, parkingID int64) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM parking_events WHERE parking_id = $1`, parkingID)
	if err != nil {
		return fmt.Errorf("delete parking events: %w", err)
	}
	return nil
}
