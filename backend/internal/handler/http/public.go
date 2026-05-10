package http

import (
	"lumipluse-backend/internal/model"
	"lumipluse-backend/internal/pkg/utils"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// incidentSeverity returns the impact severity of an incident status on service display.
// identified → outage, investigating → degraded, monitoring/resolved → no impact.
func incidentSeverity(status string) int {
	switch status {
	case "identified":
		return 3
	case "investigating":
		return 2
	default:
		return 0 // monitoring, resolved — show as operational
	}
}

// reconcileStatus adjusts service status based on active incidents.
// The most severe incident determines the service status.
func reconcileStatus(incidents []*model.Incident, svcID int64) string {
	maxSev := 0
	for _, inc := range incidents {
		if inc.ServiceID != svcID {
			continue
		}
		if sev := incidentSeverity(inc.Status); sev > maxSev {
			maxSev = sev
		}
	}

	switch maxSev {
	case 3:
		return "outage"
	case 2:
		return "degraded"
	default:
		return "operational"
	}
}

// GetSummary 获取系统整体健康状况及当前活跃故障
func (h *Handler) GetSummary(c *gin.Context) {
	services, err := h.Repo.ListServices(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to fetch services"})
		return
	}

	activeIncidents, err := h.Repo.ListActiveIncidents(c.Request.Context())
	if err != nil {
		activeIncidents = nil
	}

	activeMaints, err := h.Repo.ListActiveMaintenances(c.Request.Context())
	if err != nil {
		activeMaints = nil
	}

	// Enrich incidents with updates
	for i := range activeIncidents {
		updates, err := h.Repo.ListIncidentUpdates(c.Request.Context(), activeIncidents[i].ID)
		if err == nil {
			activeIncidents[i].Updates = updates
		}
	}

	// Build service summaries with 90-day uptime
	// Reconcile service status with active incidents in case of manually-created incidents
	serviceSummaries := make([]model.ServiceSummary, 0, len(services))
	for _, svc := range services {
		uptime := h.calcUptime(c, svc.ID, 90)
		serviceSummaries = append(serviceSummaries, model.ServiceSummary{
			ID:     svc.ID,
			Name:   svc.Name,
			Status: reconcileStatus(activeIncidents, svc.ID),
			URL:    svc.URL,
			Uptime: uptime,
		})
	}

	// Calculate overall status based on reconciled service status
	overall := "operational"
	hasDegraded := false
	hasOutage := false
	for _, s := range serviceSummaries {
		if s.Status == "outage" {
			hasOutage = true
		} else if s.Status == "degraded" {
			hasDegraded = true
		}
	}
	if hasOutage {
		overall = "outage"
	} else if hasDegraded {
		overall = "degraded"
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data: model.SummaryResponse{
			OverallStatus: overall,
			Services:      serviceSummaries,
			Incidents:     activeIncidents,
			Maintenances:  activeMaints,
		},
	})
}

// ListServices 获取所有监控服务的当前状态列表
func (h *Handler) ListServices(c *gin.Context) {
	services, err := h.Repo.ListServices(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to fetch services"})
		return
	}

	activeIncidents, err := h.Repo.ListActiveIncidents(c.Request.Context())
	if err != nil {
		activeIncidents = nil
	}

	summaries := make([]model.ServiceSummary, 0, len(services))
	for _, svc := range services {
		uptime := h.calcUptime(c, svc.ID, 90)
		summaries = append(summaries, model.ServiceSummary{
			ID:     svc.ID,
			Name:   svc.Name,
			Status: reconcileStatus(activeIncidents, svc.ID),
			URL:    svc.URL,
			Uptime: uptime,
		})
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data:    summaries,
	})
}

// GetServiceHistory 获取特定服务的历史可用性数据
func (h *Handler) GetServiceHistory(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid service id"})
		return
	}

	days := 90
	if d := c.Query("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
			days = parsed
		}
	}

	svc, err := h.Repo.GetService(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Service not found"})
		return
	}

	heartbeats, err := h.Repo.GetServiceHistory(c.Request.Context(), id, days)
	if err != nil {
		heartbeats = []*model.Heartbeat{}
	}

	uptime := h.calcUptime(c, id, days)

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data: model.ServiceHistoryResponse{
			Service:    *svc,
			Uptime:     uptime,
			Heartbeats: heartbeats,
		},
	})
}

