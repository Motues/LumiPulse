package http

import (
	"lumipluse-backend/internal/model"
	"lumipluse-backend/internal/pkg/utils"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// validateRecurrence 校验并规范化周期维护参数。返回空字符串表示通过。
// recurrence 为空即一次性维护窗口，其余字段会被清空以免残留脏数据。
func validateRecurrence(m *model.Maintenance) string {
	switch m.Recurrence {
	case "":
		m.RecurrenceInterval = 0
		m.RecurrenceWeekday = 0
		m.RecurrenceMonthday = 0
		m.RecurrenceUntil = ""
		return ""
	case "daily", "weekly", "monthly":
	default:
		return "重复方式只能是 daily / weekly / monthly"
	}

	if m.RecurrenceInterval == 0 {
		m.RecurrenceInterval = 1
	}
	if m.RecurrenceInterval < 1 || m.RecurrenceInterval > 365 {
		return "重复间隔需在 1 ~ 365 之间"
	}

	if m.Recurrence == "weekly" {
		if m.RecurrenceWeekday < 1 || m.RecurrenceWeekday > 7 {
			// 说明：实际重复日期由 scheduled_start 的星期决定，
			// recurrence_weekday 只记录用户选择的星期，便于前端回显。
			m.RecurrenceWeekday = 0
		}
		m.RecurrenceMonthday = 0
	} else {
		m.RecurrenceWeekday = 0
	}

	if m.Recurrence == "monthly" {
		if m.RecurrenceMonthday < 1 || m.RecurrenceMonthday > 31 {
			m.RecurrenceMonthday = 0
		}
	} else {
		m.RecurrenceMonthday = 0
	}

	if m.RecurrenceUntil != "" {
		if _, err := time.Parse("2006-01-02", m.RecurrenceUntil); err != nil {
			return "重复截止日期格式应为 YYYY-MM-DD"
		}
	}
	return ""
}

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
		Title:              req.Title,
		Description:        req.Description,
		ScheduledStart:     req.ScheduledStart,
		ScheduledEnd:       req.ScheduledEnd,
		Status:             req.Status,
		AffectedServices:   req.AffectedServices,
		Recurrence:         req.Recurrence,
		RecurrenceInterval: req.RecurrenceInterval,
		RecurrenceWeekday:  req.RecurrenceWeekday,
		RecurrenceMonthday: req.RecurrenceMonthday,
		RecurrenceUntil:    req.RecurrenceUntil,
	}
	if msg := validateRecurrence(m); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": msg})
		return
	}

	if err := h.Repo.CreateMaintenance(c.Request.Context(), m); err != nil {
		utils.Error("create maintenance failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to create maintenance"})
		return
	}

	h.InvalidateSummary()

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
	// 周期参数用指针区分「未传」与「显式清空」，否则无法把周期维护改回一次性
	if req.Recurrence != nil {
		m.Recurrence = *req.Recurrence
	}
	if req.RecurrenceInterval != nil {
		m.RecurrenceInterval = *req.RecurrenceInterval
	}
	if req.RecurrenceWeekday != nil {
		m.RecurrenceWeekday = *req.RecurrenceWeekday
	}
	if req.RecurrenceMonthday != nil {
		m.RecurrenceMonthday = *req.RecurrenceMonthday
	}
	if req.RecurrenceUntil != nil {
		m.RecurrenceUntil = *req.RecurrenceUntil
	}
	if msg := validateRecurrence(m); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": msg})
		return
	}

	if err := h.Repo.UpdateMaintenance(c.Request.Context(), m); err != nil {
		utils.Error("update maintenance %d failed: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to update maintenance"})
		return
	}

	h.InvalidateSummary()

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
		utils.Error("delete maintenance %d failed: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to delete maintenance"})
		return
	}

	h.InvalidateSummary()

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "Maintenance deleted",
	})
}

// AdminListMaintenances 获取维护计划列表（管理员用）
func (h *Handler) AdminListMaintenances(c *gin.Context) {
	maintenances, err := h.Repo.ListMaintenances(c.Request.Context())
	if err != nil {
		utils.Warn("list maintenances failed: %v", err)
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
