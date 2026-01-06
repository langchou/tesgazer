package tesla

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Token 认证令牌
type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	CreatedAt    time.Time `json:"created_at"`
}

// IsExpired 检查 token 是否过期
func (t *Token) IsExpired() bool {
	return time.Now().After(t.CreatedAt.Add(time.Duration(t.ExpiresIn-300) * time.Second))
}

// Client Tesla API 客户端
type Client struct {
	httpClient  *http.Client
	authHost    string
	apiHost     string
	clientID    string
	redirectURI string
	token       *Token
}

// NewClient 创建新的 Tesla API 客户端
func NewClient(authHost, apiHost, clientID, redirectURI string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		authHost:    authHost,
		apiHost:     apiHost,
		clientID:    clientID,
		redirectURI: redirectURI,
	}
}

// SetToken 设置认证令牌
func (c *Client) SetToken(token *Token) {
	c.token = token
}

// GetToken 获取当前令牌
func (c *Client) GetToken() *Token {
	return c.token
}

// RefreshToken 刷新访问令牌
func (c *Client) RefreshToken(ctx context.Context) error {
	if c.token == nil || c.token.RefreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("client_id", c.clientID)
	data.Set("refresh_token", c.token.RefreshToken)
	data.Set("scope", "openid email offline_access")

	req, err := http.NewRequestWithContext(ctx, "POST", c.authHost+"/oauth2/v3/token", strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("create refresh request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("refresh token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("refresh token failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	var tokenResp Token
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("decode token response: %w", err)
	}

	tokenResp.CreatedAt = time.Now()
	c.token = &tokenResp

	return nil
}

// doRequest 执行带认证的请求
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	if c.token == nil {
		return nil, fmt.Errorf("not authenticated")
	}

	// 检查 token 是否过期
	if c.token.IsExpired() {
		if err := c.RefreshToken(ctx); err != nil {
			return nil, fmt.Errorf("refresh token: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, c.apiHost+path, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.token.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "TeslaMate-Go/1.0")

	return c.httpClient.Do(req)
}

// apiResponse 通用 API 响应结构
type apiResponse struct {
	Response json.RawMessage `json:"response"`
	Error    string          `json:"error,omitempty"`
}

// ListVehicles 获取车辆列表
func (c *Client) ListVehicles(ctx context.Context) ([]Vehicle, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/1/products", nil)
	if err != nil {
		return nil, fmt.Errorf("list vehicles request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list vehicles failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	// 重新创建 reader 用于解码
	resp.Body = io.NopCloser(strings.NewReader(string(body)))

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// 解析产品列表，过滤出车辆
	var products []map[string]interface{}
	if err := json.Unmarshal(apiResp.Response, &products); err != nil {
		return nil, fmt.Errorf("decode products: %w", err)
	}

	var vehicles []Vehicle
	for _, p := range products {
		// 只保留有 vehicle_id 的产品（即车辆）
		if _, ok := p["vehicle_id"]; ok {
			data, _ := json.Marshal(p)
			var v Vehicle
			if err := json.Unmarshal(data, &v); err == nil {
				vehicles = append(vehicles, v)
			}
		}
	}

	return vehicles, nil
}

// GetVehicle 获取单个车辆信息
func (c *Client) GetVehicle(ctx context.Context, id int64) (*Vehicle, error) {
	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/api/1/vehicles/%d", id), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get vehicle failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	var vehicle Vehicle
	if err := json.Unmarshal(apiResp.Response, &vehicle); err != nil {
		return nil, fmt.Errorf("decode vehicle: %w", err)
	}

	return &vehicle, nil
}

// GetVehicleData 获取车辆完整数据
func (c *Client) GetVehicleData(ctx context.Context, id int64) (*VehicleData, error) {
	endpoints := "charge_state;climate_state;closures_state;drive_state;gui_settings;location_data;vehicle_config;vehicle_state"
	path := fmt.Sprintf("/api/1/vehicles/%d/vehicle_data?endpoints=%s", id, url.QueryEscape(endpoints))

	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 处理不同状态码
	switch resp.StatusCode {
	case http.StatusOK:
		// 正常
	case http.StatusRequestTimeout:
		return nil, ErrVehicleUnavailable
	case http.StatusUnauthorized:
		return nil, ErrUnauthorized
	case http.StatusTooManyRequests:
		return nil, ErrRateLimited
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get vehicle data failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	var data VehicleData
	if err := json.Unmarshal(apiResp.Response, &data); err != nil {
		return nil, fmt.Errorf("decode vehicle data: %w", err)
	}

	return &data, nil
}

// WakeUp 唤醒车辆
func (c *Client) WakeUp(ctx context.Context, id int64) error {
	resp, err := c.doRequest(ctx, "POST", fmt.Sprintf("/api/1/vehicles/%d/wake_up", id), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("wake up failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	return nil
}

// 错误定义
var (
	ErrVehicleUnavailable = fmt.Errorf("vehicle unavailable")
	ErrUnauthorized       = fmt.Errorf("unauthorized")
	ErrRateLimited        = fmt.Errorf("rate limited")
)
