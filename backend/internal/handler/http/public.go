package http

import (
	"lumipluse-backend/internal/model"
	"lumipluse-backend/internal/pkg/utils"
	"net/http"
	"strconv"
	"strings"
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
		if inc.ServiceID != svcID && !isIncidentAffectsService(inc, svcID) {
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

// isIncidentAffectsService checks if an incident affects a service via affected_services (merged incidents).
func isIncidentAffectsService(inc *model.Incident, svcID int64) bool {
	if inc.AffectedServices == "" {
		return false
	}
	for _, id := range parseAffectedServices(inc.AffectedServices) {
		if id == svcID {
			return true
		}
	}
	return false
}

// batchCalcUptime computes uptime for all given service IDs in a single query.
// Returns map[serviceID]uptimePercentage.
func (h *Handler) batchCalcUptime(c *gin.Context, serviceIDs []int64, days int) map[int64]float64 {
	dailiesMap, err := h.Repo.BatchGetServiceDailies(c.Request.Context(), serviceIDs, days)
	if err != nil {
		result := make(map[int64]float64, len(serviceIDs))
		for _, id := range serviceIDs {
			result[id] = 100.0
		}
		return result
	}

	result := make(map[int64]float64, len(serviceIDs))
	for _, id := range serviceIDs {
		dailies := dailiesMap[id]
		totalUp := 0
		totalDown := 0
		for _, d := range dailies {
			totalUp += d.UptimeCount
			totalDown += d.DowntimeCount
		}
		total := totalUp + totalDown
		if total == 0 {
			result[id] = 100.0
		} else {
			result[id] = float64(totalUp) / float64(total) * 100
		}
	}
	return result
}

// Health 健康检查端点（用于负载均衡和容器编排探针）
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data: gin.H{
			"status":  "healthy",
			"version": h.Version,
		},
	})
}

