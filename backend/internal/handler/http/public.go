package http

import (
	"context"
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
			ID:             svc.ID,
			PublicHash:     svc.PublicHash,
			FolderID:       svc.FolderID,
			Name:           svc.Name,
			Status:         statusOf(svc.ID),
			URL:            svc.URL,
			Type:           svc.Type,
			Uptime:         uptimeMap[svc.ID],
			Latency:        latency,
			Interval:       svc.Interval,
			CertExpiresAt:  svc.CertExpiresAt,
			HomepageBlocks: svc.HomepageBlocks,
		})
	}
	return summaries
}

// buildFolderSummaries 把服务按分组聚合成公开首页需要的融合条目。
//
// 融合口径（与单个服务一致，避免两处跑偏）：
//   - 可用率按探测次数加权（用每日汇总的 up/down 计数），而不是各服务可用率的简单平均；
//   - 响应时间只统计成功样本并同样按次数加权，否则慢服务会被探测少的服务带偏；
//   - 状态取最严重：全部正常 → operational，全部故障 → outage，其余 → degraded。
//
// folderIDs 为 nil 表示不过滤分组（管理端用）；给定集合时只返回其中的分组（公开首页只给可见分组）。
func (h *Handler) buildFolderSummaries(c *gin.Context, services []*model.Service, summaries []model.ServiceSummary, folderIDs map[int64]bool) []model.FolderSummary {
	if len(services) == 0 {
		return []model.FolderSummary{}
	}

	svcIDs := make([]int64, len(services))
	for i, svc := range services {
		svcIDs[i] = svc.ID
	}
	// 融合可用率/延迟需要探测次数与成功样本，直接从每日汇总取
	dailiesMap, err := h.Repo.BatchGetServiceDailies(c.Request.Context(), svcIDs, 90)
	if err != nil {
		dailiesMap = map[int64][]*model.ServiceDaily{}
	}

	summaryByService := make(map[int64]model.ServiceSummary, len(summaries))
	for _, s := range summaries {
		summaryByService[s.ID] = s
	}

	type bucket struct {
		folderID int64
		services []model.ServiceSummary
		up       int
		down     int
		latency  int
		latencyN int
	}
	buckets := make(map[int64]*bucket)
	order := make([]int64, 0)

	for _, svc := range services {
		if svc.FolderID == nil || *svc.FolderID <= 0 {
			continue
		}
		fid := *svc.FolderID
		if folderIDs != nil && !folderIDs[fid] {
			continue
		}
		summary, ok := summaryByService[svc.ID]
		if !ok {
			continue
		}
		b, exists := buckets[fid]
		if !exists {
			b = &bucket{folderID: fid}
			buckets[fid] = b
			order = append(order, fid)
		}
		b.services = append(b.services, summary)
		for _, d := range dailiesMap[svc.ID] {
			b.up += d.UptimeCount
			b.down += d.DowntimeCount
			b.latency += d.TotalLatency
			b.latencyN += d.UptimeCount
		}
	}

	if len(order) == 0 {
		return []model.FolderSummary{}
	}

	folders, err := h.Repo.ListServiceFolders(c.Request.Context())
	if err != nil {
		folders = nil
	}
	folderByID := make(map[int64]*model.ServiceFolder, len(folders))
	for _, f := range folders {
		folderByID[f.ID] = f
	}

	result := make([]model.FolderSummary, 0, len(order))
	for _, fid := range order {
		b := buckets[fid]
		meta := folderByID[fid]
		if meta == nil {
			// 分组已被删除但服务还挂着旧 ID（理论上不会发生，这里兜底跳过）
			continue
		}

		uptime := 100.0
		if total := b.up + b.down; total > 0 {
			uptime = float64(b.up) / float64(total) * 100
		}
		latency := 0
		if b.latencyN > 0 {
			latency = b.latency / b.latencyN
		}

		upCount, outageCount := 0, 0
		for _, s := range b.services {
			switch s.Status {
			case "operational":
				upCount++
			case "outage":
				outageCount++
			}
		}
		status := "degraded"
		if outageCount == len(b.services) {
			status = "outage"
		} else if upCount == len(b.services) {
			status = "operational"
		}

		result = append(result, model.FolderSummary{
			ID:          fid,
			Name:        meta.Name,
			Description: meta.Description,
			Status:      status,
			Uptime:      uptime,
			Latency:     latency,
			Services:    b.services,
		})
	}
	return result
}

