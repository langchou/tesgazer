package geocoder

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

// Client 逆地理编码客户端
// 支持高德地图 API 和 Nominatim（OpenStreetMap）
// 如果配置了高德 API Key，优先使用高德；否则使用 Nominatim
type Client struct {
	amapAPIKey string
	httpClient *http.Client
	logger     *zap.Logger

	// 缓存：避免重复请求相同坐标
	cache   map[string]*models.Address
	cacheMu sync.RWMutex

	// Nominatim 请求限流（每秒最多 1 次）
	lastNominatimRequest time.Time
	nominatimMu          sync.Mutex
}

// NewClient 创建逆地理编码客户端
func NewClient(amapAPIKey string, logger *zap.Logger) *Client {
	return &Client{
		amapAPIKey: amapAPIKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
		cache:  make(map[string]*models.Address),
	}
}

// ReverseGeocode 逆地理编码：根据经纬度获取结构化地址
func (c *Client) ReverseGeocode(ctx context.Context, lat, lng float64) (*models.Address, error) {
	// 生成缓存 key（精确到小数点后4位，约11米精度）
	cacheKey := fmt.Sprintf("%.4f,%.4f", lat, lng)

	// 检查缓存
	c.cacheMu.RLock()
	if addr, ok := c.cache[cacheKey]; ok {
		c.cacheMu.RUnlock()
		return addr, nil
	}
	c.cacheMu.RUnlock()

	var address *models.Address
	var err error

	// 优先使用高德，没有配置则使用 Nominatim
	if c.amapAPIKey != "" {
		address, err = c.reverseGeocodeAmap(ctx, lat, lng)
	} else {
		address, err = c.reverseGeocodeNominatim(ctx, lat, lng)
	}

	if err != nil {
		return nil, err
	}

	// 存入缓存
	c.cacheMu.Lock()
	c.cache[cacheKey] = address
	// 限制缓存大小
	if len(c.cache) > 10000 {
		c.cache = make(map[string]*models.Address)
		c.cache[cacheKey] = address
	}
	c.cacheMu.Unlock()

	return address, nil
}

// IsConfigured 总是返回 true，因为有 Nominatim 作为默认选项
func (c *Client) IsConfigured() bool {
	return true
}

// GetProvider 返回当前使用的服务提供商
func (c *Client) GetProvider() string {
	if c.amapAPIKey != "" {
		return "amap"
	}
	return "nominatim"
}

// ============ 高德地图实现 ============

// AmapRegeoResponse 高德逆地理编码响应
type AmapRegeoResponse struct {
	Status    string        `json:"status"`
	Info      string        `json:"info"`
	InfoCode  string        `json:"infocode"`
	Regeocode *AmapRegeocode `json:"regeocode"`
}

type AmapRegeocode struct {
	FormattedAddress string               `json:"formatted_address"`
	AddressComponent AmapAddressComponent `json:"addressComponent"`
}

type AmapAddressComponent struct {
	Country      string      `json:"country"`
	Province     string      `json:"province"`
	City         interface{} `json:"city"`
	District     interface{} `json:"district"`
	Township     interface{} `json:"township"`
	Street       interface{} `json:"street"`
	StreetNumber interface{} `json:"streetNumber"`
}

func (c *Client) reverseGeocodeAmap(ctx context.Context, lat, lng float64) (*models.Address, error) {
	// 高德 API 要求经度在前，纬度在后
	location := fmt.Sprintf("%.6f,%.6f", lng, lat)

	apiURL := fmt.Sprintf(
		"https://restapi.amap.com/v3/geocode/regeo?key=%s&location=%s&extensions=base&output=JSON",
		url.QueryEscape(c.amapAPIKey),
		url.QueryEscape(location),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("amap api returned status %d", resp.StatusCode)
	}

	var result AmapRegeoResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if result.Status != "1" {
		return nil, fmt.Errorf("amap api error: %s (code: %s)", result.Info, result.InfoCode)
	}

	if result.Regeocode == nil {
		return nil, fmt.Errorf("no regeocode result")
	}

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

	c.logger.Debug("Geocoded via Amap",
		zap.Float64("lat", lat),
		zap.Float64("lng", lng),
		zap.String("address", address.FormattedAddress))

	return address, nil
}

// ============ Nominatim (OpenStreetMap) 实现 ============

// NominatimResponse Nominatim 逆地理编码响应
type NominatimResponse struct {
	DisplayName string           `json:"display_name"`
	Address     NominatimAddress `json:"address"`
}

type NominatimAddress struct {
	Road        string `json:"road"`
	Suburb      string `json:"suburb"`
	City        string `json:"city"`
	Town        string `json:"town"`
	Village     string `json:"village"`
	County      string `json:"county"`
	State       string `json:"state"`
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
	Postcode    string `json:"postcode"`
}

func (c *Client) reverseGeocodeNominatim(ctx context.Context, lat, lng float64) (*models.Address, error) {
	// Nominatim 限流：每秒最多 1 次请求
	c.nominatimMu.Lock()
	elapsed := time.Since(c.lastNominatimRequest)
	if elapsed < time.Second {
		time.Sleep(time.Second - elapsed)
	}
	c.lastNominatimRequest = time.Now()
	c.nominatimMu.Unlock()

	apiURL := fmt.Sprintf(
		"https://nominatim.openstreetmap.org/reverse?lat=%.6f&lon=%.6f&format=json&accept-language=zh-CN",
		lat, lng,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Nominatim 要求设置 User-Agent
	req.Header.Set("User-Agent", "Tesgazer/1.0 (Tesla vehicle logger)")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nominatim api returned status %d", resp.StatusCode)
	}

	var result NominatimResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// 构建地址：Nominatim 的城市字段可能在 city/town/village 中
	city := result.Address.City
	if city == "" {
		city = result.Address.Town
	}
	if city == "" {
		city = result.Address.Village
	}

	address := &models.Address{
		FormattedAddress: result.DisplayName,
		Country:          result.Address.Country,
		Province:         result.Address.State,
		City:             city,
		District:         result.Address.County,
		Township:         result.Address.Suburb,
		Street:           result.Address.Road,
		StreetNumber:     "",
	}

	c.logger.Debug("Geocoded via Nominatim",
		zap.Float64("lat", lat),
		zap.Float64("lng", lng),
		zap.String("address", address.FormattedAddress))

	return address, nil
}

// ============ 工具函数 ============

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

// ClearCache 清空缓存
func (c *Client) ClearCache() {
	c.cacheMu.Lock()
	c.cache = make(map[string]*models.Address)
	c.cacheMu.Unlock()
}

// CacheSize 获取缓存大小
func (c *Client) CacheSize() int {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()
	return len(c.cache)
}
