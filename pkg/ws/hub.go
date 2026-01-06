package ws

import (
	"sync"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Client WebSocket 客户端
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	carIDs map[int64]bool // 订阅的车辆 ID
}

// Hub WebSocket 连接管理中心
type Hub struct {
	logger     *zap.Logger
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
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

// Run 运行 Hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			h.logger.Debug("Client connected", zap.Int("total", len(h.clients)))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			h.logger.Debug("Client disconnected", zap.Int("total", len(h.clients)))

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast 广播消息给所有客户端
func (h *Hub) Broadcast(message []byte) {
	h.broadcast <- message
}

// BroadcastToCarSubscribers 广播消息给订阅特定车辆的客户端
func (h *Hub) BroadcastToCarSubscribers(carID int64, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if client.carIDs[carID] {
			select {
			case client.send <- message:
			default:
				// 跳过慢消费者
			}
		}
	}
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
		hub:    hub,
		conn:   conn,
		send:   make(chan []byte, 256),
		carIDs: make(map[int64]bool),
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

// SubscribeCar 订阅车辆
func (c *Client) SubscribeCar(carID int64) {
	c.carIDs[carID] = true
}

// UnsubscribeCar 取消订阅车辆
func (c *Client) UnsubscribeCar(carID int64) {
	delete(c.carIDs, carID)
}

// ReadPump 读取消息
func (c *Client) ReadPump(onMessage func([]byte)) {
	defer func() {
		c.Unregister()
		c.conn.Close()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		if onMessage != nil {
			onMessage(message)
		}
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
