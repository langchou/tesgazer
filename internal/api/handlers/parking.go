package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ListParkings 获取停车列表
func (h *Handler) ListParkings(c *gin.Context) {
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

	parkings, err := h.parkingRepo.ListByCarID(c.Request.Context(), carID, perPage, offset)
	if err != nil {
		h.logger.Error("Failed to list parkings", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list parkings"})
		return
	}

	total, _ := h.parkingRepo.CountByCarID(c.Request.Context(), carID)

	c.JSON(http.StatusOK, gin.H{
		"data": parkings,
		"pagination": gin.H{
			"page":     page,
			"per_page": perPage,
			"total":    total,
		},
	})
}

// GetParking 获取停车详情
func (h *Handler) GetParking(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid parking ID"})
		return
	}

	parking, err := h.parkingRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Parking not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": parking})
}

// GetParkingEvents 获取停车事件列表
// GET /api/parkings/:id/events
func (h *Handler) GetParkingEvents(c *gin.Context) {
	parkingID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid parking ID"})
		return
	}

	// 先检查停车记录是否存在
	_, err = h.parkingRepo.GetByID(c.Request.Context(), parkingID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Parking not found"})
		return
	}

	events, err := h.parkingRepo.ListEventsByParkingID(c.Request.Context(), parkingID)
	if err != nil {
		h.logger.Error("Failed to list parking events", zap.Error(err), zap.Int64("parking_id", parkingID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list parking events"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": events})
}
