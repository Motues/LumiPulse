package http

import (
	"lumipluse-backend/internal/model"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// CreateService 创建新的监控服务
func (h *Handler) CreateService(c *gin.Context) {
	var req model.CreateServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request body"})
		return
	}

	if req.Type == "" {
		req.Type = "http"
	}
	if req.Interval == 0 {
		req.Interval = 60
	}

	svc := &model.Service{
		Name:        req.Name,
		Description: req.Description,
		URL:         req.URL,
		Type:        req.Type,
		Interval:    req.Interval,
		SortOrder:   req.SortOrder,
	}

	if err := h.Repo.CreateService(c.Request.Context(), svc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to create service"})
		return
	}

	c.JSON(http.StatusCreated, model.APIResponse{
		Code:    201,
		Message: "Service created",
		Data:    svc,
	})
}

// UpdateService 修改服务配置
func (h *Handler) UpdateService(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid service id"})
		return
	}

	svc, err := h.Repo.GetService(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Service not found"})
		return
	}

	var req model.UpdateServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request body"})
		return
	}

	if req.Name != "" {
		svc.Name = req.Name
	}
	if req.Description != "" {
		svc.Description = req.Description
	}
	if req.URL != "" {
		svc.URL = req.URL
	}
	if req.Type != "" {
		svc.Type = req.Type
	}
	if req.Interval > 0 {
		svc.Interval = req.Interval
	}
	if req.Status != "" {
		svc.Status = req.Status
	}
	if req.IsActive != nil {
		svc.IsActive = *req.IsActive
	}
	svc.SortOrder = req.SortOrder

	if err := h.Repo.UpdateService(c.Request.Context(), svc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to update service"})
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "Service updated",
		Data:    svc,
	})
}

// DeleteService 删除监控服务
func (h *Handler) DeleteService(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid service id"})
		return
	}

	if err := h.Repo.DeleteService(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to delete service"})
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "Service deleted",
	})
}

// AdminListServices 获取所有服务列表（管理员用，含详细信息和在线率）
func (h *Handler) AdminListServices(c *gin.Context) {
	services, err := h.Repo.ListServices(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to fetch services"})
		return
	}

	type ServiceDetail struct {
		model.Service
		Uptime    float64 `json:"uptime"`
		Latency   int     `json:"latency"`
	}

	result := make([]ServiceDetail, 0, len(services))
	for _, svc := range services {
		uptime := h.calcUptime(c, svc.ID, 90)
		latency := 0
		if hb, err := h.Repo.GetLatestHeartbeat(c.Request.Context(), svc.ID); err == nil {
			latency = hb.Latency
		}
		result = append(result, ServiceDetail{
			Service: *svc,
			Uptime:  uptime,
			Latency: latency,
		})
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data:    result,
	})
}
