package checker

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"lumipluse-backend/internal/config"
	"lumipluse-backend/internal/model"
	"lumipluse-backend/internal/pkg/utils"
	"lumipluse-backend/internal/repository"
)

// parseTime parses a time string in various formats and returns UTC time.
// Strings without timezone info are assumed to be CST (UTC+8) for backward compatibility.
func parseTime(s string) time.Time {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			if t.Location() == time.UTC && !strings.HasSuffix(s, "Z") && !strings.ContainsAny(s, "+-") {
				return t.Add(-8 * time.Hour)
			}
			return t.UTC()
		}
	}
	return time.Now().UTC()
}

func inMaintenance(maintenances []*model.Maintenance) bool {
	return len(maintenances) > 0
}

// probeTaskState 运行时探测状态（不持久化）
type probeTaskState struct {
	consecutiveFailures  int
	consecutiveSuccesses int
	autoIncidentID       int64     // 0 = no auto-incident
	nextDue              time.Time // 下次到期探测时间（按服务自身 interval 计算）
}

// 调度与并发参数
const (
	// schedulerTick 调度器心跳。取 5s 以便能表达最短 10s 的探测间隔。
	schedulerTick = 5 * time.Second
	// maxProbeConcurrency 单轮探测的最大并发数（有界 worker pool）
	maxProbeConcurrency = 8
)

// probeTarget 一次待执行的探测
type probeTarget struct {
	svc  *model.Service
	task *model.ProbeTask
}

type HealthChecker struct {
	repo     repository.Repository
	interval time.Duration
	// globalInsecure 全局跳过 TLS 校验（配置文件开关，作用范围大，默认关闭）
	globalInsecure bool
	secureClient   *http.Client
	insecureClient *http.Client
	states         map[int64]*probeTaskState // key = serviceID
	mu             sync.Mutex
	stop           chan struct{}
	wg             sync.WaitGroup
	// onDataChange 数据变化回调（用于失效公开页缓存）
	onDataChange func()
}

func newHTTPClient(insecure bool) *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
}

func New(repo repository.Repository, insecureSkipVerify bool) *HealthChecker {
	return &HealthChecker{
		repo:           repo,
		interval:       schedulerTick,
		globalInsecure: insecureSkipVerify,
		secureClient:   newHTTPClient(false),
		insecureClient: newHTTPClient(true),
		states:         make(map[int64]*probeTaskState),
		stop:           make(chan struct{}),
	}
}

// SetOnDataChange 注册数据变化回调。检查器会自动创建/解决事件并改动服务状态，
// 这些变更同样需要让公开页缓存失效。
func (hc *HealthChecker) SetOnDataChange(fn func()) {
	hc.onDataChange = fn
}

func (hc *HealthChecker) dataChanged() {
	if hc.onDataChange != nil {
		hc.onDataChange()
	}
}

// clientFor 根据服务配置选择 HTTP 客户端：只有显式开启（服务级或全局）才跳过校验。
func (hc *HealthChecker) clientFor(svc *model.Service) *http.Client {
	if hc.globalInsecure || svc.InsecureSkipVerify {
		return hc.insecureClient
	}
	return hc.secureClient
}

// probeInterval 返回服务的探测间隔（秒）。校验范围是 10~3600 秒，
// 这里再做一次兜底，避免脏数据导致过于频繁或永不探测。
func probeInterval(svc *model.Service) time.Duration {
	sec := svc.Interval
	if sec < 10 {
		sec = 60
	}
	if sec > 3600 {
		sec = 3600
	}
	return time.Duration(sec) * time.Second
}

func (hc *HealthChecker) Start(ctx context.Context) {
	hc.wg.Add(1)
	go hc.loop(ctx)
	utils.Info("checker started (scheduler tick: %s)", schedulerTick)
}

func (hc *HealthChecker) Stop() {
	close(hc.stop)
	hc.wg.Wait()
	utils.Info("checker stopped")
}

