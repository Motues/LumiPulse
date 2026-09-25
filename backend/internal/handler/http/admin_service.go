package http

import (
	"fmt"
	"lumipluse-backend/internal/model"
	"lumipluse-backend/internal/pkg/utils"
	"net/http"
	"strconv"
	"strings"

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
	if req.TimeoutSeconds != nil {
		if msg := validateServiceTimeout(*req.TimeoutSeconds); msg != "" {
			errs = append(errs, msg)
		}
	}
	// HTTP 探测高级匹配：空值一律维持默认行为，因此只校验填写的部分
	if msg := model.ValidateProbeConfig(req.HTTPMethod, req.HTTPHeaders, req.ExpectStatus); msg != "" {
		errs = append(errs, msg)
	}
	if msg := validateExpectKeyword(req.ExpectKeyword); msg != "" {
		errs = append(errs, msg)
	}
	if errs.HasErrors() {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": errs.Error()})
		return
	}

	// 请求方法归一化：空值等价于 GET，避免库里出现两种等价写法
	method, _ := model.NormalizeHTTPMethod(req.HTTPMethod)

	// 分组归属：0 / 未传表示未分组
	folderID, ok := h.resolveFolderID(c, req.FolderID)
	if !ok {
		return
	}

	svc := &model.Service{
		Name:          req.Name,
		Description:   req.Description,
		URL:           req.URL,
		Type:          req.Type,
		Interval:      req.Interval,
		SortOrder:     req.SortOrder,
		HTTPMethod:    method,
		HTTPHeaders:   strings.TrimSpace(req.HTTPHeaders),
		HTTPBody:      req.HTTPBody,
		ExpectStatus:  strings.TrimSpace(req.ExpectStatus),
		ExpectKeyword: strings.TrimSpace(req.ExpectKeyword),
		FolderID:      folderID,
	}
	if req.ShowOnHomepage != nil {
		svc.ShowOnHomepage = *req.ShowOnHomepage
	}
	if req.InsecureSkipVerify != nil {
		svc.InsecureSkipVerify = *req.InsecureSkipVerify
	}
	if req.TimeoutSeconds != nil {
		svc.TimeoutSeconds = *req.TimeoutSeconds
	}
	// 首页展示内容：省略 / 空串按「全部展示」处理（与历史数据语义一致）
	if req.HomepageBlocks != nil {
		svc.HomepageBlocks = model.NormalizeHomepageBlocks(*req.HomepageBlocks)
	}

	if err := h.Repo.CreateService(c.Request.Context(), svc); err != nil {
		utils.Error("create service failed: %v", err)
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
		utils.Error("failed to auto-create probe task for service %d: %v", svc.ID, err)
	}

	auditLog("service.create", fmt.Sprintf("name=%s url=%s", svc.Name, svc.URL))
	h.InvalidateSummary()

	c.JSON(http.StatusCreated, model.APIResponse{
		Code:    201,
		Message: "Service created",
		Data:    svc.MaskServiceSecrets(),
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

	if req.TimeoutSeconds != nil {
		if msg := validateServiceTimeout(*req.TimeoutSeconds); msg != "" {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": msg})
			return
		}
	}

	// 校验 HTTP 探测高级匹配字段。未传（nil）表示保持原值，传空串表示清空，
	// 因此这里只校验真正传上来的值。
	if req.HTTPMethod != nil {
		if _, ok := model.NormalizeHTTPMethod(*req.HTTPMethod); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求方法无效，支持 GET/HEAD/POST/PUT/PATCH/DELETE/OPTIONS"})
			return
		}
	}
	if req.ExpectStatus != nil {
		if _, ok := model.ParseExpectStatus(*req.ExpectStatus); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "期望状态码格式无效，支持 200 / 200,301 / 200-299 / 2xx 组合，用逗号分隔"})
			return
		}
	}
	if msg := validateExpectKeywordPtr(req.ExpectKeyword); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": msg})
		return
	}
	// 请求头 / 请求体属于敏感信息：管理端不会回显原值，因此「留空」表示保持原值。
	// 想清空已保存的配置时，传空串以外的显式值（目前通过重新填写实现）。
	if req.HTTPHeaders != nil && strings.TrimSpace(*req.HTTPHeaders) != "" {
		if _, err := model.ParseCustomHeaders(*req.HTTPHeaders); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
			return
		}
		svc.HTTPHeaders = strings.TrimSpace(*req.HTTPHeaders)
	}
	if req.HTTPBody != nil && *req.HTTPBody != "" {
		svc.HTTPBody = *req.HTTPBody
	}
	if req.HTTPMethod != nil {
		svc.HTTPMethod, _ = model.NormalizeHTTPMethod(*req.HTTPMethod)
	}
	if req.ExpectStatus != nil {
		svc.ExpectStatus = strings.TrimSpace(*req.ExpectStatus)
	}
	if req.ExpectKeyword != nil {
		svc.ExpectKeyword = strings.TrimSpace(*req.ExpectKeyword)
	}
	// 分组归属：不传保持原值，传 0 / null 表示取消分组
	if req.FolderID != nil {
		folderID, ok := h.resolveFolderID(c, req.FolderID)
		if !ok {
			return
		}
		svc.FolderID = folderID
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
	if req.InsecureSkipVerify != nil {
		svc.InsecureSkipVerify = *req.InsecureSkipVerify
	}
	if req.TimeoutSeconds != nil {
		svc.TimeoutSeconds = *req.TimeoutSeconds
	}
	// 首页展示内容：传空串表示「全部不展示」，落库为 none 标记
	if req.HomepageBlocks != nil {
		svc.HomepageBlocks = model.NormalizeHomepageBlocks(*req.HomepageBlocks)
	}
	svc.SortOrder = req.SortOrder

	if err := h.Repo.UpdateService(c.Request.Context(), svc); err != nil {
		utils.Error("update service %d failed: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to update service"})
		return
	}

	auditLog("service.update", fmt.Sprintf("id=%d name=%s", svc.ID, svc.Name))
	h.InvalidateSummary()

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "Service updated",
		Data:    svc.MaskServiceSecrets(),
	})
}