// Subscribe 公开订阅状态通知（邮箱）
func (h *Handler) Subscribe(c *gin.Context) {
	var req model.SubscribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: "请输入有效的邮箱地址"})
		return
	}

	// Validate email
	if msg := validateEmail(req.Email); msg != "" {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: msg})
		return
	}

	// Convert services to comma-separated string
	servicesStr := ""
	if len(req.Services) > 0 {
		svcStrs := make([]string, len(req.Services))
		for i, sid := range req.Services {
			svcStrs[i] = strconv.FormatInt(sid, 10)
		}
		servicesStr = strings.Join(svcStrs, ",")
	}

	// Check if already subscribed
	if existing, err := h.Repo.GetSubscriberByEmail(c.Request.Context(), req.Email); err == nil && existing != nil {
		// Update services if already subscribed
		if existing.SubscribedServices != servicesStr {
			_ = h.Repo.UpdateSubscriberServices(c.Request.Context(), req.Email, servicesStr)
		}
		c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "该邮箱已订阅"})
		return
	}

	if _, err := h.Repo.CreateSubscriber(c.Request.Context(), req.Email, servicesStr); err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: "订阅失败，请稍后重试"})
		return
	}

	c.JSON(http.StatusCreated, model.APIResponse{Code: 201, Message: "订阅成功"})
}
func (h *Handler) GetSummary(c *gin.Context) {
	if cached := h.getCached(); cached != nil {
		c.JSON(http.StatusOK, model.APIResponse{
			Code: 200, Message: "ok", Data: cached,
		})
		return
	}

	services, err := h.Repo.ListServices(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to fetch services"})
		return
	}

	// Filter to only homepage-visible services
	visibleSvcs := make([]*model.Service, 0, len(services))
	for _, svc := range services {
		if svc.ShowOnHomepage {
			visibleSvcs = append(visibleSvcs, svc)
		}
	}
	services = visibleSvcs

	activeIncidents, err := h.Repo.ListActiveIncidents(c.Request.Context())
	if err != nil {
		activeIncidents = nil
	}

	activeMaints, err := h.Repo.ListActiveMaintenances(c.Request.Context())
	if err != nil {
		activeMaints = nil
	}

	// Batch load incident updates
	if len(activeIncidents) > 0 {
		incIDs := make([]int64, len(activeIncidents))
		for i, inc := range activeIncidents {
			incIDs[i] = inc.ID
		}
		updatesMap, err := h.Repo.BatchListIncidentUpdates(c.Request.Context(), incIDs)
		if err == nil {
			for _, inc := range activeIncidents {
				updates := updatesMap[inc.ID]
				filtered := make([]*model.IncidentUpdate, 0, len(updates))
				for _, u := range updates {
					if !u.IsInternal {
						filtered = append(filtered, u)
					}
				}
				inc.Updates = filtered
			}
		}
	}

	// Batch load 90-day uptime for all services
	svcIDs := make([]int64, len(services))
	for i, svc := range services {
		svcIDs[i] = svc.ID
	}
	uptimeMap := h.batchCalcUptime(c, svcIDs, 90)

	// Build service summaries
	serviceSummaries := make([]model.ServiceSummary, 0, len(services))
	for _, svc := range services {
		serviceSummaries = append(serviceSummaries, model.ServiceSummary{
			ID:     svc.ID,
			Name:   svc.Name,
			Status: reconcileStatus(activeIncidents, svc.ID),
			URL:    svc.URL,
			Uptime: uptimeMap[svc.ID],
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
	h.setCache(model.SummaryResponse{
		OverallStatus: overall,
		Services:      serviceSummaries,
		Incidents:     activeIncidents,
		Maintenances:  activeMaints,
	})
}

// ListServices 获取所有监控服务的当前状态列表
func (h *Handler) ListServices(c *gin.Context) {
	services, err := h.Repo.ListServices(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to fetch services"})
		return
	}

	// Filter to only homepage-visible services
	visibleSvcs := make([]*model.Service, 0, len(services))
	for _, svc := range services {
		if svc.ShowOnHomepage {
			visibleSvcs = append(visibleSvcs, svc)
		}
	}
	services = visibleSvcs

	activeIncidents, err := h.Repo.ListActiveIncidents(c.Request.Context())
	if err != nil {
		activeIncidents = nil
	}

	svcIDs := make([]int64, len(services))
	for i, svc := range services {
		svcIDs[i] = svc.ID
	}
	uptimeMap := h.batchCalcUptime(c, svcIDs, 90)

	summaries := make([]model.ServiceSummary, 0, len(services))
	for _, svc := range services {
		summaries = append(summaries, model.ServiceSummary{
			ID:     svc.ID,
			Name:   svc.Name,
			Status: reconcileStatus(activeIncidents, svc.ID),
			URL:    svc.URL,
			Uptime: uptimeMap[svc.ID],
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

// aggregateLatency 将心跳记录聚合为紧凑的5分钟桶响应
func aggregateLatency(heartbeats []*model.Heartbeat, days int) model.LatencyResponse {
	const bucketSize = 5 // 5分钟
	totalBuckets := (24 * 60 / bucketSize) * days // 288 per day

	now := time.Now().UTC()
	startTime := now.AddDate(0, 0, -days)
	startTime = startTime.Truncate(time.Duration(bucketSize) * time.Minute)

	type bucketData struct {
		totalLatency int64
		count        int
		hasFailure   bool
	}

	buckets := make(map[int]*bucketData)
	for _, h := range heartbeats {
		t, err := time.Parse("2006-01-02T15:04:05Z", h.CreatedAt)
		if err != nil {
			continue
		}
		idx := int(t.Sub(startTime).Minutes()) / bucketSize
		if idx < 0 || idx >= totalBuckets {
			continue
		}
		bd, ok := buckets[idx]
		if !ok {
			bd = &bucketData{}
			buckets[idx] = bd
		}
		bd.totalLatency += int64(h.Latency)
		bd.count++
		if h.Status >= 400 {
			bd.hasFailure = true
		}
	}

	latencies := make([]int, totalBuckets)
	statuses := make([]int, totalBuckets)
	for i := 0; i < totalBuckets; i++ {
		bd, ok := buckets[i]
		if !ok || bd.count == 0 {
			latencies[i] = 0
			statuses[i] = -1
		} else if bd.hasFailure {
			latencies[i] = int(bd.totalLatency / int64(bd.count))
			statuses[i] = 1
		} else {
			latencies[i] = int(bd.totalLatency / int64(bd.count))
			statuses[i] = 0
		}
	}

	return model.LatencyResponse{
		Start:     startTime.Format("2006-01-02T15:04:05Z"),
		Interval:  bucketSize,
		Latencies: latencies,
		Statuses:  statuses,
	}
}

// GetServiceLatency 获取服务5分钟聚合延迟数据
func (h *Handler) GetServiceLatency(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid service id"})
		return
	}

	days := 1
	if d := c.Query("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 && parsed <= 30 {
			days = parsed
		}
	}

	heartbeats, err := h.Repo.GetServiceHistory(c.Request.Context(), id, days)
	if err != nil {
		heartbeats = []*model.Heartbeat{}
	}

	data := aggregateLatency(heartbeats, days)

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data:    data,
	})
}

// GetPublicIncident 获取单个故障事件详情（含更新时间线）
func (h *Handler) GetPublicIncident(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: "Invalid incident id"})
		return
	}

	inc, err := h.Repo.GetIncident(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, model.APIResponse{Code: 404, Message: "Incident not found"})
		return
	}

	updates, err := h.Repo.ListIncidentUpdates(c.Request.Context(), id)
	if err != nil {
		updates = []*model.IncidentUpdate{}
	}
	filtered := make([]*model.IncidentUpdate, 0, len(updates))
	for _, u := range updates {
		if !u.IsInternal {
			filtered = append(filtered, u)
		}
	}
	inc.Updates = filtered

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data:    inc,
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

	// Batch load updates
	if len(incidents) > 0 {
		incIDs := make([]int64, len(incidents))
		for i, inc := range incidents {
			incIDs[i] = inc.ID
		}
		updatesMap, err := h.Repo.BatchListIncidentUpdates(c.Request.Context(), incIDs)
		if err == nil {
			for _, inc := range incidents {
				updates := updatesMap[inc.ID]
				filtered := make([]*model.IncidentUpdate, 0, len(updates))
				for _, u := range updates {
					if !u.IsInternal {
						filtered = append(filtered, u)
					}
				}
				inc.Updates = filtered
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
			"show_admin_footer_button": utils.GetSetting("show_admin_footer_button"),
			"custom_footer": utils.GetSetting("custom_footer"),
			"sub_enable_email": utils.GetSetting("sub_enable_email"),
			"sub_enable_rss": utils.GetSetting("sub_enable_rss"),
			"sub_enable_atom": utils.GetSetting("sub_enable_atom"),
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