func (hc *HealthChecker) loop(ctx context.Context) {
	defer hc.wg.Done()

	hc.cleanup(ctx)
	lastCleanup := time.Now()
	lastTokenGC := time.Now()

	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	hc.checkAll(ctx)
	hc.reconcileMaintenanceStatus(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-hc.stop:
			return
		case <-ticker.C:
			hc.checkAll(ctx)
			hc.reconcileMaintenanceStatus(ctx)

			now := time.Now()
			if now.Sub(lastCleanup) > 24*time.Hour {
				hc.cleanup(ctx)
				lastCleanup = now
			}
			if now.Sub(lastTokenGC) > 10*time.Minute {
				if n := utils.CleanupExpiredTokens(); n > 0 {
					utils.Info("cleaned up %d expired admin sessions", n)
				}
				lastTokenGC = now
			}
		}
	}
}

func (hc *HealthChecker) checkAll(ctx context.Context) {
	services, err := hc.repo.ListServices(ctx)
	if err != nil {
		utils.Info("checker list services error: %v", err)
		return
	}

	// Load probe tasks and build serviceID -> probeTask map
	probeTasks, err := hc.repo.ListProbeTasks(ctx)
	if err != nil {
		utils.Info("checker list probe tasks error: %v", err)
		return
	}
	taskMap := make(map[int64]*model.ProbeTask, len(probeTasks))
	for _, t := range probeTasks {
		taskMap[t.ServiceID] = t
	}

	// Build set of service IDs under active maintenance
	inMaint := make(map[int64]bool)
	if maints, err := hc.repo.ListMaintenances(ctx); err == nil {
		for _, m := range maints {
			if m.Status == "in_progress" && m.AffectedServices != "" {
				for _, idStr := range strings.Split(m.AffectedServices, ",") {
					idStr = strings.TrimSpace(idStr)
					if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
						inMaint[id] = true
					}
				}
			}
		}
	}

	now := time.Now()
	alive := make(map[int64]bool, len(services))
	dueTargets := make([]probeTarget, 0, len(services))

	for _, svc := range services {
		alive[svc.ID] = true

		if !svc.IsActive {
			continue
		}
		// Check if service has an active probe task
		task, hasTask := taskMap[svc.ID]
		if !hasTask || !task.IsActive {
			continue
		}

		// 按服务自身的 interval 判断是否到期，而不是所有服务共用一个固定周期
		hc.mu.Lock()
		st, exists := hc.states[svc.ID]
		if !exists {
			st = &probeTaskState{}
			hc.states[svc.ID] = st
		}
		isDue := st.nextDue.IsZero() || !now.Before(st.nextDue)
		if isDue {
			st.nextDue = now.Add(probeInterval(svc))
		}
		hc.mu.Unlock()

		if isDue {
			dueTargets = append(dueTargets, probeTarget{svc: svc, task: task})
		}
	}

	// 清理已被删除服务的运行时状态，避免 states 无限增长
	hc.mu.Lock()
	for id := range hc.states {
		if !alive[id] {
			delete(hc.states, id)
		}
	}
	hc.mu.Unlock()

	if len(dueTargets) == 0 {
		return
	}

	// 有界并发探测：串行探测会让一轮耗时随服务数量线性增长，
	// 在多个服务同时故障（各自等满 10s 超时）时监控精度会明显下降。
	sem := make(chan struct{}, maxProbeConcurrency)
	var wg sync.WaitGroup
	for _, t := range dueTargets {
		wg.Add(1)
		sem <- struct{}{}
		go func(t probeTarget) {
			defer wg.Done()
			defer func() { <-sem }()
			hc.probeAndRecord(ctx, t.svc, t.task, inMaint)
		}(t)
	}
	wg.Wait()
}

// probeAndRecord 执行一次探测并写入心跳与每日统计
func (hc *HealthChecker) probeAndRecord(ctx context.Context, svc *model.Service, task *model.ProbeTask, inMaint map[int64]bool) {
	status, latency, message := hc.performCheck(svc)

	// Record heartbeat
	hb := &model.Heartbeat{
		ServiceID: svc.ID,
		Status:    status,
		Latency:   latency,
		Message:   message,
	}
	if err := hc.repo.CreateHeartbeat(ctx, hb); err != nil {
		utils.Info("checker create heartbeat failed for service %d: %v", svc.ID, err)
	}

	isUp := (status >= 200 && status < 400) || status == 1

	// Update daily record
	today := time.Now().Format("2006-01-02")
	daily, err := hc.repo.GetOrCreateServiceDaily(ctx, svc.ID, today)
	if err != nil {
		utils.Info("checker get/create daily failed for service %d: %v", svc.ID, err)
	} else {
		if isUp {
			daily.UptimeCount++
		} else if !inMaint[svc.ID] {
			daily.DowntimeCount++
		}
		daily.TotalLatency += latency
		if err := hc.repo.UpdateServiceDaily(ctx, daily); err != nil {
			utils.Info("checker update daily failed for service %d: %v", svc.ID, err)
		}
	}

	// Auto-incident logic with probe task config
	hc.trackProbeState(ctx, svc, task, isUp)
}

