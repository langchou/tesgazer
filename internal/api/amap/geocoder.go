package amap

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/langchou/tesgazer/internal/models"
	"go.uber.org/zap"
)

// GeocoderClient 高德逆地理编码客户端
type GeocoderClient struct {
	apiKey     string
	httpClient *http.Client
	logger     *zap.Logger

	// 缓存：避免重复请求相同坐标
	cache   map[string]*models.Address
	cacheMu sync.RWMutex
}

// RegeoResponse 高德逆地理编码响应
type RegeoResponse struct {
	Status    string     `json:"status"`    // "1" 成功, "0" 失败
	Info      string     `json:"info"`      // 状态信息
	InfoCode  string     `json:"infocode"`  // 状态码
	Regeocode *Regeocode `json:"regeocode"` // 逆地理编码结果
}

// Regeocode 逆地理编码结果
type Regeocode struct {
	FormattedAddress string           `json:"formatted_address"` // 格式化地址
	AddressComponent AddressComponent `json:"addressComponent"`  // 地址组成部分
}

// AddressComponent 地址组成部分
type AddressComponent struct {
	Country      string      `json:"country"`
	Province     string      `json:"province"`
	City         interface{} `json:"city"`         // 可能为空数组 []
	District     interface{} `json:"district"`     // 可能为空数组 []
	Township     interface{} `json:"township"`     // 可能为空数组 []
	Street       interface{} `json:"street"`       // 可能为空数组 []
	StreetNumber interface{} `json:"streetNumber"` // 可能为空数组 []
}

// interfaceToString 将 interface{} 转换为字符串
// 高德 API 返回的字段可能是字符串或空数组 []
func interfaceToString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	default:
		return ""
	}
}

// NewGeocoderClient 创建高德逆地理编码客户端
func NewGeocoderClient(apiKey string, logger *zap.Logger) *GeocoderClient {
	return &GeocoderClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
		cache:  make(map[string]*models.Address),
	}
}

// ReverseGeocode 逆地理编码：根据经纬度获取结构化地址
// 参数: lat 纬度, lng 经度
// 返回: 结构化地址对象
func (c *GeocoderClient) ReverseGeocode(ctx context.Context, lat, lng float64) (*models.Address, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("amap api key not configured")
	}

	// 生成缓存 key（精确到小数点后4位，约11米精度）
	cacheKey := fmt.Sprintf("%.4f,%.4f", lng, lat)

	// 检查缓存
	c.cacheMu.RLock()
	if addr, ok := c.cache[cacheKey]; ok {
		c.cacheMu.RUnlock()
		return addr, nil
	}
	c.cacheMu.RUnlock()

	// 高德 API 要求经度在前，纬度在后
	location := fmt.Sprintf("%.6f,%.6f", lng, lat)

	// 构建请求 URL
	apiURL := fmt.Sprintf(
		"https://restapi.amap.com/v3/geocode/regeo?key=%s&location=%s&extensions=base&output=JSON",
		url.QueryEscape(c.apiKey),
		url.QueryEscape(location),
	)

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("amap api returned status %d", resp.StatusCode)
	}

	// 解析响应
	var result RegeoResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// 检查返回状态
	if result.Status != "1" {
		c.logger.Warn("Amap geocode failed",
			zap.String("info", result.Info),
			zap.String("infocode", result.InfoCode),
			zap.Float64("lat", lat),
			zap.Float64("lng", lng))
		return nil, fmt.Errorf("amap api error: %s (code: %s)", result.Info, result.InfoCode)
	}

	if result.Regeocode == nil {
		return nil, fmt.Errorf("no regeocode result")
	}

	// 构建结构化地址
	comp := result.Regeocode.AddressComponent
	address := &models.Address{
		FormattedAddress: result.Regeocode.FormattedAddress,
		Country:          comp.Country,
		Province:         comp.Province,
		City:             interfaceToString(comp.City),
		District:         interfaceToString(comp.District),
		Township:         interfaceToString(comp.Township),
		Street:           interfaceToString(comp.Street),
		StreetNumber:     interfaceToString(comp.StreetNumber),
	}

	// 存入缓存
	c.cacheMu.Lock()
	c.cache[cacheKey] = address
	// 限制缓存大小（简单策略：超过 10000 条清空）
	if len(c.cache) > 10000 {
		c.cache = make(map[string]*models.Address)
		c.cache[cacheKey] = address
	}
	c.cacheMu.Unlock()

	c.logger.Debug("Geocoded address",
		zap.Float64("lat", lat),
		zap.Float64("lng", lng),
		zap.String("formatted", address.FormattedAddress),
		zap.String("province", address.Province),
		zap.String("city", address.City),
		zap.String("district", address.District))

	return address, nil
}

// IsConfigured 检查是否已配置 API Key
func (c *GeocoderClient) IsConfigured() bool {
	return c.apiKey != ""
}

// ClearCache 清空缓存
func (c *GeocoderClient) ClearCache() {
	c.cacheMu.Lock()
	c.cache = make(map[string]*models.Address)
	c.cacheMu.Unlock()
}

// CacheSize 获取缓存大小
func (c *GeocoderClient) CacheSize() int {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()
	return len(c.cache)
}
