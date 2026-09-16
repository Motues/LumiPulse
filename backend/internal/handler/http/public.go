package http

import (
	"lumipluse-backend/internal/config"
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

// buildServiceSummaries 统一组装 ServiceSummary。
// 该结构原先在三处分别拼装，导致公开接口漏填 Latency / Interval / Type
// （详情页显示「0ms」「0s」）。集中在这里可以避免再次跑偏。
func (h *Handler) buildServiceSummaries(c *gin.Context, services []*model.Service, statusOf func(int64) string) []model.ServiceSummary {
	svcIDs := make([]int64, len(services))
	for i, svc := range services {
		svcIDs[i] = svc.ID
	}
	uptimeMap := h.batchCalcUptime(c, svcIDs, 90)
	// 批量取各服务的最新心跳，用于展示当前响应时间
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
			Name:       svc.Name,
			Status:     statusOf(svc.ID),
			URL:        svc.URL,
			Type:       svc.Type,
			Uptime:     uptimeMap[svc.ID],
			Latency:    latency,
			Interval:   svc.Interval,
		})
	}
	return summaries
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
	services = visibleServices(services)

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

	// Build service summaries
	serviceSummaries := h.buildServiceSummaries(c, services, func(id int64) string {
		return reconcileStatus(activeIncidents, id)
	})

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
	services = visibleServices(services)

	activeIncidents, err := h.Repo.ListActiveIncidents(c.Request.Context())
	if err != nil {
		activeIncidents = nil
	}

	summaries := h.buildServiceSummaries(c, services, func(id int64) string {
		return reconcileStatus(activeIncidents, id)
	})

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data:    summaries,
	})
}

// resolveServiceByHash 通过公开 hash 定位服务。
// 公开接口一律用 hash 定位，避免自增 ID 出现在 URL 中；失败时已写出 404 响应。
func (h *Handler) resolveServiceByHash(c *gin.Context) (*model.Service, bool) {
	hash := strings.TrimSpace(c.Param("hash"))
	if hash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid service hash"})
		return nil, false
	}

	svc, err := h.Repo.GetServiceByHash(c.Request.Context(), hash)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Service not found"})
		return nil, false
	}
	return svc, true
}

// GetServiceHistory 获取特定服务的历史可用性数据
func (h *Handler) GetServiceHistory(c *gin.Context) {
	svc, ok := h.resolveServiceByHash(c)
	if !ok {
		return
	}
	id := svc.ID

	maxDays := config.GlobalConfig.HeartbeatRetention()
	days := 90
	if d := c.Query("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
			days = parsed
		}
	}
	if days > maxDays {
		days = maxDays
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

// aggregateLatency 将心跳按固定时间窗聚合为紧凑的分桶响应。
// 聚合下沉到 SQLite 完成（GROUP BY），不再把窗口内的原始心跳全部读进内存。
func (h *Handler) aggregateLatency(c *gin.Context, serviceID int64, days int) model.LatencyResponse {
	const bucketSize = 5                          // 5分钟
	totalBuckets := (24 * 60 / bucketSize) * days // 288 per day

	now := time.Now().UTC()
	startTime := now.AddDate(0, 0, -days)
	startTime = startTime.Truncate(time.Duration(bucketSize) * time.Minute)

	latencies := make([]int, totalBuckets)
	statuses := make([]int, totalBuckets)
	for i := range statuses {
		statuses[i] = -1
	}

	// SQLite 按 UTC 解析不带时区的时间串，因此这里统一用 UTC 墙上时间传参
	buckets, err := h.Repo.GetLatencyBuckets(c.Request.Context(), serviceID,
		startTime.Format("2006-01-02 15:04:05"), bucketSize*60, totalBuckets)
	if err == nil {
		for _, b := range buckets {
			if b.Bucket < 0 || b.Bucket >= totalBuckets || b.Total == 0 {
				continue
			}
			latencies[b.Bucket] = int(b.AvgLatency)
			if b.Failures > 0 {
				statuses[b.Bucket] = 1
			} else {
				statuses[b.Bucket] = 0
			}
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
	svc, ok := h.resolveServiceByHash(c)
	if !ok {
		return
	}

	// 上限与心跳保留天数对齐，避免请求到已被清理的时间范围却看不出原因
	maxDays := config.GlobalConfig.HeartbeatRetention()
	days := 1
	if d := c.Query("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
			days = parsed
		}
	}
	if days > maxDays {
		days = maxDays
	}

	data := h.aggregateLatency(c, svc.ID, days)

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data:    data,
	})
}

// GetPublicIncident 获取单个故障事件详情（含更新时间线）
// 公开访问使用随机 hash 而非自增 ID，避免暴露数据库内部标识。
func (h *Handler) GetPublicIncident(c *gin.Context) {
	hash := strings.TrimSpace(c.Param("hash"))
	if hash == "" {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: "Invalid incident hash"})
		return
	}

	inc, err := h.Repo.GetIncidentByHash(c.Request.Context(), hash)
	if err != nil {
		c.JSON(http.StatusNotFound, model.APIResponse{Code: 404, Message: "Incident not found"})
		return
	}

	updates, err := h.Repo.ListIncidentUpdates(c.Request.Context(), inc.ID)
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
			"site_name":                utils.GetSetting("site_name"),
			"site_icon":                utils.GetSetting("site_icon"),
			"email_enabled":            emailEnabled,
			"show_admin_footer_button": utils.GetSetting("show_admin_footer_button"),
			"custom_footer":            utils.GetSetting("custom_footer"),
			"sub_enable_email":         utils.GetSetting("sub_enable_email"),
			"sub_enable_rss":           utils.GetSetting("sub_enable_rss"),
			"sub_enable_atom":          utils.GetSetting("sub_enable_atom"),
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