func (hc *HealthChecker) trackProbeState(ctx context.Context, svc *model.Service, task *model.ProbeTask, isUp bool) {
	hc.mu.Lock()
	st, exists := hc.states[svc.ID]
	if !exists {
		st = &probeTaskState{}
		hc.states[svc.ID] = st
	}
	triggerCount := task.TriggerCount

	if isUp {
		st.consecutiveFailures = 0
		st.consecutiveSuccesses++
		shouldResolve := st.consecutiveSuccesses >= 5
		incID := st.autoIncidentID
		hc.mu.Unlock()

		if shouldResolve {
			if incID == 0 {
				if existing, err := hc.repo.GetActiveIncidentByService(ctx, svc.ID); err == nil && existing != nil {
					incID = existing.ID
				}
			}
			if incID != 0 {
				hc.tryResolveIncident(ctx, svc, incID)
			}
		}
	} else {
		st.consecutiveSuccesses = 0
		st.consecutiveFailures++
		failCount := st.consecutiveFailures
		hasAutoIncident := st.autoIncidentID != 0
		hc.mu.Unlock()

		if failCount >= triggerCount && !hasAutoIncident {
			// Don't create incident if service is under active maintenance
			maints, err := hc.repo.ListActiveMaintenancesByService(ctx, svc.ID)
			if err == nil && inMaintenance(maints) {
				hc.mu.Lock()
				st.consecutiveFailures = 0
				hc.mu.Unlock()
				utils.Info("checker skipping incident for service %s (under maintenance)", svc.Name)
				return
			}

			// Don't create a new incident if one already exists for this service
			if existing, err := hc.repo.GetActiveIncidentByService(ctx, svc.ID); err == nil && existing != nil {
				hc.mu.Lock()
				st.autoIncidentID = existing.ID
				hc.mu.Unlock()
				utils.Info("checker linked existing incident #%d for service %s", existing.ID, svc.Name)
				return
			}

			hc.createIncident(ctx, svc, task, st)
		}
	}
}

// tryResolveIncident 尝试解决事件。对于合并事件，只有所有受影响服务都恢复时才解决。
func (hc *HealthChecker) tryResolveIncident(ctx context.Context, svc *model.Service, incID int64) {
	inc, err := hc.repo.GetIncident(ctx, incID)
	if err != nil {
		utils.Info("checker incident #%d not found for resolve: %v", incID, err)
		return
	}

	affectedIDs := parseAffectedServices(inc.AffectedServices)

	// If merged incident with multiple services, check if ALL are recovered
	if len(affectedIDs) > 1 {
		allRecovered := true
		for _, svcID := range affectedIDs {
			hc.mu.Lock()
			st, ok := hc.states[svcID]
			if ok && st.consecutiveSuccesses < 5 {
				allRecovered = false
			}
			hc.mu.Unlock()
			if !allRecovered {
				break
			}
		}
		if !allRecovered {
			return // Not all services recovered yet
		}
	}

	hc.resolveIncident(ctx, svc, inc)
}

func (hc *HealthChecker) resolveIncident(ctx context.Context, svc *model.Service, inc *model.Incident) {
	now := time.Now().Format(time.RFC3339)

	update := &model.IncidentUpdate{
		IncidentID: inc.ID,
		Status:     "resolved",
		Content:    fmt.Sprintf("%s 服务已恢复运行", svc.Name),
	}
	if err := hc.repo.CreateIncidentUpdate(ctx, update); err != nil {
		utils.Info("checker failed to create incident update: %v", err)
	}

	inc.Status = "resolved"
	inc.UpdatedAt = now
	hc.repo.UpdateIncident(ctx, inc)

	// Restore all affected services to operational
	affectedIDs := parseAffectedServices(inc.AffectedServices)
	for _, svcID := range affectedIDs {
		if s, err := hc.repo.GetService(ctx, svcID); err == nil {
			s.Status = "operational"
			hc.repo.UpdateService(ctx, s)
		}
	}

	// Reset states for all affected services
	hc.mu.Lock()
	for _, svcID := range affectedIDs {
		if st, ok := hc.states[svcID]; ok {
			st.autoIncidentID = 0
		}
	}
	hc.mu.Unlock()

	utils.Info("checker resolved incident #%d for service %s", inc.ID, svc.Name)
	hc.dataChanged()
	notifyResolved(svc.Name)
}

