package ws

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// MessageType WebSocket 消息类型
const (
	MsgTypeInit        = "init"         // 初始化数据（车辆列表+状态）
	MsgTypeStateUpdate = "state_update" // 状态更新
	MsgTypeError       = "error"        // 错误消息
)

// Message WebSocket 消息结构
type Message struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// InitData 初始化数据
type InitData struct {
	Cars   interface{} `json:"cars"`
	States interface{} `json:"states"`
}

// Client WebSocket 客户端
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// Hub WebSocket 连接管理中心
type Hub struct {
	logger     *zap.Logger
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex

	// 初始数据提供者回调
	getInitData func() *InitData
}

// NewHub 创建 Hub
func NewHub(logger *zap.Logger) *Hub {
	return &Hub{
		logger:     logger,
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// SetInitDataProvider 设置初始数据提供者
func (h *Hub) SetInitDataProvider(provider func() *InitData) {
	h.getInitData = provider
}

// Run 运行 Hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			h.logger.Info("WebSocket client connected", zap.Int("total_clients", len(h.clients)))

			// 发送初始数据
			h.sendInitData(client)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			h.logger.Info("WebSocket client disconnected", zap.Int("total_clients", len(h.clients)))

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// 慢消费者，关闭连接
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// sendInitData 发送初始数据给新连接的客户端
func (h *Hub) sendInitData(client *Client) {
	if h.getInitData == nil {
		h.logger.Warn("No init data provider set")
		return
	}

	initData := h.getInitData()
	if initData == nil {
		h.logger.Warn("Init data provider returned nil")
		return
	}

	msg := Message{
		Type: MsgTypeInit,
		Data: initData,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("Failed to marshal init data", zap.Error(err))
		return
	}

	select {
	case client.send <- data:
		h.logger.Debug("Sent init data to client")
	default:
		h.logger.Warn("Failed to send init data, client buffer full")
	}
}

// Broadcast 广播消息给所有客户端
func (h *Hub) Broadcast(message []byte) {
	h.broadcast <- message
}

// BroadcastMessage 广播结构化消息给所有客户端
func (h *Hub) BroadcastMessage(msgType string, data interface{}) {
	msg := Message{
		Type: msgType,
		Data: data,
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("Failed to marshal broadcast message", zap.Error(err))
		return
	}

	h.Broadcast(jsonData)
}

// BroadcastStateUpdate 广播状态更新
func (h *Hub) BroadcastStateUpdate(state interface{}) {
	h.BroadcastMessage(MsgTypeStateUpdate, state)
}

// ClientCount 获取客户端数量
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// NewClient 创建客户端
func NewClient(hub *Hub, conn *websocket.Conn) *Client {
	return &Client{
		hub:  hub,
		conn: conn,
		send: make(chan []byte, 256),
	}
}

// Register 注册客户端
func (c *Client) Register() {
	c.hub.register <- c
}

// Unregister 注销客户端
func (c *Client) Unregister() {
	c.hub.unregister <- c
}

// ReadPump 读取消息（保持连接活跃）
func (c *Client) ReadPump() {
	defer func() {
		c.Unregister()
		c.conn.Close()
	}()

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		// 简化版不处理客户端消息，仅保持连接
	}
}

// WritePump 发送消息
func (c *Client) WritePump() {
	defer c.conn.Close()

	for message := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
			break
		}
	}
}
