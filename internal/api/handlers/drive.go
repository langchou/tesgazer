package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

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

// GetFootprint 获取足迹数据 (批量行程轨迹)
func (h *Handler) GetFootprint(c *gin.Context) {
	carID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid car ID"})
		return
	}

	// 默认最近 90 天
	end := time.Now()
	start := end.AddDate(0, 0, -90)

	if s := c.Query("start"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			start = t
		}
	}
	if e := c.Query("end"); e != "" {
		if t, err := time.Parse(time.RFC3339, e); err == nil {
			end = t
		}
	}

	paths, err := h.driveRepo.GetDrivePathsInRange(c.Request.Context(), carID, start, end)
	if err != nil {
		h.logger.Error("Failed to get drive paths", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get footprint data"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": paths})
}
