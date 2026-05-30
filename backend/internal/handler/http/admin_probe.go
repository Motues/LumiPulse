package http

import (
	"fmt"
	"lumipluse-backend/internal/model"
	"lumipluse-backend/internal/pkg/utils"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ListProbeTasks 获取所有探测任务列表
func (h *Handler) ListProbeTasks(c *gin.Context) {
	tasks, err := h.Repo.ListProbeTasks(c.Request.Context())
	if err != nil {
		utils.Warn("list probe tasks failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to fetch probe tasks"})
		return
	}
	if tasks == nil {
		tasks = []*model.ProbeTask{}
	}
	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "ok", Data: tasks})
}

// CreateProbeTask 创建探测任务
func (h *Handler) CreateProbeTask(c *gin.Context) {
	var req model.CreateProbeTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request body"})
		return
	}

	if req.TriggerCount <= 0 {
		req.TriggerCount = 5
	}

	// Check service exists
	if _, err := h.Repo.GetService(c.Request.Context(), req.ServiceID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Service not found"})
		return
	}

	// Check if probe task already exists for this service
	if existing, _ := h.Repo.GetProbeTaskByService(c.Request.Context(), req.ServiceID); existing != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "Probe task already exists for this service"})
		return
	}

	task := &model.ProbeTask{
		ServiceID:    req.ServiceID,
		ServerID:     req.ServerID,
		TriggerCount: req.TriggerCount,
		IsActive:     true,
	}
	if err := h.Repo.CreateProbeTask(c.Request.Context(), task); err != nil {
		utils.Error("create probe task failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to create probe task"})
		return
	}

	auditLog("probe.create", fmt.Sprintf("serviceId=%d triggerCount=%d", task.ServiceID, task.TriggerCount))

	c.JSON(http.StatusCreated, model.APIResponse{
		Code:    201,
		Message: "Probe task created",
		Data:    task,
	})
}

// UpdateProbeTask 修改探测任务
func (h *Handler) UpdateProbeTask(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid probe task id"})
		return
	}

	task, err := h.Repo.GetProbeTask(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Probe task not found"})
		return
	}

	var req model.UpdateProbeTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request body"})
		return
	}

	if req.TriggerCount > 0 {
		task.TriggerCount = req.TriggerCount
	}
	if req.IsActive != nil {
		task.IsActive = *req.IsActive
	}
	task.ServerID = req.ServerID

	if err := h.Repo.UpdateProbeTask(c.Request.Context(), task); err != nil {
		utils.Error("update probe task %d failed: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to update probe task"})
		return
	}

	auditLog("probe.update", fmt.Sprintf("id=%d triggerCount=%d", task.ID, task.TriggerCount))

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "Probe task updated",
		Data:    task,
	})
}

// DeleteProbeTask 删除探测任务
func (h *Handler) DeleteProbeTask(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid probe task id"})
		return
	}

	if err := h.Repo.DeleteProbeTask(c.Request.Context(), id); err != nil {
		utils.Error("delete probe task %d failed: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to delete probe task"})
		return
	}

	auditLog("probe.delete", fmt.Sprintf("id=%d", id))

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "Probe task deleted",
	})
}
