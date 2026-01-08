package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

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