func (hc *HealthChecker) createIncident(ctx context.Context, svc *model.Service, task *model.ProbeTask, st *probeTaskState) {
	now := time.Now().Format(time.RFC3339)

	inc := &model.Incident{
		ServiceID:        svc.ID,
		Title:            fmt.Sprintf("%s 服务异常", svc.Name),
		Impact:           "major",
		Status:           "investigating",
		AffectedServices: fmt.Sprintf("%d", svc.ID),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := hc.repo.CreateIncident(ctx, inc); err != nil {
		utils.Info("checker failed to create incident: %v", err)
		return
	}

	update := &model.IncidentUpdate{
		IncidentID: inc.ID,
		Status:     "investigating",
		Content:    fmt.Sprintf("检测到 %s 服务连续异常，正在排查中", svc.Name),
	}
	hc.repo.CreateIncidentUpdate(ctx, update)

	svc.Status = "degraded"
	hc.repo.UpdateService(ctx, svc)

	hc.mu.Lock()
	st.autoIncidentID = inc.ID
	hc.mu.Unlock()

	utils.Info("checker auto-created incident #%d for service %s", inc.ID, svc.Name)
	hc.dataChanged()
	notifyAlert(svc.Name, svc.URL, now)

	// Try auto-merge with same-server incidents (check server's auto_merge setting)
	if task.ServerID != nil {
		if srv, err := hc.repo.GetServer(ctx, *task.ServerID); err == nil && srv.AutoMerge {
			hc.tryAutoMerge(ctx, inc, task)
		}
	}
}

// tryAutoMerge 检查同服务器是否有其他活跃事件，有则自动合并
func (hc *HealthChecker) tryAutoMerge(ctx context.Context, newInc *model.Incident, task *model.ProbeTask) {
	if task.ServerID == nil {
		return
	}

	siblings, err := hc.repo.ListIncidentsByServer(ctx, *task.ServerID)
	if err != nil {
		utils.Info("checker list incidents by server error: %v", err)
		return
	}

	for _, sib := range siblings {
		if sib.ID == newInc.ID || sib.Status == "resolved" {
			continue
		}
		// Merge: keep the earlier incident (sib), merge newInc into it
		hc.mergeIncidents(ctx, sib, newInc)
		return
	}
}

// mergeIncidents 将 source 作为子事件合并到 target
func (hc *HealthChecker) mergeIncidents(ctx context.Context, target, source *model.Incident) {
	// Set source as child of target
	source.ParentID = &target.ID
	hc.repo.UpdateIncident(ctx, source)

	// Merge affected services into target for status tracking
	targetSvcs := parseAffectedServices(target.AffectedServices)
	sourceSvcs := parseAffectedServices(source.AffectedServices)
	merged := appendIfMissing(targetSvcs, sourceSvcs...)
	target.AffectedServices = joinServiceIDs(merged)
	hc.repo.UpdateIncident(ctx, target)

	// Add merge notes
	hc.repo.CreateIncidentUpdate(ctx, &model.IncidentUpdate{
		IncidentID: target.ID,
		Status:     "system",
		Content:    fmt.Sprintf("已合并子事件 #%d（%s）", source.ID, source.Title),
		IsInternal: true,
	})
	hc.repo.CreateIncidentUpdate(ctx, &model.IncidentUpdate{
		IncidentID: source.ID,
		Status:     "system",
		Content:    fmt.Sprintf("已作为子事件合并到 #%d（%s）", target.ID, target.Title),
		IsInternal: true,
	})

	// Update checker states: source services point to target incident
	hc.mu.Lock()
	for _, svcID := range sourceSvcs {
		if st, ok := hc.states[svcID]; ok {
			st.autoIncidentID = target.ID
		}
	}
	hc.mu.Unlock()

	utils.Info("checker merged incident #%d as child of #%d", source.ID, target.ID)
	hc.dataChanged()
}

func (hc *HealthChecker) reconcileMaintenanceStatus(ctx context.Context) {
	maintenances, err := hc.repo.ListMaintenances(ctx)
	if err != nil {
		utils.Info("checker list maintenances error: %v", err)
		return
	}

	now := time.Now().UTC()
	changed := false

	for _, m := range maintenances {
		if m.Status != "scheduled" && m.Status != "in_progress" {
			continue
		}

		start := parseTime(m.ScheduledStart)
		end := parseTime(m.ScheduledEnd)

		if m.Status == "scheduled" && !now.Before(start) {
			m.Status = "in_progress"
			if err := hc.repo.UpdateMaintenance(ctx, m); err != nil {
				utils.Info("checker failed to update maintenance #%d to in_progress: %v", m.ID, err)
			} else {
				utils.Info("checker auto-updated maintenance #%d (%s) to in_progress", m.ID, m.Title)
				changed = true
			}
		} else if m.Status == "in_progress" && !now.Before(end) {
			m.Status = "completed"
			if err := hc.repo.UpdateMaintenance(ctx, m); err != nil {
				utils.Info("checker failed to update maintenance #%d to completed: %v", m.ID, err)
			} else {
				utils.Info("checker auto-updated maintenance #%d (%s) to completed", m.ID, m.Title)
				changed = true
			}
		}
	}

	if changed {
		hc.dataChanged()
	}
}

func notifyAlert(name, url, ts string) {
	raw := utils.GetSetting("notify_services")
	if raw == "" {
		return
	}
	subject := fmt.Sprintf("服务异常告警: %s", name)
	body := fmt.Sprintf("服务 %s (%s) 连续检测失败，已自动创建故障事件。\n\n检测时间: %s", name, url, ts)
	if err := utils.SendAlert(subject, body); err != nil {
		utils.Info("checker send alert failed: %v", err)
	}
}

func notifyResolved(name string) {
	raw := utils.GetSetting("notify_services")
	if raw == "" {
		return
	}
	subject := fmt.Sprintf("服务恢复通知: %s", name)
	body := fmt.Sprintf("服务 %s 已恢复运行。", name)
	if err := utils.SendAlert(subject, body); err != nil {
		utils.Info("checker send alert failed: %v", err)
	}
}

func (hc *HealthChecker) performCheck(svc *model.Service) (int, int, string) {
	switch strings.ToLower(svc.Type) {
	case "http":
		return hc.checkHTTP(svc)
	default:
		return hc.checkTCP(svc)
	}
}

func (hc *HealthChecker) checkHTTP(svc *model.Service) (int, int, string) {
	start := time.Now()
	req, err := http.NewRequest("GET", svc.URL, nil)
	if err != nil {
		return 0, int(time.Since(start).Milliseconds()), err.Error()
	}
	req.Close = true
	req.Header.Set("User-Agent", "LumiPulse")

	resp, err := hc.clientFor(svc).Do(req)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		return 0, latency, err.Error()
	}
	defer resp.Body.Close()
	return resp.StatusCode, latency, ""
}

