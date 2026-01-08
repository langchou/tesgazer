package models

import (
	"database/sql/driver"
	"encoding/json"
)

// Address 结构化地址信息（用于逆地理编码结果）
type Address struct {
	FormattedAddress string `json:"formatted_address,omitempty"` // 完整格式化地址
	Country          string `json:"country,omitempty"`           // 国家
	Province         string `json:"province,omitempty"`          // 省
	City             string `json:"city,omitempty"`              // 市
	District         string `json:"district,omitempty"`          // 区/县
	Township         string `json:"township,omitempty"`          // 乡镇/街道
	Street           string `json:"street,omitempty"`            // 道路
	StreetNumber     string `json:"street_number,omitempty"`     // 门牌号
}

// Value 实现 driver.Valuer 接口，用于存储到数据库
func (a Address) Value() (driver.Value, error) {
	return json.Marshal(a)
}

// Scan 实现 sql.Scanner 接口，用于从数据库读取
func (a *Address) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, a)
}

// Geofence 地理围栏
type Geofence struct {
	ID        int64   `json:"id" db:"id"`
	Name      string  `json:"name" db:"name"`
	Latitude  float64 `json:"latitude" db:"latitude"`
	Longitude float64 `json:"longitude" db:"longitude"`
	Radius    int     `json:"radius" db:"radius"` // 米
}