// buildDailyPairs 把每日汇总 + 事件状态整理成 [upCount, downCount, statusCode] 三元组数组。
// 供单个服务接口与批量接口共用，保证两者输出完全一致。
func buildDailyPairs(dailies []*model.ServiceDaily, incidents []*model.Incident, days int) [][3]int {
	dailyMap := make(map[string]*model.ServiceDaily, len(dailies))
	for _, d := range dailies {
		dailyMap[d.Date] = d
	}

	now := time.Now()
	pairs := make([][3]int, 0, days)
	for i := days - 1; i >= 0; i-- {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		if d, ok := dailyMap[date]; ok && (d.UptimeCount > 0 || d.DowntimeCount > 0) {
			pairs = append(pairs, [3]int{d.UptimeCount, d.DowntimeCount, incidentStatusForDate(incidents, date)})
		} else {
			pairs = append(pairs, [3]int{-1, -1, -1})
		}
	}
	return pairs
}

// visibleServices 返回首页可见的服务
func visibleServices(services []*model.Service) []*model.Service {
	visible := make([]*model.Service, 0, len(services))
	for _, svc := range services {
		if svc.ShowOnHomepage {
			visible = append(visible, svc)
		}
	}
	return visible
}

// GetDailyStats 获取单个服务 90 天每日故障/在线时间统计，每个元素为 [upCount, downCount, statusCode]
func (h *Handler) GetDailyStats(c *gin.Context) {
	svc, ok := h.resolveServiceByHash(c)
	if !ok {
		return
	}
	id := svc.ID

	// 上限与每日汇总保留天数对齐
	maxDays := config.GlobalConfig.DailyRetention()
	days := 90
	if d := c.Query("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
			days = parsed
		}
	}
	if days > maxDays {
		days = maxDays
	}

	dailies, err := h.Repo.GetServiceDailies(c.Request.Context(), id, days)
	if err != nil {
		dailies = []*model.ServiceDaily{}
	}

	// Load incidents for this service to determine per-day status
	incidents, _ := h.Repo.ListServiceIncidents(c.Request.Context(), id, days)

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data: gin.H{
			"serviceId":  id,
			"publicHash": svc.PublicHash,
			"days":       buildDailyPairs(dailies, incidents, days),
		},
	})
}

// clampDailyDays 解析并夹紧每日统计的 days 参数
func clampDailyDays(raw string, def int) int {
	maxDays := config.GlobalConfig.DailyRetention()
	days := def
	if raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			days = parsed
		}
	}
	if days > maxDays {
		days = maxDays
	}
	if days < 1 {
		days = 1
	}
	return days
}

// batchDailyStats 一次性汇总多个服务的每日统计。
// onlyVisible=true 时只统计首页可见服务（公开接口），否则返回全部（管理接口）。
func (h *Handler) batchDailyStats(c *gin.Context, days int, onlyVisible bool) ([]model.ServiceDailyStats, error) {
	services, err := h.Repo.ListServices(c.Request.Context())
	if err != nil {
		return nil, err
	}
	if onlyVisible {
		services = visibleServices(services)
	}

	svcIDs := make([]int64, len(services))
	for i, svc := range services {
		svcIDs[i] = svc.ID
	}

	dailiesMap, err := h.Repo.BatchGetServiceDailies(c.Request.Context(), svcIDs, days)
	if err != nil {
		dailiesMap = map[int64][]*model.ServiceDaily{}
	}

	// 一次取回窗口内所有事件，再按服务分组（原先按服务逐个查询）
	incidents, _ := h.Repo.ListIncidentsSince(c.Request.Context(), days)
	incidentsByService := make(map[int64][]*model.Incident)
	for _, inc := range incidents {
		incidentsByService[inc.ServiceID] = append(incidentsByService[inc.ServiceID], inc)
	}

	stats := make([]model.ServiceDailyStats, 0, len(svcIDs))
	for _, svc := range services {
		stats = append(stats, model.ServiceDailyStats{
			ServiceID:  svc.ID,
			PublicHash: svc.PublicHash,
			Days:       buildDailyPairs(dailiesMap[svc.ID], incidentsByService[svc.ID], days),
		})
	}
	return stats, nil
}

// GetBatchDailyStats 一次性返回所有首页可见服务的每日统计。
// 首页需要为每个服务渲染状态矩阵，逐个服务请求会产生 N 次串行往返（N+1）。
func (h *Handler) GetBatchDailyStats(c *gin.Context) {
	days := clampDailyDays(c.Query("days"), 90)

	stats, err := h.batchDailyStats(c, days, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to fetch services"})
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data: gin.H{
			"days":     days,
			"services": stats,
		},
	})
}

// AdminBatchDailyStats 管理端批量每日统计（含未在首页展示的服务）
func (h *Handler) AdminBatchDailyStats(c *gin.Context) {
	days := clampDailyDays(c.Query("days"), 90)

	stats, err := h.batchDailyStats(c, days, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to fetch services"})
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data: gin.H{
			"days":     days,
			"services": stats,
		},
	})
}
