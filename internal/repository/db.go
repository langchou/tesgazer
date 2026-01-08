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
		migrationAddTpmsToPositions,
		migrationAddOdometerToDrives,
		migrationAddEnergyToDrives,
		migrationCreateParkings,
		migrationAddAddressToDrives,
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

// 添加 TPMS 胎压字段到 positions 表
const migrationAddTpmsToPositions = `
ALTER TABLE positions ADD COLUMN IF NOT EXISTS tpms_pressure_fl DOUBLE PRECISION;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS tpms_pressure_fr DOUBLE PRECISION;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS tpms_pressure_rl DOUBLE PRECISION;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS tpms_pressure_rr DOUBLE PRECISION;
`

// 添加里程表字段到 drives 表，并修复历史数据
const migrationAddOdometerToDrives = `
-- 添加里程表字段
ALTER TABLE drives ADD COLUMN IF NOT EXISTS start_odometer_km DOUBLE PRECISION DEFAULT 0;
ALTER TABLE drives ADD COLUMN IF NOT EXISTS end_odometer_km DOUBLE PRECISION;

-- 修复历史行程数据：关联位置记录到对应的行程
UPDATE positions p SET drive_id = d.id
FROM drives d
WHERE p.car_id = d.car_id
  AND p.drive_id IS NULL
  AND p.recorded_at >= d.start_time
  AND p.recorded_at <= COALESCE(d.end_time, NOW())
  AND d.end_time IS NOT NULL;

-- 修复历史行程数据：根据位置记录计算 distance_km
UPDATE drives d SET
  start_odometer_km = COALESCE((
    SELECT MIN(p.odometer) FROM positions p
    WHERE p.drive_id = d.id AND p.odometer > 0
  ), 0),
  end_odometer_km = (
    SELECT MAX(p.odometer) FROM positions p
    WHERE p.drive_id = d.id AND p.odometer > 0
  ),
  distance_km = COALESCE((
    SELECT MAX(p.odometer) - MIN(p.odometer) FROM positions p
    WHERE p.drive_id = d.id AND p.odometer > 0
  ), 0)
WHERE d.end_time IS NOT NULL AND d.distance_km = 0;
`

// 添加能量统计字段到 drives 表，并修复历史数据的统计值
const migrationAddEnergyToDrives = `
-- 添加能量统计字段
ALTER TABLE drives ADD COLUMN IF NOT EXISTS energy_used_kwh DOUBLE PRECISION;
ALTER TABLE drives ADD COLUMN IF NOT EXISTS energy_regen_kwh DOUBLE PRECISION;

-- 修复历史行程数据：计算 speed_max, power_max, power_min, 温度平均值
UPDATE drives d SET
  speed_max = (
    SELECT MAX(p.speed) FROM positions p
    WHERE p.drive_id = d.id AND p.speed IS NOT NULL
  ),
  power_max = (
    SELECT MAX(p.power) FROM positions p
    WHERE p.drive_id = d.id AND p.power IS NOT NULL
  ),
  power_min = (
    SELECT MIN(p.power) FROM positions p
    WHERE p.drive_id = d.id AND p.power IS NOT NULL
  ),
  inside_temp_avg = (
    SELECT AVG(p.inside_temp) FROM positions p
    WHERE p.drive_id = d.id AND p.inside_temp IS NOT NULL
  ),
  outside_temp_avg = (
    SELECT AVG(p.outside_temp) FROM positions p
    WHERE p.drive_id = d.id AND p.outside_temp IS NOT NULL
  )
WHERE d.end_time IS NOT NULL AND d.speed_max IS NULL;

-- 修复历史行程数据：计算能量消耗和回收
-- 注意：这需要更复杂的查询，基于时间间隔计算
UPDATE drives d SET
  energy_used_kwh = sub.energy_used,
  energy_regen_kwh = sub.energy_regen
FROM (
  SELECT
    drive_id,
    COALESCE(SUM(CASE WHEN power > 0 THEN power * interval_seconds / 3600.0 ELSE 0 END), 0) as energy_used,
    COALESCE(SUM(CASE WHEN power < 0 THEN ABS(power) * interval_seconds / 3600.0 ELSE 0 END), 0) as energy_regen
  FROM (
    SELECT
      drive_id,
      power,
      EXTRACT(EPOCH FROM (
        LEAD(recorded_at) OVER (PARTITION BY drive_id ORDER BY recorded_at) - recorded_at
      )) as interval_seconds
    FROM positions
    WHERE drive_id IS NOT NULL AND power IS NOT NULL
  ) intervals
  WHERE interval_seconds IS NOT NULL AND interval_seconds < 60
  GROUP BY drive_id
) sub
WHERE d.id = sub.drive_id AND d.energy_used_kwh IS NULL;
`

