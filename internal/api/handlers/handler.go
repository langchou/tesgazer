package handlers

import (
	"net/http"
	"strconv"

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
	vehicleService *service.VehicleService,
	wsHub *ws.Hub,
) *Handler {
	return &Handler{
		logger:         logger,
		carRepo:        carRepo,
		driveRepo:      driveRepo,
		chargeRepo:     chargeRepo,
		posRepo:        posRepo,
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
		api.POST("/cars/:id/suspend", h.SuspendLogging)  // 暂停日志记录
		api.POST("/cars/:id/resume", h.ResumeLogging)    // 恢复日志记录

		// 行程
		api.GET("/cars/:id/drives", h.ListDrives)
		api.GET("/drives/:id", h.GetDrive)
		api.GET("/drives/:id/positions", h.GetDrivePositions)

		// 充电
		api.GET("/cars/:id/charges", h.ListCharges)
		api.GET("/charges/:id", h.GetCharge)
		api.GET("/charges/:id/details", h.GetChargeDetails)

		// 统计
		api.GET("/cars/:id/stats", h.GetCarStats)
	}

	// WebSocket
	r.GET("/ws", h.HandleWebSocket)

	// 健康检查
	r.GET("/health", h.HealthCheck)
}

// ListCars 获取车辆列表
func (h *Handler) ListCars(c *gin.Context) {
	cars, err := h.carRepo.List(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to list cars", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list cars"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": cars})
}

// GetCar 获取车辆详情
func (h *Handler) GetCar(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid car ID"})
		return
	}

	car, err := h.carRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Car not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": car})
}

// GetCarState 获取车辆实时状态
func (h *Handler) GetCarState(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid car ID"})
		return
	}

	state, ok := h.vehicleService.GetState(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Car state not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": state})
}

// SuspendLogging 暂停日志记录
// POST /api/cars/:id/suspend
// 手动暂停车辆的日志记录，允许车辆进入休眠以减少吸血鬼功耗
func (h *Handler) SuspendLogging(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid car ID"})
		return
	}

	if err := h.vehicleService.SuspendLogging(id); err != nil {
		h.logger.Error("Failed to suspend logging", zap.Error(err), zap.Int64("car_id", id))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Logging suspended via API", zap.Int64("car_id", id))
	c.JSON(http.StatusOK, gin.H{
		"message": "Logging suspended",
		"car_id":  id,
	})
}

// ResumeLogging 恢复日志记录
// POST /api/cars/:id/resume
// 手动恢复车辆的日志记录
func (h *Handler) ResumeLogging(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid car ID"})
		return
	}

	if err := h.vehicleService.ResumeLogging(id); err != nil {
		h.logger.Error("Failed to resume logging", zap.Error(err), zap.Int64("car_id", id))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Logging resumed via API", zap.Int64("car_id", id))
	c.JSON(http.StatusOK, gin.H{
		"message": "Logging resumed",
		"car_id":  id,
	})
}

// ListDrives 获取行程列表
func (h *Handler) ListDrives(c *gin.Context) {
	carID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid car ID"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	offset := (page - 1) * perPage

	drives, err := h.driveRepo.ListByCarID(c.Request.Context(), carID, perPage, offset)
	if err != nil {
		h.logger.Error("Failed to list drives", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list drives"})
		return
	}

	total, _ := h.driveRepo.CountByCarID(c.Request.Context(), carID)

	c.JSON(http.StatusOK, gin.H{
		"data": drives,
		"pagination": gin.H{
			"page":     page,
			"per_page": perPage,
			"total":    total,
		},
	})
}

// GetDrive 获取行程详情
func (h *Handler) GetDrive(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid drive ID"})
		return
	}

	drive, err := h.driveRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Drive not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": drive})
}

// GetDrivePositions 获取行程轨迹
func (h *Handler) GetDrivePositions(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid drive ID"})
		return
	}

	positions, err := h.posRepo.ListByDriveID(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("Failed to list positions", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list positions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": positions})
}

// ListCharges 获取充电列表
func (h *Handler) ListCharges(c *gin.Context) {
	carID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid car ID"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	offset := (page - 1) * perPage

	charges, err := h.chargeRepo.ListProcessesByCarID(c.Request.Context(), carID, perPage, offset)
	if err != nil {
		h.logger.Error("Failed to list charges", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list charges"})
		return
	}

	total, _ := h.chargeRepo.CountProcessesByCarID(c.Request.Context(), carID)

	c.JSON(http.StatusOK, gin.H{
		"data": charges,
		"pagination": gin.H{
			"page":     page,
			"per_page": perPage,
			"total":    total,
		},
	})
}

// GetCharge 获取充电详情
func (h *Handler) GetCharge(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid charge ID"})
		return
	}

	charge, err := h.chargeRepo.GetProcessByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Charge not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": charge})
}

// GetChargeDetails 获取充电曲线数据
func (h *Handler) GetChargeDetails(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid charge ID"})
		return
	}

	charges, err := h.chargeRepo.ListChargesByProcessID(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("Failed to list charge details", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list charge details"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": charges})
}

// GetCarStats 获取车辆统计
func (h *Handler) GetCarStats(c *gin.Context) {
	carID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid car ID"})
		return
	}

	car, err := h.carRepo.GetByID(c.Request.Context(), carID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Car not found"})
		return
	}

	driveCount, _ := h.driveRepo.CountByCarID(c.Request.Context(), carID)
	chargeCount, _ := h.chargeRepo.CountProcessesByCarID(c.Request.Context(), carID)

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"car":          car,
			"drive_count":  driveCount,
			"charge_count": chargeCount,
		},
	})
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
		"status": "ok",
		"ws_clients": h.wsHub.ClientCount(),
	})
}
