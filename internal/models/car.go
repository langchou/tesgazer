package models

import "time"

// Car 车辆信息
type Car struct {
	ID             int64     `json:"id" db:"id"`
	TeslaID        int64     `json:"tesla_id" db:"tesla_id"`
	TeslaVehicleID int64     `json:"tesla_vehicle_id" db:"tesla_vehicle_id"`
	VIN            string    `json:"vin" db:"vin"`
	Name           string    `json:"name" db:"name"`
	Model          string    `json:"model" db:"model"`
	TrimBadging    string    `json:"trim_badging" db:"trim_badging"`
	ExteriorColor  string    `json:"exterior_color" db:"exterior_color"`
	WheelType      string    `json:"wheel_type" db:"wheel_type"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// Settings 设置
type Settings struct {
	ID    int64  `json:"id" db:"id"`
	CarID int64  `json:"car_id" db:"car_id"`
	Key   string `json:"key" db:"key"`
	Value string `json:"value" db:"value"`
}

// Token 存储的认证令牌
type Token struct {
	ID           int64     `json:"id" db:"id"`
	AccessToken  string    `json:"access_token" db:"access_token"`
	RefreshToken string    `json:"refresh_token" db:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}
