package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB 数据库连接池封装
type DB struct {
	Pool *pgxpool.Pool
}

// New 创建数据库连接
func New(ctx context.Context, databaseURL string) (*DB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	// 连接池配置
	config.MaxConns = 10
	config.MinConns = 2

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	// 测试连接
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &DB{Pool: pool}, nil
}

// Close 关闭连接池
func (db *DB) Close() {
	db.Pool.Close()
}

// Migrate 执行数据库迁移
func (db *DB) Migrate(ctx context.Context) error {
	migrations := []string{
		migrationCreateCars,
		migrationCreatePositions,
		migrationCreateDrives,
		migrationCreateChargingProcesses,
		migrationCreateCharges,
		migrationCreateStates,
		migrationCreateGeofences,
		migrationCreateTokens,
	}

	for _, m := range migrations {
		if _, err := db.Pool.Exec(ctx, m); err != nil {
			return fmt.Errorf("execute migration: %w", err)
		}
	}

	return nil
}

// 数据库迁移 SQL
const migrationCreateCars = `
CREATE TABLE IF NOT EXISTS cars (
    id BIGSERIAL PRIMARY KEY,
    tesla_id BIGINT NOT NULL UNIQUE,
    tesla_vehicle_id BIGINT NOT NULL,
    vin VARCHAR(17) NOT NULL UNIQUE,
    name VARCHAR(255),
    model VARCHAR(50),
    trim_badging VARCHAR(50),
    exterior_color VARCHAR(50),
    wheel_type VARCHAR(50),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_cars_tesla_id ON cars(tesla_id);
`

const migrationCreatePositions = `
CREATE TABLE IF NOT EXISTS positions (
    id BIGSERIAL PRIMARY KEY,
    car_id BIGINT NOT NULL REFERENCES cars(id),
    drive_id BIGINT,
    latitude DOUBLE PRECISION NOT NULL,
    longitude DOUBLE PRECISION NOT NULL,
    heading INT,
    speed INT,
    power INT,
    odometer DOUBLE PRECISION,
    battery_level INT,
    range_km DOUBLE PRECISION,
    inside_temp DOUBLE PRECISION,
    outside_temp DOUBLE PRECISION,
    elevation INT,
    recorded_at TIMESTAMP WITH TIME ZONE NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_positions_car_id ON positions(car_id);
CREATE INDEX IF NOT EXISTS idx_positions_drive_id ON positions(drive_id);
CREATE INDEX IF NOT EXISTS idx_positions_recorded_at ON positions(recorded_at);
`

const migrationCreateDrives = `
CREATE TABLE IF NOT EXISTS drives (
    id BIGSERIAL PRIMARY KEY,
    car_id BIGINT NOT NULL REFERENCES cars(id),
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP WITH TIME ZONE,
    start_position_id BIGINT,
    end_position_id BIGINT,
    start_geofence_id BIGINT,
    end_geofence_id BIGINT,
    distance_km DOUBLE PRECISION DEFAULT 0,
    duration_min DOUBLE PRECISION DEFAULT 0,
    start_battery_level INT,
    end_battery_level INT,
    start_range_km DOUBLE PRECISION,
    end_range_km DOUBLE PRECISION,
    speed_max INT,
    power_max INT,
    power_min INT,
    inside_temp_avg DOUBLE PRECISION,
    outside_temp_avg DOUBLE PRECISION
);
CREATE INDEX IF NOT EXISTS idx_drives_car_id ON drives(car_id);
CREATE INDEX IF NOT EXISTS idx_drives_start_time ON drives(start_time);
`

const migrationCreateChargingProcesses = `
CREATE TABLE IF NOT EXISTS charging_processes (
    id BIGSERIAL PRIMARY KEY,
    car_id BIGINT NOT NULL REFERENCES cars(id),
    position_id BIGINT,
    geofence_id BIGINT,
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP WITH TIME ZONE,
    start_battery_level INT,
    end_battery_level INT,
    start_range_km DOUBLE PRECISION,
    end_range_km DOUBLE PRECISION,
    charge_energy_added DOUBLE PRECISION DEFAULT 0,
    charger_power_max INT,
    duration_min DOUBLE PRECISION DEFAULT 0,
    outside_temp_avg DOUBLE PRECISION,
    cost DOUBLE PRECISION
);
CREATE INDEX IF NOT EXISTS idx_charging_processes_car_id ON charging_processes(car_id);
CREATE INDEX IF NOT EXISTS idx_charging_processes_start_time ON charging_processes(start_time);
`

const migrationCreateCharges = `
CREATE TABLE IF NOT EXISTS charges (
    id BIGSERIAL PRIMARY KEY,
    charging_process_id BIGINT NOT NULL REFERENCES charging_processes(id),
    battery_level INT,
    usable_battery_level INT,
    range_km DOUBLE PRECISION,
    charger_power INT,
    charger_voltage INT,
    charger_current INT,
    charge_energy_added DOUBLE PRECISION,
    outside_temp DOUBLE PRECISION,
    recorded_at TIMESTAMP WITH TIME ZONE NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_charges_charging_process_id ON charges(charging_process_id);
`

const migrationCreateStates = `
CREATE TABLE IF NOT EXISTS states (
    id BIGSERIAL PRIMARY KEY,
    car_id BIGINT NOT NULL REFERENCES cars(id),
    state VARCHAR(20) NOT NULL,
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP WITH TIME ZONE
);
CREATE INDEX IF NOT EXISTS idx_states_car_id ON states(car_id);
CREATE INDEX IF NOT EXISTS idx_states_start_time ON states(start_time);
`

const migrationCreateGeofences = `
CREATE TABLE IF NOT EXISTS geofences (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    latitude DOUBLE PRECISION NOT NULL,
    longitude DOUBLE PRECISION NOT NULL,
    radius INT NOT NULL DEFAULT 50
);
`

const migrationCreateTokens = `
CREATE TABLE IF NOT EXISTS tokens (
    id BIGSERIAL PRIMARY KEY,
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
`
