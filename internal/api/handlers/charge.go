package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

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
