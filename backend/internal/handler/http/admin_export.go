package http

import (
	"lumipluse-backend/internal/model"
	"lumipluse-backend/internal/pkg/utils"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// AdminExport 导出系统数据
func (h *Handler) AdminExport(c *gin.Context) {
	ctx := c.Request.Context()

	services, err := h.Repo.ListServices(ctx)
	if err != nil {
		services = []*model.Service{}
	}
	// 导出文件可能被下载/转发，探测请求头与请求体属于敏感信息，一律不回显；
	// 重新导入后这两个字段为空，需要重新填写。
	for _, svc := range services {
		svc.MaskServiceSecrets()
	}

	incidents, _, err := h.Repo.ListIncidents(ctx, 1, 100000)
	if err != nil {
		incidents = []*model.Incident{}
	}

	// Load updates for all incidents
	allUpdates := []*model.IncidentUpdate{}
	incIDs := make([]int64, len(incidents))
	for i, inc := range incidents {
		incIDs[i] = inc.ID
	}
	if len(incIDs) > 0 {
		if updatesMap, err := h.Repo.BatchListIncidentUpdates(ctx, incIDs); err == nil {
			for _, updates := range updatesMap {
				allUpdates = append(allUpdates, updates...)
			}
		}
	}

	maintenances, err := h.Repo.ListMaintenances(ctx)
	if err != nil {
		maintenances = []*model.Maintenance{}
	}

	folders, err := h.Repo.ListServiceFolders(ctx)
	if err != nil {
		folders = []*model.ServiceFolder{}
	}
	// 分组在导入时会被清空重建、自增 ID 顺延，因此导出文件里不写库内 ID，
	// 而写「导出内引用号」（从 1 起的紧凑序号）：服务的 folder_id 同步改写成引用号，
	// 导入时再按引用号映射到新分组 ID。这样导出的数据可以安全地导入到任意实例，
	// 不会因为新旧自增 ID 相同却指向不同分组而挂错分组。
	folderRef := make(map[int64]int64, len(folders))
	for i, f := range folders {
		ref := int64(i + 1)
		folderRef[f.ID] = ref
		f.ID = ref
	}
	for _, svc := range services {
		if svc.FolderID == nil {
			continue
		}
		if ref, ok := folderRef[*svc.FolderID]; ok {
			refID := ref
			svc.FolderID = &refID
		} else {
			// 脏数据：分组不存在时按未分组导出
			svc.FolderID = nil
		}
	}

	settings := utils.GetAllSettings()

	data := model.ExportData{
		Version:         1,
		ExportedAt:      time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		Services:        services,
		Incidents:       incidents,
		IncidentUpdates: allUpdates,
		Maintenances:    maintenances,
		Settings:        settings,
		ServiceFolders:  folders,
	}

	utils.Info("data exported: %d services, %d incidents, %d maintenances", len(services), len(incidents), len(maintenances))
	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", "attachment; filename=lumipulse-export-"+time.Now().Format("2006-01-02")+".json")
	c.JSON(http.StatusOK, data)
}

// AdminImport 导入系统数据
func (h *Handler) AdminImport(c *gin.Context) {
	// Require explicit confirmation
	if c.Query("confirm") != "true" {
		c.JSON(http.StatusBadRequest, model.APIResponse{
			Code:    400,
			Message: "导入将覆盖现有服务、事件、维护计划和系统设置，此操作不可撤销。请在请求中添加 ?confirm=true 确认。",
		})
		return
	}

	var data model.ExportData
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: "无效的导入文件格式"})
		return
	}

	if data.Version != 1 {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: "不支持的数据版本"})
		return
	}

	ctx := c.Request.Context()

	importData := &model.ImportData{
		Services:        data.Services,
		Incidents:       data.Incidents,
		IncidentUpdates: data.IncidentUpdates,
		Maintenances:    data.Maintenances,
		ServiceFolders:  data.ServiceFolders,
	}

	if err := h.Repo.ImportFullData(ctx, importData); err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: "导入失败: " + err.Error()})
		return
	}

	// Import settings
	for key, value := range data.Settings {
		_ = utils.SetSetting(key, value)
	}

	// 导入会整体替换服务/事件/维护计划，必须失效公开总览缓存
	h.InvalidateSummary()

	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "导入成功"})
}
