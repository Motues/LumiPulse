package http

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"lumipluse-backend/internal/model"
	"lumipluse-backend/internal/pkg/utils"

	"github.com/gin-gonic/gin"
)

// ServiceFolderResponse 分组响应：带上下属服务数量，便于前端列表直接展示
type ServiceFolderResponse struct {
	model.ServiceFolder
	ServiceCount int `json:"serviceCount"`
}

// AdminListServiceFolders 获取全部分组
func (h *Handler) AdminListServiceFolders(c *gin.Context) {
	folders, err := h.Repo.ListServiceFolders(c.Request.Context())
	if err != nil {
		utils.Error("list service folders failed: %v", err)
		c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: "Failed to fetch service folders"})
		return
	}

	result := make([]ServiceFolderResponse, 0, len(folders))
	for _, f := range folders {
		count, _ := h.Repo.CountServicesInFolder(c.Request.Context(), f.ID)
		result = append(result, ServiceFolderResponse{ServiceFolder: *f, ServiceCount: count})
	}

	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "ok", Data: result})
}

// CreateServiceFolder 创建分组
func (h *Handler) CreateServiceFolder(c *gin.Context) {
	var req model.CreateServiceFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request body"})
		return
	}
	if msg := validateFolderName(req.Name); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": msg})
		return
	}

	folder := &model.ServiceFolder{
		Name:           strings.TrimSpace(req.Name),
		Description:    req.Description,
		ShowOnHomepage: true,
		SortOrder:      req.SortOrder,
	}
	if req.ShowOnHomepage != nil {
		folder.ShowOnHomepage = *req.ShowOnHomepage
	}

	if err := h.Repo.CreateServiceFolder(c.Request.Context(), folder); err != nil {
		utils.Error("create service folder failed: %v", err)
		c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: "Failed to create service folder"})
		return
	}

	auditLog("service_folder.create", fmt.Sprintf("name=%s", folder.Name))
	h.InvalidateSummary()

	c.JSON(http.StatusCreated, model.APIResponse{Code: 201, Message: "Service folder created", Data: folder})
}

// UpdateServiceFolder 修改分组
func (h *Handler) UpdateServiceFolder(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid folder id"})
		return
	}

	folder, err := h.Repo.GetServiceFolder(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Service folder not found"})
		return
	}

	var req model.UpdateServiceFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request body"})
		return
	}

	if req.Name != nil {
		if msg := validateFolderName(*req.Name); msg != "" {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": msg})
			return
		}
		folder.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		folder.Description = *req.Description
	}
	if req.ShowOnHomepage != nil {
		folder.ShowOnHomepage = *req.ShowOnHomepage
	}
	if req.SortOrder != nil {
		folder.SortOrder = *req.SortOrder
	}

	if err := h.Repo.UpdateServiceFolder(c.Request.Context(), folder); err != nil {
		utils.Error("update service folder %d failed: %v", id, err)
		c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: "Failed to update service folder"})
		return
	}

	auditLog("service_folder.update", fmt.Sprintf("id=%d name=%s", folder.ID, folder.Name))
	h.InvalidateSummary()

	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "Service folder updated", Data: folder})
}

// DeleteServiceFolder 删除分组。
// 分组下的服务不会被删除，只是回到「未分组」独立展示。
func (h *Handler) DeleteServiceFolder(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid folder id"})
		return
	}

	folder, _ := h.Repo.GetServiceFolder(c.Request.Context(), id)

	if err := h.Repo.DeleteServiceFolder(c.Request.Context(), id); err != nil {
		utils.Error("delete service folder %d failed: %v", id, err)
		c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: "Failed to delete service folder"})
		return
	}

	if folder != nil {
		auditLog("service_folder.delete", fmt.Sprintf("id=%d name=%s", id, folder.Name))
	}
	h.InvalidateSummary()

	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "Service folder deleted"})
}

// AssignServiceFolder 把服务移动到指定分组（folderId 传 null / 0 表示取消分组）
func (h *Handler) AssignServiceFolder(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid service id"})
		return
	}

	var req model.AssignServiceFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request body"})
		return
	}

	if _, err := h.Repo.GetService(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Service not found"})
		return
	}

	var folderID *int64
	if req.FolderID != nil && *req.FolderID > 0 {
		if _, err := h.Repo.GetServiceFolder(c.Request.Context(), *req.FolderID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Service folder not found"})
			return
		}
		folderID = req.FolderID
	}

	if err := h.Repo.AssignServiceFolder(c.Request.Context(), id, folderID); err != nil {
		utils.Error("assign service %d to folder failed: %v", id, err)
		c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: "Failed to assign service folder"})
		return
	}

	auditLog("service_folder.assign", fmt.Sprintf("service=%d folder=%v", id, folderID))
	h.InvalidateSummary()

	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "Service folder assigned"})
}

// AdminFolderSummaries 管理端获取所有分组的融合结果（含未在首页展示的分组）。
// 与公开总览共用同一套融合口径，保证管理端预览与首页展示一致。
// 状态直接取服务表里的 status（由检查器与事件流程维护），
// 与公开页面的差异仅在于「公开页面还会按在办事件补偿状态」。
func (h *Handler) AdminFolderSummaries(c *gin.Context) {
	services, err := h.Repo.ListServices(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: "Failed to fetch services"})
		return
	}

	svcIDs := make([]int64, len(services))
	for i, svc := range services {
		svcIDs[i] = svc.ID
	}
	uptimeMap := h.batchCalcUptime(c, svcIDs, 90)
	latestMap, _ := h.Repo.BatchGetLatestHeartbeats(c.Request.Context(), svcIDs)

	summaries := make([]model.ServiceSummary, 0, len(services))
	for _, svc := range services {
		latency := 0
		if hb, ok := latestMap[svc.ID]; ok {
			latency = hb.Latency
		}
		summaries = append(summaries, model.ServiceSummary{
			ID:         svc.ID,
			PublicHash: svc.PublicHash,
			FolderID:   svc.FolderID,
			Name:       svc.Name,
			Status:     svc.Status,
			URL:        svc.URL,
			Type:       svc.Type,
			Uptime:     uptimeMap[svc.ID],
			Latency:    latency,
			Interval:   svc.Interval,
		})
	}

	// folderIDs 传 nil：不过滤分组，返回全部分组的融合结果
	folders := h.buildFolderSummaries(c, services, summaries, nil)

	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "ok", Data: folders})
}

// validateFolderName 校验分组名称
func validateFolderName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "分组名称不能为空"
	}
	if len([]rune(name)) > 100 {
		return "分组名称不超过100个字符"
	}
	return ""
}
