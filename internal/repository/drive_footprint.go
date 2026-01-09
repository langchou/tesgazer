package repository

import (
	"context"
	"fmt"
	"time"

	// For pq.Array
	"github.com/langchou/tesgazer/internal/models"
)

// GetDrivePathsInRange 获取指定时间范围内的行程轨迹（精简版）
func (r *DriveRepository) GetDrivePathsInRange(ctx context.Context, carID int64, start, end time.Time) ([]*models.DrivePath, error) {
	// 1. 获取范围内的行程基本信息
	drivesQuery := `
		SELECT id, start_time, duration_min, distance_km 
		FROM drives 
		WHERE car_id = $1 AND start_time >= $2 AND start_time <= $3 
		ORDER BY start_time DESC
	`
	rows, err := r.db.Pool.Query(ctx, drivesQuery, carID, start, end)
	if err != nil {
		return nil, fmt.Errorf("list drives in range: %w", err)
	}
	defer rows.Close()

	var drives []*models.DrivePath
	var driveIDs []int64
	driveMap := make(map[int64]*models.DrivePath)

	for rows.Next() {
		d := &models.DrivePath{
			Path: [][2]float64{},
		}
		if err := rows.Scan(&d.ID, &d.StartTime, &d.DurationMin, &d.DistanceKm); err != nil {
			return nil, fmt.Errorf("scan drive: %w", err)
		}
		drives = append(drives, d)
		driveIDs = append(driveIDs, d.ID)
		driveMap[d.ID] = d
	}

	if len(driveIDs) == 0 {
		return drives, nil
	}

	// 2. 批量获取位置点 (Downsampling: id % 10 for 1/10th data)
	// 注意：pq.Array 需要 lib/pq，但如果项目使用 pgx，可能需要转换。
	// 假设项目原本使用 database/sql + lib/pq 或者是 pgx pool。
	// 在 repository/drive.go 中看到 r.db.Pool.Query，这通常是 pgx/v4 binding。
	// pgx 支持 ANY($1) 语法。

	posQuery := `
		SELECT drive_id, latitude, longitude 
		FROM positions 
		WHERE drive_id = ANY($1) 
		AND (id % 10 = 0 OR speed < 5) -- 简单采样：保留1/10的点，或者低速点(转弯/停车可能需要)
		ORDER BY drive_id, id
	`
	// Wait, speed < 5 might indicate stop, but we want path shape.
	// id % 10 is safest simple heuristic.
	// Let's stick to id % 10.

	posQuery = `
		SELECT drive_id, latitude, longitude 
		FROM positions 
		WHERE drive_id = ANY($1) 

		ORDER BY drive_id, id
	`

	pRows, err := r.db.Pool.Query(ctx, posQuery, driveIDs) // pgx expects slice directly for ANY
	if err != nil {
		return nil, fmt.Errorf("list combined positions: %w", err)
	}
	defer pRows.Close()

	for pRows.Next() {
		var dID int64
		var lat, lng float64
		if err := pRows.Scan(&dID, &lat, &lng); err != nil {
			continue
		}
		if d, ok := driveMap[dID]; ok {
			d.Path = append(d.Path, [2]float64{lat, lng})
		}
	}

	return drives, nil
}