// ListIncidents 获取最近的故障事件列表（分页）
func (h *Handler) ListIncidents(c *gin.Context) {
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

	// Enrich with updates
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

// ListMaintenances 获取计划中或进行中的维护任务
func (h *Handler) ListMaintenances(c *gin.Context) {
	maintenances, err := h.Repo.ListActiveMaintenances(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to fetch maintenances"})
		return
	}
	if maintenances == nil {
		maintenances = []*model.Maintenance{}
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data:    maintenances,
	})
}

// GetSiteConfig 获取公开站点配置（无认证）
func (h *Handler) GetSiteConfig(c *gin.Context) {
	emailEnabled := utils.GetSetting("email_enabled")
	if emailEnabled == "" {
		emailEnabled = "false"
	}
	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data: gin.H{
			"site_name":     utils.GetSetting("site_name"),
			"site_icon":     utils.GetSetting("site_icon"),
			"email_enabled": emailEnabled,
		},
	})
}

func (h *Handler) calcUptime(c *gin.Context, serviceID int64, days int) float64 {
	dailies, err := h.Repo.GetServiceDailies(c.Request.Context(), serviceID, days)
	if err != nil || len(dailies) == 0 {
		return 100.0
	}

	totalUp := 0
	totalDown := 0
	for _, d := range dailies {
		totalUp += d.UptimeCount
		totalDown += d.DowntimeCount
	}

	total := totalUp + totalDown
	if total == 0 {
		return 100.0
	}
	return float64(totalUp) / float64(total) * 100
}

// incidentStatusForDate determines the status code for a service on a given date.
// Status codes: -1=no data, 0=normal, 1=investigating, 2=identified, 3=monitoring, 4=resolved
func incidentStatusForDate(incidents []*model.Incident, date string) int {
	for _, inc := range incidents {
		incDate := inc.CreatedAt[:10]

		// Incident started after this date — doesn't affect it
		if incDate > date {
			continue
		}

		// If resolved, check whether resolution happened before this date
		if inc.Status == "resolved" {
			resolvedDate := inc.UpdatedAt[:10]
			if resolvedDate < date {
				continue
			}
			return 4
		}

		// Active incident covers this date — use its current status
		switch inc.Status {
		case "investigating":
			return 1
		case "identified":
			return 2
		case "monitoring":
			return 3
		}
	}
	return 0
}

// GetDailyStats 获取服务90天每日故障/在线时间统计，每个元素为 [upCount, downCount, statusCode]
func (h *Handler) GetDailyStats(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid service id"})
		return
	}

	days := 90
	if d := c.Query("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 && parsed <= 365 {
			days = parsed
		}
	}

	dailies, err := h.Repo.GetServiceDailies(c.Request.Context(), id, days)
	if err != nil {
		dailies = []*model.ServiceDaily{}
	}

	// Load incidents for this service to determine per-day status
	incidents, _ := h.Repo.ListServiceIncidents(c.Request.Context(), id, days)

	// Build date -> counts map from DB records
	dailyMap := make(map[string]*model.ServiceDaily)
	for _, d := range dailies {
		dailyMap[d.Date] = d
	}

	// Build array of [uptimeCount, downtimeCount, statusCode] triples
	now := time.Now()
	pairs := make([][3]int, 0, days)
	for i := days - 1; i >= 0; i-- {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		if d, ok := dailyMap[date]; ok && (d.UptimeCount > 0 || d.DowntimeCount > 0) {
			statusCode := incidentStatusForDate(incidents, date)
			pairs = append(pairs, [3]int{d.UptimeCount, d.DowntimeCount, statusCode})
		} else {
			pairs = append(pairs, [3]int{-1, -1, -1})
		}
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data: gin.H{
			"serviceId": id,
			"days":      pairs,
		},
	})
}
