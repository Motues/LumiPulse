package http

import (
	"lumipluse-backend/internal/model"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// CreateIncident 发布新故障事件
func (h *Handler) CreateIncident(c *gin.Context) {
	var req model.CreateIncidentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request body"})
		return
	}

	// Validate impact
	switch req.Impact {
	case "minor", "major", "critical":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Impact must be minor, major, or critical"})
		return
	}

	if req.Status == "" {
		req.Status = "investigating"
	}

	inc := &model.Incident{
		ServiceID: req.ServiceID,
		Title:     req.Title,
		Impact:    req.Impact,
		Status:    req.Status,
	}

	if err := h.Repo.CreateIncident(c.Request.Context(), inc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to create incident"})
		return
	}

	// Update service status based on impact
	if svc, err := h.Repo.GetService(c.Request.Context(), req.ServiceID); err == nil {
		switch req.Impact {
		case "critical":
			svc.Status = "outage"
		case "major":
			svc.Status = "degraded"
		case "minor":
			svc.Status = "degraded"
		}
		h.Repo.UpdateService(c.Request.Context(), svc)
	}

	c.JSON(http.StatusCreated, model.APIResponse{
		Code:    201,
		Message: "Incident created",
		Data:    inc,
	})
}

// CreateIncidentUpdate 为现有故障添加进展更新
func (h *Handler) CreateIncidentUpdate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid incident id"})
		return
	}

	// Verify incident exists
	inc, err := h.Repo.GetIncident(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Incident not found"})
		return
	}

	var req model.CreateIncidentUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request body"})
		return
	}

	update := &model.IncidentUpdate{
		IncidentID: id,
		Status:     req.Status,
		Content:    req.Content,
	}

	if err := h.Repo.CreateIncidentUpdate(c.Request.Context(), update); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to create update"})
		return
	}

	// Sync status to incident record
	inc.Status = req.Status
	h.Repo.UpdateIncident(c.Request.Context(), inc)

	// Update service status based on incident state
	if svc, err := h.Repo.GetService(c.Request.Context(), inc.ServiceID); err == nil {
		if req.Status == "resolved" {
			svc.Status = "operational"
		} else {
			switch inc.Impact {
			case "critical":
				svc.Status = "outage"
			default:
				svc.Status = "degraded"
			}
		}
		h.Repo.UpdateService(c.Request.Context(), svc)
	}

	c.JSON(http.StatusCreated, model.APIResponse{
		Code:    201,
		Message: "Incident update created",
		Data:    update,
	})
}

// UpdateIncident 更改故障级别或标记为已解决
func (h *Handler) UpdateIncident(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid incident id"})
		return
	}

	inc, err := h.Repo.GetIncident(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Incident not found"})
		return
	}

	var req model.UpdateIncidentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request body"})
		return
	}

	if req.Title != "" {
		inc.Title = req.Title
	}
	if req.Impact != "" {
		inc.Impact = req.Impact
	}
	if req.Status != "" {
		inc.Status = req.Status

		// If resolved, restore service status
		if req.Status == "resolved" {
			if svc, err := h.Repo.GetService(c.Request.Context(), inc.ServiceID); err == nil {
				svc.Status = "operational"
				h.Repo.UpdateService(c.Request.Context(), svc)
			}
		}
	}

	if err := h.Repo.UpdateIncident(c.Request.Context(), inc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to update incident"})
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "Incident updated",
		Data:    inc,
	})
}

// DeleteIncident 删除故障事件
func (h *Handler) DeleteIncident(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid incident id"})
		return
	}

	// Restore service status before deleting
	if inc, err := h.Repo.GetIncident(c.Request.Context(), id); err == nil {
		if inc.Status != "resolved" {
			if svc, err := h.Repo.GetService(c.Request.Context(), inc.ServiceID); err == nil {
				svc.Status = "operational"
				h.Repo.UpdateService(c.Request.Context(), svc)
			}
		}
	}

	if err := h.Repo.DeleteIncident(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to delete incident"})
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "Incident deleted",
	})
}

// AdminListIncidents 获取故障事件列表（管理员用，含详细信息和全部状态）
func (h *Handler) AdminListIncidents(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}

	incidents, total, err := h.Repo.ListIncidents(c.Request.Context(), page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to fetch incidents"})
		return
	}

	for i := range incidents {
		updates, err := h.Repo.ListIncidentUpdates(c.Request.Context(), incidents[i].ID)
		if err == nil {
			incidents[i].Updates = updates
		}
	}

	totalPage := (total + int64(limit) - 1) / int64(limit)
	if total == 0 {
		totalPage = 0
	}
	if incidents == nil {
		incidents = []*model.Incident{}
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data: gin.H{
			"incidents":  incidents,
			"pagination": model.Pagination{Page: page, Limit: limit, TotalPage: totalPage},
		},
	})
}