// visibleFolders 返回允许在公开首页展示的分组 ID 集合
func visibleFolders(folders []*model.ServiceFolder) map[int64]bool {
	visible := make(map[int64]bool, len(folders))
	for _, f := range folders {
		if f.ShowOnHomepage {
			visible[f.ID] = true
		}
	}
	return visible
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

// Ready readiness 探针。与 /health（liveness，只证明进程还在）不同，
// 这里会真正探测数据库连通性；数据库不可用时返回 503，编排系统应停止向该实例导流。
func (h *Handler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	if err := h.Repo.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, model.APIResponse{
			Code:    503,
			Message: "database unavailable",
			Data:    gin.H{"status": "unavailable"},
		})
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data: gin.H{
			"status":  "ready",
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

// sanitizeIncidentPostmortem 按事件自己的开关决定是否公开复盘内容。
// 未勾选「对外公开」时清空根因 / 处理措施 / 文档链接，
// 这样公开接口（总览、事件列表、事件详情）都不可能把内部信息带出去。
// 同时清空「确认人」等内部运维信息：公开页面只展示事件本身的状态。
func sanitizeIncidentPostmortem(inc *model.Incident) {
	if inc == nil {
		return
	}
	inc.AcknowledgedBy = ""
	inc.AcknowledgedAt = ""
	if inc.PostmortemPublic {
		return
	}
	inc.RootCause = ""
	inc.Resolution = ""
	inc.PostmortemURL = ""
}

// sanitizeIncidentPostmortems 批量版本
func sanitizeIncidentPostmortems(incidents []*model.Incident) {
	for _, inc := range incidents {
		sanitizeIncidentPostmortem(inc)
	}
}

// GetSummary 获取系统整体健康状况
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

	folders, err := h.Repo.ListServiceFolders(c.Request.Context())
	if err != nil {
		folders = nil
	}

	// Filter to only homepage-visible services（分组可见性会覆盖服务自身的开关）
	services = visibleServices(services, folders)

	activeIncidents, err := h.Repo.ListActiveIncidents(c.Request.Context())
	if err != nil {
		activeIncidents = nil
	}
	// 复盘内容默认不对外，公开前统一过滤
	sanitizeIncidentPostmortems(activeIncidents)

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

	// 服务分组的融合结果（只包含首页可见的分组）
	folderSummaries := h.buildFolderSummaries(c, services, serviceSummaries, visibleFolders(folders))

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

	response := model.SummaryResponse{
		OverallStatus: overall,
		Services:      serviceSummaries,
		Folders:       folderSummaries,
		Incidents:     activeIncidents,
		Maintenances:  activeMaints,
	}
	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data:    response,
	})
	h.setCache(response)
}