// 创建 parkings 停车记录表
const migrationCreateParkings = `
CREATE TABLE IF NOT EXISTS parkings (
    id BIGSERIAL PRIMARY KEY,
    car_id BIGINT NOT NULL REFERENCES cars(id),
    position_id BIGINT,
    geofence_id BIGINT,
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP WITH TIME ZONE,
    duration_min DOUBLE PRECISION DEFAULT 0,

    -- 位置
    latitude DOUBLE PRECISION NOT NULL,
    longitude DOUBLE PRECISION NOT NULL,

    -- 电量变化
    start_battery_level INT,
    end_battery_level INT,
    start_range_km DOUBLE PRECISION,
    end_range_km DOUBLE PRECISION,
    start_odometer DOUBLE PRECISION,
    end_odometer DOUBLE PRECISION,
    energy_used_kwh DOUBLE PRECISION,

    -- 温度
    start_inside_temp DOUBLE PRECISION,
    end_inside_temp DOUBLE PRECISION,
    start_outside_temp DOUBLE PRECISION,
    end_outside_temp DOUBLE PRECISION,
    inside_temp_avg DOUBLE PRECISION,
    outside_temp_avg DOUBLE PRECISION,

    -- 空调和哨兵模式使用时长
    climate_used_min DOUBLE PRECISION,
    sentry_mode_used_min DOUBLE PRECISION,

    -- 起始状态快照
    start_locked BOOLEAN DEFAULT false,
    start_sentry_mode BOOLEAN DEFAULT false,
    start_doors_open BOOLEAN DEFAULT false,
    start_windows_open BOOLEAN DEFAULT false,
    start_frunk_open BOOLEAN DEFAULT false,
    start_trunk_open BOOLEAN DEFAULT false,
    start_is_climate_on BOOLEAN DEFAULT false,
    start_is_user_present BOOLEAN DEFAULT false,

    -- 结束状态快照
    end_locked BOOLEAN,
    end_sentry_mode BOOLEAN,
    end_doors_open BOOLEAN,
    end_windows_open BOOLEAN,
    end_frunk_open BOOLEAN,
    end_trunk_open BOOLEAN,
    end_is_climate_on BOOLEAN,
    end_is_user_present BOOLEAN,

    -- 胎压 (开始)
    start_tpms_pressure_fl DOUBLE PRECISION,
    start_tpms_pressure_fr DOUBLE PRECISION,
    start_tpms_pressure_rl DOUBLE PRECISION,
    start_tpms_pressure_rr DOUBLE PRECISION,

    -- 胎压 (结束)
    end_tpms_pressure_fl DOUBLE PRECISION,
    end_tpms_pressure_fr DOUBLE PRECISION,
    end_tpms_pressure_rl DOUBLE PRECISION,
    end_tpms_pressure_rr DOUBLE PRECISION,

    -- 软件版本
    car_version VARCHAR(50)
);
CREATE INDEX IF NOT EXISTS idx_parkings_car_id ON parkings(car_id);
CREATE INDEX IF NOT EXISTS idx_parkings_start_time ON parkings(start_time);
`

// 添加地址字段到 drives 表（用于逆地理编码结果）
const migrationAddAddressToDrives = `
-- 添加起止经纬度字段（用于前端展示和逆地理编码）
ALTER TABLE drives ADD COLUMN IF NOT EXISTS start_latitude DOUBLE PRECISION;
ALTER TABLE drives ADD COLUMN IF NOT EXISTS start_longitude DOUBLE PRECISION;
ALTER TABLE drives ADD COLUMN IF NOT EXISTS end_latitude DOUBLE PRECISION;
ALTER TABLE drives ADD COLUMN IF NOT EXISTS end_longitude DOUBLE PRECISION;

-- 添加 JSONB 类型地址字段（结构化数据）
ALTER TABLE drives ADD COLUMN IF NOT EXISTS start_address JSONB;
ALTER TABLE drives ADD COLUMN IF NOT EXISTS end_address JSONB;
`