func (hc *HealthChecker) checkTCP(svc *model.Service) (int, int, string) {
	start := time.Now()
	dialer := net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.Dial("tcp", svc.URL)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		return 0, latency, err.Error()
	}
	conn.Close()
	return 1, latency, ""
}

func (hc *HealthChecker) cleanup(ctx context.Context) {
	dailyDays := config.GlobalConfig.DailyRetention()
	heartbeatDays := config.GlobalConfig.HeartbeatRetention()

	beforeDaily := time.Now().AddDate(0, 0, -dailyDays).Format("2006-01-02")
	if err := hc.repo.DeleteOldServiceDailies(ctx, beforeDaily); err != nil {
		utils.Info("checker cleanup dailies failed: %v", err)
	} else {
		utils.Info("checker cleaned up daily records older than %d days", dailyDays)
	}

	beforeHB := time.Now().AddDate(0, 0, -heartbeatDays).UTC().Format("2006-01-02T15:04:05Z")
	if err := hc.repo.DeleteOldHeartbeats(ctx, beforeHB); err != nil {
		utils.Info("checker cleanup heartbeats failed: %v", err)
	} else {
		utils.Info("checker cleaned up heartbeats older than 7 days")
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

func joinServiceIDs(ids []int64) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	return strings.Join(parts, ",")
}