// ListServices 获取所有监控服务的当前状态列表
func (h *Handler) ListServices(c *gin.Context) {
	services, err := h.Repo.ListServices(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to fetch services"})
		return
	}

	folders, err := h.Repo.ListServiceFolders(c.Request.Context())
	if err != nil {
		folders = nil
	}

	// Filter to only homepage-visible services（分组可见性会覆盖服务自身的开关）
	services = visibleServices(services, folders)

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

// --- 维护窗口标注 ---
//
// 维护期间的失败探测属于「计划内停机」，与真实故障区分开：延迟分桶用状态 2、
// 热力图格子用 maintenance 标记、每日矩阵用四元组第 4 位表达，
// 前端统一用蓝色而不是红色渲染。

// cstZone 展示口径统一使用北京时间（与热力图 / 每日矩阵一致）
var cstZone = time.FixedZone("CST", 8*3600)

// 延迟分桶状态：0=正常、1=故障、-1=无数据。
// 2 是本项目扩展的「维护窗口内的故障」，前端用蓝色区分计划内停机与真实故障。
const (
	latencyStatusNoData             = -1
	latencyStatusOK                 = 0
	latencyStatusFailure            = 1
	latencyStatusMaintenanceFailure = 2
)

// maintenanceWindow 一个已解析的维护窗口及其受影响服务集合。
type maintenanceWindow struct {
	start      time.Time
	end        time.Time
	serviceIDs map[int64]bool
}

// maintenanceTimeLayouts 维护计划起止时间的解析格式。
// 带时区的写法优先，其余按北京时间墙上时间解析（日期选择器输出的就是这种写法）。
var maintenanceTimeLayouts = []string{
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02 15:04:05Z07:00",
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

// parseMaintenanceTime 解析维护计划的时间。
// 不带时区的写法统一按 CST 解析：心跳存的是 UTC，若把墙上时间当 UTC 处理，
// 维护窗口会整体偏移 8 小时，蓝色标注就会落到错误的时段上。
func parseMaintenanceTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range maintenanceTimeLayouts {
		if strings.Contains(layout, "Z07:00") {
			if t, err := time.Parse(layout, raw); err == nil {
				return t, true
			}
			continue
		}
		if t, err := time.ParseInLocation(layout, raw, cstZone); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// parseMaintenanceWindows 把维护计划解析成窗口列表。
// 已取消的维护不参与标注（那些窗口里的故障是真实故障）；起止颠倒的脏数据直接跳过。
func parseMaintenanceWindows(maintenances []*model.Maintenance) []maintenanceWindow {
	windows := make([]maintenanceWindow, 0, len(maintenances))
	for _, m := range maintenances {
		if m == nil || m.Status == "cancelled" {
			continue
		}
		start, okStart := parseMaintenanceTime(m.ScheduledStart)
		end, okEnd := parseMaintenanceTime(m.ScheduledEnd)
		if !okStart || !okEnd || !end.After(start) {
			continue
		}
		ids := make(map[int64]bool)
		for _, part := range strings.Split(m.AffectedServices, ",") {
			if id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64); err == nil {
				ids[id] = true
			}
		}
		windows = append(windows, maintenanceWindow{start: start, end: end, serviceIDs: ids})
	}
	return windows
}

// windowsForService 过滤出会影响该服务的维护窗口
func windowsForService(windows []maintenanceWindow, svcID int64) []maintenanceWindow {
	result := make([]maintenanceWindow, 0, len(windows))
	for _, w := range windows {
		if w.serviceIDs[svcID] {
			result = append(result, w)
		}
	}
	return result
}

// serviceMaintenanceWindows 读取并过滤出会影响该服务的维护窗口（含已完成的窗口，
// 否则历史数据没法回溯标注）。
func (h *Handler) serviceMaintenanceWindows(c *gin.Context, svcID int64) []maintenanceWindow {
	maintenances, err := h.Repo.ListMaintenances(c.Request.Context())
	if err != nil {
		return nil
	}
	return windowsForService(parseMaintenanceWindows(maintenances), svcID)
}

// overlapsMaintenance 判断时间区间 [start, end) 是否与任一维护窗口相交
func overlapsMaintenance(windows []maintenanceWindow, start, end time.Time) bool {
	for _, w := range windows {
		if start.Before(w.end) && end.After(w.start) {
			return true
		}
	}
	return false
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
		statuses[i] = latencyStatusNoData
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
				statuses[b.Bucket] = latencyStatusFailure
			} else {
				statuses[b.Bucket] = latencyStatusOK
			}
		}
	}

	// 维护窗口内的故障单独标成状态 2：前端用蓝色画这段曲线，
	// 与「真实故障」的红色区分开。
	if windows := h.serviceMaintenanceWindows(c, serviceID); len(windows) > 0 {
		bucketDuration := time.Duration(bucketSize) * time.Minute
		for i := range statuses {
			if statuses[i] != latencyStatusFailure {
				continue
			}
			begin := startTime.Add(time.Duration(i) * bucketDuration)
			if overlapsMaintenance(windows, begin, begin.Add(bucketDuration)) {
				statuses[i] = latencyStatusMaintenanceFailure
			}
		}
	}

	// 分位数汇总与分桶聚合共用同一时间窗口，前端可放在同一张卡片里对照
	stats, err := h.Repo.GetLatencyStats(c.Request.Context(), serviceID,
		startTime.Format("2006-01-02 15:04:05"))
	if err != nil {
		stats = nil
	}

	return model.LatencyResponse{
		Start:     startTime.Format("2006-01-02T15:04:05Z"),
		Interval:  bucketSize,
		Latencies: latencies,
		Statuses:  statuses,
		Stats:     stats,
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

// GetServiceLatencyHeatmap 获取服务的响应时间热力图数据（按「日期 × 小时」聚合）。
// 与延迟接口的区别：只返回聚合后的格子，7 天原始心跳（约 2000 个 5 分钟桶）无需下发。
func (h *Handler) GetServiceLatencyHeatmap(c *gin.Context) {
	svc, ok := h.resolveServiceByHash(c)
	if !ok {
		return
	}

	// 上限与心跳保留天数对齐
	maxDays := config.GlobalConfig.HeartbeatRetention()
	days := 7
	if d := c.Query("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
			days = parsed
		}
	}
	if days > maxDays {
		days = maxDays
	}
	if days < 1 {
		days = 1
	}

	// 以北京时间的「今天 00:00」为终点，向前取 days 天（含今天）
	loc := cstZone
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	from := today.AddDate(0, 0, -(days - 1))

	cells, err := h.Repo.GetLatencyHeatmap(c.Request.Context(), svc.ID,
		from.UTC().Format("2006-01-02T15:04:05Z"))
	if err != nil {
		cells = nil
	}
	if cells == nil {
		cells = []*model.LatencyHeatmapCell{}
	}

	// 维护窗口内的小时单独标注：前端把这些小时的失败探测画成蓝色（计划内停机）
	if windows := h.serviceMaintenanceWindows(c, svc.ID); len(windows) > 0 {
		for _, cell := range cells {
			hourStart, err := time.ParseInLocation("2006-01-02", cell.Day, loc)
			if err != nil {
				continue
			}
			hourStart = hourStart.Add(time.Duration(cell.Hour) * time.Hour)
			cell.Maintenance = overlapsMaintenance(windows, hourStart, hourStart.Add(time.Hour))
		}
	}

	maxAvg := 0
	for _, cell := range cells {
		if cell.Avg > maxAvg {
			maxAvg = cell.Avg
		}
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data: model.LatencyHeatmapResponse{
			From:   from.Format("2006-01-02"),
			To:     today.Format("2006-01-02"),
			Days:   days,
			MaxAvg: maxAvg,
			Cells:  cells,
		},
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
	sanitizeIncidentPostmortem(inc)

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
	// 复盘内容默认不对外
	sanitizeIncidentPostmortems(incidents)

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

// buildDailyPairs 把每日汇总 + 事件状态整理成
// [upCount, downCount, statusCode, maintenanceCount] 四元组数组。
// 供单个服务接口与批量接口共用，保证两者输出完全一致。
// maintenanceCount 是维护窗口内的失败探测次数：不计入可用率，
// 前端把它当作「计划内停机」用蓝色展示。
func buildDailyPairs(dailies []*model.ServiceDaily, incidents []*model.Incident, days int) [][4]int {
	dailyMap := make(map[string]*model.ServiceDaily, len(dailies))
	for _, d := range dailies {
		dailyMap[d.Date] = d
	}

	now := time.Now()
	pairs := make([][4]int, 0, days)
	for i := days - 1; i >= 0; i-- {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		if d, ok := dailyMap[date]; ok && (d.UptimeCount > 0 || d.DowntimeCount > 0 || d.MaintenanceCount > 0) {
			pairs = append(pairs, [4]int{d.UptimeCount, d.DowntimeCount, incidentStatusForDate(incidents, date), d.MaintenanceCount})
		} else {
			pairs = append(pairs, [4]int{-1, -1, -1, 0})
		}
	}
	return pairs
}

// visibleServices 返回首页可见的服务。
// 分组的可见性优先：属于「首页可见分组」的服务即便自身关闭了 show_on_homepage
// 也会随分组一起展示（聚合条目需要完整明细）；所属分组不可见或分组已被删除的服务
// 不会展示。未分组的服务仍按自身的 show_on_homepage 判断。
func visibleServices(services []*model.Service, folders []*model.ServiceFolder) []*model.Service {
	visibleFolders := visibleFolders(folders)
	visible := make([]*model.Service, 0, len(services))
	for _, svc := range services {
		if svc.FolderID != nil && *svc.FolderID > 0 {
			if visibleFolders[*svc.FolderID] {
				visible = append(visible, svc)
			}
			continue
		}
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
		folders, err := h.Repo.ListServiceFolders(c.Request.Context())
		if err != nil {
			folders = nil
		}
		services = visibleServices(services, folders)
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
