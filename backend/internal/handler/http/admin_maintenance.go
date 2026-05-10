package http

import (
	"lumipluse-backend/internal/model"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// CreateMaintenance 创建维护计划
func (h *Handler) CreateMaintenance(c *gin.Context) {
	var req model.CreateMaintenanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request body"})
		return
	}

	if req.Status == "" {
		req.Status = "scheduled"
	}

	m := &model.Maintenance{
		Title:            req.Title,
		Description:      req.Description,
		ScheduledStart:   req.ScheduledStart,
		ScheduledEnd:     req.ScheduledEnd,
		Status:           req.Status,
		AffectedServices: req.AffectedServices,
	}

	if err := h.Repo.CreateMaintenance(c.Request.Context(), m); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to create maintenance"})
		return
	}

	c.JSON(http.StatusCreated, model.APIResponse{
		Code:    201,
		Message: "Maintenance created",
		Data:    m,
	})
}

// UpdateMaintenance 调整维护时间或说明
func (h *Handler) UpdateMaintenance(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid maintenance id"})
		return
	}

	m, err := h.Repo.GetMaintenance(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Maintenance not found"})
		return
	}

	var req model.UpdateMaintenanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request body"})
		return
	}

	if req.Title != "" {
		m.Title = req.Title
	}
	if req.Description != "" {
		m.Description = req.Description
	}
	if req.ScheduledStart != "" {
		m.ScheduledStart = req.ScheduledStart
	}
	if req.ScheduledEnd != "" {
		m.ScheduledEnd = req.ScheduledEnd
	}
	if req.Status != "" {
		m.Status = req.Status
	}
	if req.AffectedServices != "" {
		m.AffectedServices = req.AffectedServices
	}

	if err := h.Repo.UpdateMaintenance(c.Request.Context(), m); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to update maintenance"})
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "Maintenance updated",
		Data:    m,
	})
}

// DeleteMaintenance 删除维护计划
func (h *Handler) DeleteMaintenance(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid maintenance id"})
		return
	}

	if err := h.Repo.DeleteMaintenance(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to delete maintenance"})
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "Maintenance deleted",
	})
}

// AdminListMaintenances 获取维护计划列表（管理员用）
func (h *Handler) AdminListMaintenances(c *gin.Context) {
	maintenances, err := h.Repo.ListMaintenances(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to fetch maintenances"})
		return
	}
	if maintenances == nil {
		maintenances = []*model.Maintenance{}
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data:    maintenances,
	})
}
