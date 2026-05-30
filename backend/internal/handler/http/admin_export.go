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

	settings := utils.GetAllSettings()

	data := model.ExportData{
		Version:         1,
		ExportedAt:      time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		Services:        services,
		Incidents:       incidents,
		IncidentUpdates: allUpdates,
		Maintenances:    maintenances,
		Settings:        settings,
	}

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
	}

	if err := h.Repo.ImportFullData(ctx, importData); err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: "导入失败: " + err.Error()})
		return
	}

	// Import settings
	for key, value := range data.Settings {
		_ = utils.SetSetting(key, value)
	}

	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "导入成功"})
}
