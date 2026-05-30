package http

import (
	"fmt"
	"lumipluse-backend/internal/model"
	"net/http"
	"strconv"
	"strings"

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
		ServiceID:        req.ServiceID,
		Title:            req.Title,
		Impact:           req.Impact,
		Status:           req.Status,
		AffectedServices: fmt.Sprintf("%d", req.ServiceID),
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
	auditLog("incident.create", fmt.Sprintf("title=%s impact=%s", inc.Title, inc.Impact))

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

	// Sync status to child events
	h.Repo.UpdateChildrenStatus(c.Request.Context(), id, req.Status)

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

// UpdateIncidentUpdate 修改事件更新记录
func (h *Handler) UpdateIncidentUpdate(c *gin.Context) {
	updateIDStr := c.Param("updateId")
	updateID, err := strconv.ParseInt(updateIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid update id"})
		return
	}

	var req model.CreateIncidentUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request body"})
		return
	}

	update := &model.IncidentUpdate{
		ID:      updateID,
		Status:  req.Status,
		Content: req.Content,
	}

	if err := h.Repo.UpdateIncidentUpdate(c.Request.Context(), update); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to update"})
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "Incident update modified",
	})
}

// DeleteIncidentUpdate 删除事件更新记录
func (h *Handler) DeleteIncidentUpdate(c *gin.Context) {
	updateIDStr := c.Param("updateId")
	updateID, err := strconv.ParseInt(updateIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid update id"})
		return
	}

	if err := h.Repo.DeleteIncidentUpdate(c.Request.Context(), updateID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to delete"})
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "Incident update deleted",
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

	// Sync status to child events
	if req.Status != "" {
		h.Repo.UpdateChildrenStatus(c.Request.Context(), id, req.Status)
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

	if len(incidents) > 0 {
		incIDs := make([]int64, len(incidents))
		for i, inc := range incidents {
			incIDs[i] = inc.ID
		}
		updatesMap, err := h.Repo.BatchListIncidentUpdates(c.Request.Context(), incIDs)
		if err == nil {
			for _, inc := range incidents {
				inc.Updates = updatesMap[inc.ID]
			}
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

// GetAdminIncident 获取单个事件详情（管理员用，含子事件和所有更新）
func (h *Handler) GetAdminIncident(c *gin.Context) {
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

	// Load all updates including internal ones (admin view)
	updates, err := h.Repo.ListIncidentUpdates(c.Request.Context(), id)
	if err != nil {
		updates = []*model.IncidentUpdate{}
	}
	inc.Updates = updates

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data:    inc,
	})
}

// MergeIncident 合并事件 — 将源事件作为子事件挂到当前事件下
func (h *Handler) MergeIncident(c *gin.Context) {
	idStr := c.Param("id")
	targetID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid incident id"})
		return
	}

	var req model.MergeIncidentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request body"})
		return
	}

	if req.SourceID == targetID {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Cannot merge incident with itself"})
		return
	}

	target, err := h.Repo.GetIncident(c.Request.Context(), targetID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Target incident not found"})
		return
	}

	source, err := h.Repo.GetIncident(c.Request.Context(), req.SourceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Source incident not found"})
		return
	}

	if source.Status == "resolved" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Cannot merge a resolved incident"})
		return
	}
	if source.ParentID != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Source incident is already a child of another incident"})
		return
	}

	// Set source as child of target
	source.ParentID = &target.ID
	if err := h.Repo.UpdateIncident(c.Request.Context(), source); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to merge incidents"})
		return
	}

	// Merge affected services into target for status tracking
	targetSvcs := parseAffectedServices(target.AffectedServices)
	sourceSvcs := parseAffectedServices(source.AffectedServices)
	merged := appendIfMissing(targetSvcs, sourceSvcs...)
	target.AffectedServices = joinServiceIDs(merged)
	h.Repo.UpdateIncident(c.Request.Context(), target)

	// Add merge notes
	h.Repo.CreateIncidentUpdate(c.Request.Context(), &model.IncidentUpdate{
		IncidentID: target.ID,
		Status:     "system",
		Content:    fmt.Sprintf("已合并子事件 #%d（%s）", source.ID, source.Title),
		IsInternal: true,
	})
	h.Repo.CreateIncidentUpdate(c.Request.Context(), &model.IncidentUpdate{
		IncidentID: source.ID,
		Status:     "system",
		Content:    fmt.Sprintf("已作为子事件合并到 #%d（%s）", target.ID, target.Title),
		IsInternal: true,
	})

	auditLog("incident.merge", fmt.Sprintf("target=%d source=%d", targetID, source.ID))

	// Reload target with children
	target, _ = h.Repo.GetIncident(c.Request.Context(), targetID)

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "Incidents merged",
		Data:    target,
	})
}

// SplitIncident 拆分 — 将子事件从父事件中拆分出去
func (h *Handler) SplitIncident(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid incident id"})
		return
	}

	child, err := h.Repo.GetIncident(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Incident not found"})
		return
	}

	if child.ParentID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Incident is not a child event, nothing to split"})
		return
	}

	parentID := *child.ParentID

	// Clear parent_id to split out
	child.ParentID = nil
	if err := h.Repo.UpdateIncident(c.Request.Context(), child); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to split incident"})
		return
	}

	// Add split notes
	h.Repo.CreateIncidentUpdate(c.Request.Context(), &model.IncidentUpdate{
		IncidentID: child.ID,
		Status:     "system",
		Content:    fmt.Sprintf("已从父事件 #%d 拆分出来", parentID),
		IsInternal: true,
	})
	h.Repo.CreateIncidentUpdate(c.Request.Context(), &model.IncidentUpdate{
		IncidentID: parentID,
		Status:     "system",
		Content:    fmt.Sprintf("子事件 #%d 已拆分出去", child.ID),
		IsInternal: true,
	})

	auditLog("incident.split", fmt.Sprintf("id=%d parent=%d", id, parentID))

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "Incident split",
	})
}

// Helper functions for affected_services
func parseAffectedServices(s string) []int64 {
	if s == "" {
		return nil
	}
	var ids []int64
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if id, err := strconv.ParseInt(p, 10, 64); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

func appendIfMissing(existing []int64, newIDs ...int64) []int64 {
	existMap := make(map[int64]bool)
	for _, id := range existing {
		existMap[id] = true
	}
	result := make([]int64, len(existing))
	copy(result, existing)
	for _, id := range newIDs {
		if !existMap[id] {
			result = append(result, id)
		}
	}
	return result
}

func joinServiceIDs(ids []int64) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	return strings.Join(parts, ",")
}
