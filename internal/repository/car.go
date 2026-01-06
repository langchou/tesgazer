package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/langchou/tesgazer/internal/models"
)

// CarRepository 车辆数据仓库
type CarRepository struct {
	db *DB
}

// NewCarRepository 创建车辆仓库
func NewCarRepository(db *DB) *CarRepository {
	return &CarRepository{db: db}
}

// Create 创建车辆
func (r *CarRepository) Create(ctx context.Context, car *models.Car) error {
	query := `
		INSERT INTO cars (tesla_id, tesla_vehicle_id, vin, name, model, trim_badging, exterior_color, wheel_type, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`
	now := time.Now()
	err := r.db.Pool.QueryRow(ctx, query,
		car.TeslaID,
		car.TeslaVehicleID,
		car.VIN,
		car.Name,
		car.Model,
		car.TrimBadging,
		car.ExteriorColor,
		car.WheelType,
		now,
		now,
	).Scan(&car.ID)

	if err != nil {
		return fmt.Errorf("insert car: %w", err)
	}

	car.CreatedAt = now
	car.UpdatedAt = now
	return nil
}

// GetByTeslaID 通过 Tesla ID 获取车辆
func (r *CarRepository) GetByTeslaID(ctx context.Context, teslaID int64) (*models.Car, error) {
	query := `
		SELECT id, tesla_id, tesla_vehicle_id, vin, name, model, trim_badging, exterior_color, wheel_type, created_at, updated_at
		FROM cars WHERE tesla_id = $1
	`
	car := &models.Car{}
	err := r.db.Pool.QueryRow(ctx, query, teslaID).Scan(
		&car.ID,
		&car.TeslaID,
		&car.TeslaVehicleID,
		&car.VIN,
		&car.Name,
		&car.Model,
		&car.TrimBadging,
		&car.ExteriorColor,
		&car.WheelType,
		&car.CreatedAt,
		&car.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get car by tesla_id: %w", err)
	}
	return car, nil
}

// GetByID 通过 ID 获取车辆
func (r *CarRepository) GetByID(ctx context.Context, id int64) (*models.Car, error) {
	query := `
		SELECT id, tesla_id, tesla_vehicle_id, vin, name, model, trim_badging, exterior_color, wheel_type, created_at, updated_at
		FROM cars WHERE id = $1
	`
	car := &models.Car{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&car.ID,
		&car.TeslaID,
		&car.TeslaVehicleID,
		&car.VIN,
		&car.Name,
		&car.Model,
		&car.TrimBadging,
		&car.ExteriorColor,
		&car.WheelType,
		&car.CreatedAt,
		&car.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get car by id: %w", err)
	}
	return car, nil
}

// List 获取所有车辆
func (r *CarRepository) List(ctx context.Context) ([]*models.Car, error) {
	query := `
		SELECT id, tesla_id, tesla_vehicle_id, vin, name, model, trim_badging, exterior_color, wheel_type, created_at, updated_at
		FROM cars ORDER BY id
	`
	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list cars: %w", err)
	}
	defer rows.Close()

	var cars []*models.Car
	for rows.Next() {
		car := &models.Car{}
		err := rows.Scan(
			&car.ID,
			&car.TeslaID,
			&car.TeslaVehicleID,
			&car.VIN,
			&car.Name,
			&car.Model,
			&car.TrimBadging,
			&car.ExteriorColor,
			&car.WheelType,
			&car.CreatedAt,
			&car.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan car: %w", err)
		}
		cars = append(cars, car)
	}

	return cars, nil
}

// Update 更新车辆
func (r *CarRepository) Update(ctx context.Context, car *models.Car) error {
	query := `
		UPDATE cars SET name = $1, model = $2, trim_badging = $3, exterior_color = $4, wheel_type = $5, updated_at = $6
		WHERE id = $7
	`
	car.UpdatedAt = time.Now()
	_, err := r.db.Pool.Exec(ctx, query,
		car.Name,
		car.Model,
		car.TrimBadging,
		car.ExteriorColor,
		car.WheelType,
		car.UpdatedAt,
		car.ID,
	)
	if err != nil {
		return fmt.Errorf("update car: %w", err)
	}
	return nil
}

// Upsert 创建或更新车辆
func (r *CarRepository) Upsert(ctx context.Context, car *models.Car) error {
	query := `
		INSERT INTO cars (tesla_id, tesla_vehicle_id, vin, name, model, trim_badging, exterior_color, wheel_type, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (tesla_id) DO UPDATE SET
			name = EXCLUDED.name,
			model = EXCLUDED.model,
			trim_badging = EXCLUDED.trim_badging,
			exterior_color = EXCLUDED.exterior_color,
			wheel_type = EXCLUDED.wheel_type,
			updated_at = EXCLUDED.updated_at
		RETURNING id, created_at
	`
	now := time.Now()
	err := r.db.Pool.QueryRow(ctx, query,
		car.TeslaID,
		car.TeslaVehicleID,
		car.VIN,
		car.Name,
		car.Model,
		car.TrimBadging,
		car.ExteriorColor,
		car.WheelType,
		now,
		now,
	).Scan(&car.ID, &car.CreatedAt)

	if err != nil {
		return fmt.Errorf("upsert car: %w", err)
	}

	car.UpdatedAt = now
	return nil
}
