package http

import (
	"context"
	"errors"
	"fmt"
	"lumipluse-backend/internal/model"
	"lumipluse-backend/internal/pkg/i18n"
	"lumipluse-backend/internal/pkg/utils"
	"lumipluse-backend/internal/repository"
	"net/http"
	"strconv"
	"strings"
	"time"

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
		utils.Error("create incident failed: %v", err)
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
	h.InvalidateSummary()

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
	if req.Status == "resolved" {
		// 合并事件可能覆盖多个服务，必须全部恢复，否则其余服务会永久停在故障态
		h.restoreIncidentServices(c.Request.Context(), inc)
	} else if svc, err := h.Repo.GetService(c.Request.Context(), inc.ServiceID); err == nil {
		switch inc.Impact {
		case "critical":
			svc.Status = "outage"
		default:
			svc.Status = "degraded"
		}
		h.Repo.UpdateService(c.Request.Context(), svc)
	}

	h.InvalidateSummary()

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
	}

	// 复盘字段：指针区分「未传」与「显式清空」，因此传空字符串也能把内容清掉
	if req.RootCause != nil {
		inc.RootCause = *req.RootCause
	}
	if req.Resolution != nil {
		inc.Resolution = *req.Resolution
	}
	if req.PostmortemURL != nil {
		inc.PostmortemURL = *req.PostmortemURL
	}
	if req.PostmortemPublic != nil {
		inc.PostmortemPublic = *req.PostmortemPublic
	}
	// 人工确认：确认后不再发送升级通知。取消确认会重置升级计数，
	// 让事件重新走一遍「未确认 → 升级」的流程（否则取消确认后不会再提醒）。
	ackChanged := false
	if req.Acknowledged != nil && *req.Acknowledged != inc.Acknowledged {
		inc.Acknowledged = *req.Acknowledged
		ackChanged = true
		if inc.Acknowledged {
			inc.AcknowledgedAt = time.Now().UTC().Format(time.RFC3339)
			inc.AcknowledgedBy = adminName()
		} else {
			inc.AcknowledgedAt = ""
			inc.AcknowledgedBy = ""
			inc.EscalationCount = 0
			inc.LastEscalatedAt = ""
		}
	}

	if err := h.Repo.UpdateIncident(c.Request.Context(), inc); err != nil {
		utils.Error("update incident %d failed: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to update incident"})
		return
	}

	// 确认 / 取消确认都记一条时间线，便于回溯谁在什么时候接手
	if ackChanged {
		note := i18n.Current().IncidentAcknowledgedNote(inc.AcknowledgedBy)
		if !inc.Acknowledged {
			note = i18n.Current().IncidentUnacknowledgedNote()
		}
		if err := h.Repo.CreateIncidentUpdate(c.Request.Context(), &model.IncidentUpdate{
			IncidentID: id,
			Status:     "system",
			Content:    note,
		}); err != nil {
			utils.Info("failed to record ack note for incident %d: %v", id, err)
		}
	}

	// Sync status to child events
	if req.Status != "" {
		h.Repo.UpdateChildrenStatus(c.Request.Context(), id, req.Status)
	}

	// 解决事件后恢复全部受影响服务的状态（合并事件可能覆盖多个服务）
	if req.Status == "resolved" {
		h.restoreIncidentServices(c.Request.Context(), inc)
	}

	h.InvalidateSummary()

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

	// Restore service status before deleting（合并事件要恢复全部受影响服务）
	if inc, err := h.Repo.GetIncident(c.Request.Context(), id); err == nil {
		if inc.Status != "resolved" {
			h.restoreIncidentServices(c.Request.Context(), inc)
		}
	}

	if err := h.Repo.DeleteIncident(c.Request.Context(), id); err != nil {
		utils.Error("delete incident %d failed: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to delete incident"})
		return
	}

	h.InvalidateSummary()

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

	// 管理端列表包含子事件：自动合并后的子事件不会在公开页出现，
	// 但后台必须能看到，避免"收到告警邮件却找不到事件"。
	incidents, total, err := h.Repo.ListIncidentsWithChildren(c.Request.Context(), page, limit)
	if err != nil {
		utils.Error("list incidents failed: %v", err)
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
	if target.ParentID != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Target incident is a child event and cannot host other incidents"})
		return
	}
	// 父事件已解决时不能接收子事件：解决时已经同步过子事件，之后挂进来的事件
	// 不会再被解决，会在公开列表里彻底消失（幽灵事件）。
	if target.Status == "resolved" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Cannot merge into a resolved incident"})
		return
	}

	// 合并必须在一个事务里完成（校验 + 写父子关系 + 重挂子树 + 合并受影响服务），
	// 否则并发的两个请求可能把 A 挂到 B 下、同时把 B 挂到 A 下，形成循环父子。
	if err := h.Repo.MergeIncidents(c.Request.Context(), target.ID, source.ID); err != nil {
		if errors.Is(err, repository.ErrIncidentConflict) {
			c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "事件状态已变化（已被解决或已合并），请刷新后重试"})
			return
		}
		utils.Error("merge incident failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to merge incidents"})
		return
	}

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

	h.InvalidateSummary()

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

	// 拆分同样要在事务里做：解除父子关系的同时，把只有该子事件还在声明的服务
	// 从父事件的 affected_services 中移除，否则父事件会继续为已拆走的服务兜底。
	parentID, err := h.Repo.SplitIncident(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrIncidentConflict) {
			c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "事件状态已变化（已被拆分或解决），请刷新后重试"})
			return
		}
		utils.Error("split incident %d failed: %v", id, err)
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
	h.InvalidateSummary()

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "Incident split",
	})
}

// restoreIncidentServices 把事件关联的所有服务恢复为 operational。
//
// 合并事件的 affected_services 可能包含多个服务、子事件也各自关联服务，
// 只恢复 inc.ServiceID 会让其余服务永久停在 degraded/outage：
// 检查器只会恢复它自己解决的那条事件，人工解决后不会再有人来擦这个状态。
func (h *Handler) restoreIncidentServices(ctx context.Context, inc *model.Incident) {
	if inc == nil {
		return
	}

	ids := make([]int64, 0, 4)
	if inc.ServiceID > 0 {
		ids = append(ids, inc.ServiceID)
	}
	for _, id := range parseAffectedServices(inc.AffectedServices) {
		ids = appendIfMissing(ids, id)
	}
	for _, child := range inc.Children {
		if child.ServiceID > 0 {
			ids = appendIfMissing(ids, child.ServiceID)
		}
		for _, id := range parseAffectedServices(child.AffectedServices) {
			ids = appendIfMissing(ids, id)
		}
	}

	for _, id := range ids {
		svc, err := h.Repo.GetService(ctx, id)
		if err != nil || svc == nil {
			continue // 服务已被删除
		}
		if svc.Status == "operational" {
			continue
		}
		svc.Status = "operational"
		if err := h.Repo.UpdateService(ctx, svc); err != nil {
			utils.Info("failed to restore service %d: %v", id, err)
		}
	}
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