// resolveFolderID 校验并归一化服务分组归属。
// 返回 nil 表示未分组；第二个返回值为 false 时表示分组不存在，响应已写出。
func (h *Handler) resolveFolderID(c *gin.Context, raw *int64) (*int64, bool) {
	if raw == nil || *raw <= 0 {
		return nil, true
	}
	if _, err := h.Repo.GetServiceFolder(c.Request.Context(), *raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Service folder not found"})
		return nil, false
	}
	folderID := *raw
	return &folderID, true
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
		utils.Error("delete service %d failed: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to delete service"})
		return
	}

	if svc != nil {
		auditLog("service.delete", fmt.Sprintf("id=%d name=%s", id, svc.Name))
	}
	h.InvalidateSummary()

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
			utils.Error("reorder services failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to reorder services"})
			return
		}
	}

	auditLog("service.reorder", fmt.Sprintf("count=%d", len(req.Services)))
	h.InvalidateSummary()

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "Services reordered",
	})
}

// AdminListServices 获取所有服务列表（管理员用，含详细信息和在线率）
func (h *Handler) AdminListServices(c *gin.Context) {
	services, err := h.Repo.ListServices(c.Request.Context())
	if err != nil {
		utils.Warn("list services failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to fetch services"})
		return
	}

	type ServiceDetail struct {
		model.Service
		Uptime  float64 `json:"uptime"`
		Latency int     `json:"latency"`
	}

	svcIDs := make([]int64, len(services))
	for i, svc := range services {
		svcIDs[i] = svc.ID
	}
	uptimeMap := h.batchCalcUptime(c, svcIDs, 90)

	// Batch load latest heartbeat for all services（避免按服务逐个查询）
	latencyMap := make(map[int64]int, len(services))
	if hbMap, err := h.Repo.BatchGetLatestHeartbeats(c.Request.Context(), svcIDs); err == nil {
		for id, hb := range hbMap {
			latencyMap[id] = hb.Latency
		}
	}

	result := make([]ServiceDetail, 0, len(services))
	for _, svc := range services {
		result = append(result, ServiceDetail{
			// 探测密钥不回显：前端只用于表单回填，留空即保持原值
			Service: *svc.MaskServiceSecrets(),
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
