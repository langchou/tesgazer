package tesla

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// StreamingHost Tesla Streaming API 地址
const StreamingHost = "wss://streaming.vn.teslamotors.com/streaming/"

// StreamData Tesla Streaming API 推送的数据
// 参考: https://tesla-api.timdorr.com/vehicle/streaming
type StreamData struct {
	MsgType    string  `json:"msg_type"`              // 消息类型: data:subscribe, data:update, data:error
	Tag        string  `json:"tag,omitempty"`         // vehicle_id
	Value      string  `json:"value,omitempty"`       // 逗号分隔的值
	ErrorType  string  `json:"error_type,omitempty"`  // 错误类型
	ConnectionTimeout int `json:"connection_timeout,omitempty"` // 超时时间

	// 解析后的字段
	Timestamp  int64   `json:"-"` // 时间戳 (毫秒)
	Speed      int     `json:"-"` // 速度 (mph)
	Odometer   float64 `json:"-"` // 里程 (miles)
	SOC        int     `json:"-"` // 电量百分比
	Elevation  int     `json:"-"` // 海拔 (m)
	EstHeading int     `json:"-"` // 航向角
	EstLat     float64 `json:"-"` // 纬度
	EstLng     float64 `json:"-"` // 经度
	Power      int     `json:"-"` // 功率 (kW)，负值=充电，正值=耗电
	ShiftState string  `json:"-"` // 挡位: D, N, R, P, ""
	Range      int     `json:"-"` // 续航 (miles)
	EstRange   int     `json:"-"` // 估计续航 (miles)
	Heading    int     `json:"-"` // 航向角
}

// StreamingCallbacks 流数据回调函数
type StreamingCallbacks struct {
	OnData          func(vehicleID int64, data *StreamData) // 收到数据
	OnConnect       func(vehicleID int64)                   // 连接成功
	OnDisconnect    func(vehicleID int64, err error)        // 断开连接
	OnVehicleOffline func(vehicleID int64)                  // 车辆离线，停止重连
}

// StreamingClient Tesla Streaming WebSocket 客户端
type StreamingClient struct {
	logger       *zap.Logger
	vehicleID    int64
	accessToken  string
	host         string
	conn         *websocket.Conn
	callbacks    StreamingCallbacks

	mu              sync.RWMutex
	connected       bool
	vehicleOffline  bool // 车辆离线标记，停止自动重连
	stopCh          chan struct{}
	reconnectCh     chan struct{}

	// 重连配置
	reconnectDelay    time.Duration
	maxReconnectDelay time.Duration
	currentDelay      time.Duration
}

// NewStreamingClient 创建 Streaming 客户端
func NewStreamingClient(logger *zap.Logger, vehicleID int64, accessToken string) *StreamingClient {
	return &StreamingClient{
		logger:            logger,
		vehicleID:         vehicleID,
		accessToken:       accessToken,
		host:              StreamingHost,
		stopCh:            make(chan struct{}),
		reconnectCh:       make(chan struct{}, 1),
		reconnectDelay:    1 * time.Second,
		maxReconnectDelay: 30 * time.Second,
		currentDelay:      1 * time.Second,
	}
}

// SetCallbacks 设置回调函数
func (c *StreamingClient) SetCallbacks(callbacks StreamingCallbacks) {
	c.callbacks = callbacks
}

// SetHost 设置自定义 host (用于测试)
func (c *StreamingClient) SetHost(host string) {
	c.host = host
}

