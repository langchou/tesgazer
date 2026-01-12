package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/langchou/tesgazer/internal/repository"
	"github.com/langchou/tesgazer/internal/service"
	"github.com/langchou/tesgazer/pkg/ws"
)

// Handler HTTP 处理器
type Handler struct {
	logger         *zap.Logger
	carRepo        *repository.CarRepository
	driveRepo      *repository.DriveRepository
	chargeRepo     *repository.ChargeRepository
	posRepo        *repository.PositionRepository
	parkingRepo    *repository.ParkingRepository
	vehicleService *service.VehicleService
	wsHub          *ws.Hub
	upgrader       websocket.Upgrader
}

// NewHandler 创建处理器
func NewHandler(
	logger *zap.Logger,
	carRepo *repository.CarRepository,
	driveRepo *repository.DriveRepository,
	chargeRepo *repository.ChargeRepository,
	posRepo *repository.PositionRepository,
	parkingRepo *repository.ParkingRepository,
	vehicleService *service.VehicleService,
	wsHub *ws.Hub,
) *Handler {
	return &Handler{
		logger:         logger,
		carRepo:        carRepo,
		driveRepo:      driveRepo,
		chargeRepo:     chargeRepo,
		posRepo:        posRepo,
		parkingRepo:    parkingRepo,
		vehicleService: vehicleService,
		wsHub:          wsHub,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // 开发环境允许所有来源
			},
		},
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	// API 路由
	api := r.Group("/api")
	{
		// 车辆
		api.GET("/cars", h.ListCars)
		api.GET("/cars/:id", h.GetCar)
		api.GET("/cars/:id/state", h.GetCarState)
		api.POST("/cars/:id/suspend", h.SuspendLogging) // 暂停日志记录
		api.POST("/cars/:id/resume", h.ResumeLogging)   // 恢复日志记录
		api.GET("/cars/:id/stats", h.GetCarStats)

		// 行程
		api.GET("/cars/:id/drives", h.ListDrives)
		api.GET("/drives/:id", h.GetDrive)
		api.GET("/drives/:id/positions", h.GetDrivePositions)
		api.GET("/cars/:id/footprint", h.GetFootprint)

		// 充电
		api.GET("/cars/:id/charges", h.ListCharges)
		api.GET("/charges/:id", h.GetCharge)
		api.GET("/charges/:id/details", h.GetChargeDetails)

		// 停车
		api.GET("/cars/:id/parkings", h.ListParkings)
		api.GET("/parkings/:id", h.GetParking)
		api.GET("/parkings/:id/events", h.GetParkingEvents)
	}

	// WebSocket
	r.GET("/ws", h.HandleWebSocket)

	// 健康检查
	r.GET("/health", h.HealthCheck)
}

// HandleWebSocket WebSocket 处理
func (h *Handler) HandleWebSocket(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade websocket", zap.Error(err))
		return
	}

	client := ws.NewClient(h.wsHub, conn)
	client.Register()

	// 启动读写协程
	go client.ReadPump()
	go client.WritePump()
}

// HealthCheck 健康检查
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":     "ok",
		"ws_clients": h.wsHub.ClientCount(),
	})
}
