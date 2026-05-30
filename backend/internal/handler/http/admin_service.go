package http

import (
	"fmt"
	"log"
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

	// Validation
	var errs validationErrors
	if msg := validateServiceName(req.Name); msg != "" {
		errs = append(errs, msg)
	}
	if msg := validateServiceURL(req.URL); msg != "" {
		errs = append(errs, msg)
	}
	if req.Interval > 0 {
		if msg := validateServiceInterval(req.Interval); msg != "" {
			errs = append(errs, msg)
		}
	}
	if errs.HasErrors() {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": errs.Error()})
		return
	}

	svc := &model.Service{
		Name:        req.Name,
		Description: req.Description,
		URL:         req.URL,
		Type:        req.Type,
		Interval:    req.Interval,
		SortOrder:   req.SortOrder,
		}
		if req.ShowOnHomepage != nil {
			svc.ShowOnHomepage = *req.ShowOnHomepage
		}

		if err := h.Repo.CreateService(c.Request.Context(), svc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to create service"})
		return
	}

	// Auto-create probe task for new service
	probeTask := &model.ProbeTask{
		ServiceID:    svc.ID,
		TriggerCount: 5,
		IsActive:     true,
	}
	if err := h.Repo.CreateProbeTask(c.Request.Context(), probeTask); err != nil {
		log.Printf("[handler] failed to auto-create probe task for service %d: %v", svc.ID, err)
	}

	auditLog("service.create", fmt.Sprintf("name=%s url=%s", svc.Name, svc.URL))

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
	if req.ShowOnHomepage != nil {
		svc.ShowOnHomepage = *req.ShowOnHomepage
	}
	svc.SortOrder = req.SortOrder

	if err := h.Repo.UpdateService(c.Request.Context(), svc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to update service"})
		return
	}

	auditLog("service.update", fmt.Sprintf("id=%d name=%s", svc.ID, svc.Name))

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

	svc, _ := h.Repo.GetService(c.Request.Context(), id)

	if err := h.Repo.DeleteService(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to delete service"})
		return
	}

	if svc != nil {
		auditLog("service.delete", fmt.Sprintf("id=%d name=%s", id, svc.Name))
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "Service deleted",
	})
}

// AdminReorderServices 批量调整服务排序
func (h *Handler) AdminReorderServices(c *gin.Context) {
	var req model.ReorderServicesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request body"})
		return
	}

	for _, item := range req.Services {
		if err := h.Repo.UpdateServiceSortOrder(c.Request.Context(), item.ID, item.SortOrder); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to reorder services"})
			return
		}
	}

	auditLog("service.reorder", fmt.Sprintf("count=%d", len(req.Services)))

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "Services reordered",
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

	svcIDs := make([]int64, len(services))
	for i, svc := range services {
		svcIDs[i] = svc.ID
	}
	uptimeMap := h.batchCalcUptime(c, svcIDs, 90)

	// Batch load latest heartbeat for all services
	latencyMap := make(map[int64]int, len(services))
	for _, svc := range services {
		if hb, err := h.Repo.GetLatestHeartbeat(c.Request.Context(), svc.ID); err == nil {
			latencyMap[svc.ID] = hb.Latency
		}
	}

	result := make([]ServiceDetail, 0, len(services))
	for _, svc := range services {
		result = append(result, ServiceDetail{
			Service: *svc,
			Uptime:  uptimeMap[svc.ID],
			Latency: latencyMap[svc.ID],
		})
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data:    result,
	})
}