// Connect 连接到 Streaming API
func (c *StreamingClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.connected {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	// 建立 WebSocket 连接
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, c.host, nil)
	if err != nil {
		return fmt.Errorf("dial streaming: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.connected = true
	c.currentDelay = c.reconnectDelay // 重置重连延迟
	c.mu.Unlock()

	// 发送订阅消息
	if err := c.subscribe(); err != nil {
		c.Close()
		return fmt.Errorf("subscribe: %w", err)
	}

	c.logger.Info("Streaming connected",
		zap.Int64("vehicle_id", c.vehicleID))

	// 通知连接成功
	if c.callbacks.OnConnect != nil {
		c.callbacks.OnConnect(c.vehicleID)
	}

	// 启动读取循环
	go c.readLoop()

	return nil
}

// Close 关闭连接
func (c *StreamingClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	c.connected = false

	// 关闭 stop channel
	select {
	case <-c.stopCh:
		// 已经关闭
	default:
		close(c.stopCh)
	}

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// IsConnected 检查连接状态
func (c *StreamingClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// subscribe 发送订阅消息
func (c *StreamingClient) subscribe() error {
	// Tesla Streaming API 订阅格式
	// 字段顺序: speed,odometer,soc,elevation,est_heading,est_lat,est_lng,power,shift_state,range,est_range,heading
	subscribeMsg := map[string]interface{}{
		"msg_type":  "data:subscribe_oauth",
		"token":     c.accessToken,
		"value":     "speed,odometer,soc,elevation,est_heading,est_lat,est_lng,power,shift_state,range,est_range,heading",
		"tag":       strconv.FormatInt(c.vehicleID, 10),
	}

	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	return conn.WriteJSON(subscribeMsg)
}

// readLoop 消息读取循环
func (c *StreamingClient) readLoop() {
	defer func() {
		c.mu.Lock()
		wasConnected := c.connected
		c.connected = false
		c.mu.Unlock()

		if wasConnected {
			// 通知断开
			if c.callbacks.OnDisconnect != nil {
				c.callbacks.OnDisconnect(c.vehicleID, nil)
			}
			// 尝试重连
			c.triggerReconnect()
		}
	}()

	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()

		if conn == nil {
			return
		}

		// 设置读取超时
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				c.logger.Debug("Streaming connection closed normally",
					zap.Int64("vehicle_id", c.vehicleID))
			} else {
				c.logger.Warn("Streaming read error",
					zap.Int64("vehicle_id", c.vehicleID),
					zap.Error(err))
			}
			return
		}

		// 解析消息
		var data StreamData
		if err := json.Unmarshal(message, &data); err != nil {
			c.logger.Warn("Failed to parse streaming message",
				zap.Int64("vehicle_id", c.vehicleID),
				zap.String("message", string(message)),
				zap.Error(err))
			continue
		}

		c.handleMessage(&data)
	}
}

// handleMessage 处理消息
func (c *StreamingClient) handleMessage(data *StreamData) {
	switch data.MsgType {
	case "data:update":
		// 解析逗号分隔的值
		c.parseDataValue(data)

		c.logger.Debug("Streaming data received",
			zap.Int64("vehicle_id", c.vehicleID),
			zap.String("shift_state", data.ShiftState),
			zap.Int("power", data.Power),
			zap.Int("speed", data.Speed),
			zap.Int("soc", data.SOC))

		// 触发回调
		if c.callbacks.OnData != nil {
			c.callbacks.OnData(c.vehicleID, data)
		}

	case "data:error":
		c.logger.Warn("Streaming error",
			zap.Int64("vehicle_id", c.vehicleID),
			zap.String("error_type", data.ErrorType),
			zap.String("value", data.Value))

		// 车辆离线错误：停止重连，等待 RESTful API 检测到车辆上线后再启动
		if data.ErrorType == "vehicle_error" && strings.Contains(data.Value, "offline") {
			c.mu.Lock()
			c.vehicleOffline = true
			c.mu.Unlock()

			c.logger.Info("Vehicle is offline, stopping streaming reconnect",
				zap.Int64("vehicle_id", c.vehicleID))

			// 通知上层车辆离线
			if c.callbacks.OnVehicleOffline != nil {
				c.callbacks.OnVehicleOffline(c.vehicleID)
			}
			return // 不触发重连，直接退出
		}

		// 其他错误触发重连
		if data.ErrorType == "vehicle_disconnected" || data.ErrorType == "vehicle_error" {
			c.triggerReconnect()
		}

	case "control:hello":
		c.logger.Debug("Streaming hello received",
			zap.Int64("vehicle_id", c.vehicleID),
			zap.Int("timeout", data.ConnectionTimeout))

	default:
		c.logger.Debug("Unknown streaming message type",
			zap.Int64("vehicle_id", c.vehicleID),
			zap.String("msg_type", data.MsgType))
	}
}

// parseDataValue 解析逗号分隔的值
// 字段顺序: timestamp,speed,odometer,soc,elevation,est_heading,est_lat,est_lng,power,shift_state,range,est_range,heading
func (c *StreamingClient) parseDataValue(data *StreamData) {
	if data.Value == "" {
		return
	}

	parts := strings.Split(data.Value, ",")
	if len(parts) < 13 {
		c.logger.Warn("Incomplete streaming data",
			zap.Int64("vehicle_id", c.vehicleID),
			zap.Int("parts_count", len(parts)))
		return
	}

	// 解析各字段（忽略错误，使用默认值）
	data.Timestamp, _ = strconv.ParseInt(parts[0], 10, 64)
	data.Speed, _ = strconv.Atoi(parts[1])
	data.Odometer, _ = strconv.ParseFloat(parts[2], 64)
	data.SOC, _ = strconv.Atoi(parts[3])
	data.Elevation, _ = strconv.Atoi(parts[4])
	data.EstHeading, _ = strconv.Atoi(parts[5])
	data.EstLat, _ = strconv.ParseFloat(parts[6], 64)
	data.EstLng, _ = strconv.ParseFloat(parts[7], 64)
	data.Power, _ = strconv.Atoi(parts[8])
	data.ShiftState = parts[9]
	data.Range, _ = strconv.Atoi(parts[10])
	data.EstRange, _ = strconv.Atoi(parts[11])
	data.Heading, _ = strconv.Atoi(parts[12])
}

// triggerReconnect 触发重连
func (c *StreamingClient) triggerReconnect() {
	select {
	case c.reconnectCh <- struct{}{}:
	default:
		// 已有重连请求排队
	}
}

// StartWithReconnect 启动并自动重连
func (c *StreamingClient) StartWithReconnect(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				c.Close()
				return
			case <-c.stopCh:
				return
			default:
			}

			// 检查车辆是否离线，如果离线则停止重连循环
			c.mu.RLock()
			offline := c.vehicleOffline
			c.mu.RUnlock()
			if offline {
				c.logger.Debug("Vehicle offline, streaming reconnect loop stopped",
					zap.Int64("vehicle_id", c.vehicleID))
				return
			}

			// 尝试连接
			if err := c.Connect(ctx); err != nil {
				c.logger.Warn("Streaming connect failed, will retry",
					zap.Int64("vehicle_id", c.vehicleID),
					zap.Duration("delay", c.currentDelay),
					zap.Error(err))

				// 等待重连延迟
				select {
				case <-ctx.Done():
					return
				case <-c.stopCh:
					return
				case <-time.After(c.currentDelay):
				}

				// 指数退避
				c.currentDelay *= 2
				if c.currentDelay > c.maxReconnectDelay {
					c.currentDelay = c.maxReconnectDelay
				}
				continue
			}

			// 连接成功，等待断开重连信号
			select {
			case <-ctx.Done():
				c.Close()
				return
			case <-c.stopCh:
				return
			case <-c.reconnectCh:
				// 再次检查是否因车辆离线而触发
				c.mu.RLock()
				offline := c.vehicleOffline
				c.mu.RUnlock()
				if offline {
					c.logger.Debug("Vehicle offline, not reconnecting",
						zap.Int64("vehicle_id", c.vehicleID))
					c.Close()
					return
				}

				c.logger.Info("Reconnecting streaming",
					zap.Int64("vehicle_id", c.vehicleID))
				c.Close()
				// 重置 stopCh 和 connected 状态
				c.mu.Lock()
				c.stopCh = make(chan struct{})
				c.mu.Unlock()
			}
		}
	}()
}

// Stop 停止客户端（包括重连循环）
func (c *StreamingClient) Stop() {
	c.Close()
}

// IsVehicleOffline 检查车辆是否离线
func (c *StreamingClient) IsVehicleOffline() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.vehicleOffline
}

// ResetAndRestart 重置离线标记并重新启动连接（当 RESTful 检测到车辆上线时调用）
func (c *StreamingClient) ResetAndRestart(ctx context.Context) {
	c.mu.Lock()
	c.vehicleOffline = false
	c.currentDelay = c.reconnectDelay
	c.stopCh = make(chan struct{})
	c.mu.Unlock()

	c.logger.Info("Streaming reset and restarting",
		zap.Int64("vehicle_id", c.vehicleID))

	c.StartWithReconnect(ctx)
}
