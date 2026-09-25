package checker

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"lumipluse-backend/internal/config"
	"lumipluse-backend/internal/model"
	"lumipluse-backend/internal/pkg/i18n"
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
	// lastAlertAt 上次发送异常告警的时间，用于冷却窗口去重（见 alert_policy.go）
	lastAlertAt time.Time
}

// 调度与并发参数
const (
	// schedulerTick 调度器心跳。取 5s 以便能表达最短 10s 的探测间隔。
	schedulerTick = 5 * time.Second
	// defaultProbeTimeoutSeconds 未配置 timeout_seconds 时的单次探测超时
	defaultProbeTimeoutSeconds = 10
	// maxProbeTimeoutSeconds 单次探测超时的上限，避免脏数据让探测永不结束
	maxProbeTimeoutSeconds = 300
)

// probeResult 一次探测的结果
type probeResult struct {
	status  int
	latency int
	message string
	// isUp 本次探测是否判定成功。判定口径由 model.Service 统一给出
	// （期望状态码 + 期望关键字），TCP 探测为连通即成功。
	isUp bool
	// certExpiresAt 仅 HTTPS 探测成功拿到证书时非零值
	certExpiresAt time.Time
}

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
	// probeConcurrency 单轮探测的最大并发数（来自配置文件的 MAX_PROBE_CONCURRENCY）
	probeConcurrency int
	secureClient     *http.Client
	insecureClient   *http.Client
	states           map[int64]*probeTaskState // key = serviceID
	mu               sync.Mutex
	stop             chan struct{}
	wg               sync.WaitGroup
	// onDataChange 数据变化回调（用于失效公开页缓存）
	onDataChange func()
	// onDailyMaintenance 每日任务回调（归档月报后触发，用于发送月报邮件）。
	// 注入而不是直接依赖 handler 包，避免 checker ↔ handler 循环依赖。
	onDailyMaintenance func(context.Context, time.Time)
}

// newHTTPClient 构造探测用客户端。
// 单次请求超时统一由 context 控制（每个服务的 timeout_seconds 不同），
// 因此这里不设 Client.Timeout，否则会与按服务的超时相互干扰。
func newHTTPClient(insecure bool) *http.Client {
	return &http.Client{
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

func New(repo repository.Repository, insecureSkipVerify bool, probeConcurrency int) *HealthChecker {
	if probeConcurrency <= 0 {
		probeConcurrency = 8
	}
	return &HealthChecker{
		repo:             repo,
		interval:         schedulerTick,
		globalInsecure:   insecureSkipVerify,
		probeConcurrency: probeConcurrency,
		secureClient:     newHTTPClient(false),
		insecureClient:   newHTTPClient(true),
		states:           make(map[int64]*probeTaskState),
		stop:             make(chan struct{}),
	}
}

// SetOnDataChange 注册数据变化回调。检查器会自动创建/解决事件并改动服务状态，
// 这些变更同样需要让公开页缓存失效。
func (hc *HealthChecker) SetOnDataChange(fn func()) {
	hc.onDataChange = fn
}

// SetOnDailyMaintenance 注册每日任务回调。检查器在归档月报之后调用它，
// 用于发送月度 SLA 报告邮件等「每天最多一次」的收尾工作。
func (hc *HealthChecker) SetOnDailyMaintenance(fn func(context.Context, time.Time)) {
	hc.onDailyMaintenance = fn
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

// probeTimeout 返回服务的单次探测超时。
// 未配置（0）时用默认 10 秒；上限 300 秒，避免脏数据让探测长时间挂住。
func probeTimeout(svc *model.Service) time.Duration {
	sec := svc.TimeoutSeconds
	if sec <= 0 {
		sec = defaultProbeTimeoutSeconds
	}
	if sec > maxProbeTimeoutSeconds {
		sec = maxProbeTimeoutSeconds
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
	// 启动时先记一次扫描时间，避免进程刚起来就为历史事件补发升级通知（那会在重启时刷屏）。
	// 新事件仍然会在创建后按升级时限正常升级。
	lastEscalationScan := time.Now()

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
			// 未确认事件的升级扫描：按分钟计的时限，不必每个心跳都查库
			if now.Sub(lastEscalationScan) >= escalationScanInterval {
				hc.escalateIncidents(ctx)
				lastEscalationScan = now
			}
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
	// 在多个服务同时故障（各自等满超时）时监控精度会明显下降。
	sem := make(chan struct{}, hc.probeConcurrency)
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
	res := hc.performCheck(ctx, svc)
	status, latency, message := res.status, res.latency, res.message

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

	isUp := res.isUp

	// HTTPS 证书到期信息（纯 HTTP 服务拿不到证书，这里会直接跳过）
	hc.syncCertExpiry(ctx, svc, res.certExpiresAt)

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
		} else {
			// 维护窗口内的失败属于计划内停机：不计入可用率，
			// 但单独计数，前端矩阵据此用蓝色（而不是红色）展示
			daily.MaintenanceCount++
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
		Content:    i18n.Current().IncidentResolvedNote(svc.Name),
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

	// 恢复通知带上本次不可用时长：从事件创建到判定恢复的间隔
	downtimeSeconds := int(time.Since(parseTime(inc.CreatedAt)).Seconds())
	if downtimeSeconds < 0 {
		downtimeSeconds = 0
	}
	hc.notifyResolved(ctx, svc, downtimeSeconds)
}

func (hc *HealthChecker) createIncident(ctx context.Context, svc *model.Service, task *model.ProbeTask, st *probeTaskState) {
	now := time.Now().Format(time.RFC3339)

	inc := &model.Incident{
		ServiceID:        svc.ID,
		Title:            i18n.Current().IncidentTitle(svc.Name),
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
		Content:    i18n.Current().IncidentCreatedNote(svc.Name),
	}
	hc.repo.CreateIncidentUpdate(ctx, update)

	svc.Status = "degraded"
	hc.repo.UpdateService(ctx, svc)

	hc.mu.Lock()
	st.autoIncidentID = inc.ID
	hc.mu.Unlock()

	utils.Info("checker auto-created incident #%d for service %s", inc.ID, svc.Name)
	hc.dataChanged()

	// 冷却窗口内只通知一次：事件照常创建，仅抑制重复通知
	if hc.allowAlert(svc.ID) {
		hc.notifyAlert(ctx, svc, now, inc.Impact)
	} else {
		utils.Info("checker suppressed alert for service %s (within cooldown)", svc.Name)
	}

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
			// 周期维护：窗口结束后直接推进到下一个窗口，状态回到 scheduled；
			// 周期已结束（超过 recurrence_until）则和一次性维护一样置为 completed。
			if advanced, ok := hc.advanceRecurringMaintenance(ctx, m, start, end, now); ok {
				changed = changed || advanced
				continue
			}

			m.Status = "completed"
			if err := hc.repo.UpdateMaintenance(ctx, m); err != nil {
				utils.Info("checker failed to update maintenance #%d to completed: %v", m.ID, err)
			} else {
				utils.Info("checker auto-updated maintenance #%d (%s) to completed", m.ID, m.Title)
				changed = true
			}
		} else if m.Status == "scheduled" {
			// 还在等待开始：到了提醒窗口就发一次提前提醒
			hc.maybeRemindUpcomingMaintenance(ctx, m, start, now)
		}
	}

	if changed {
		hc.dataChanged()
	}
}

// maybeRemindUpcomingMaintenance 在维护计划开始前发送一次提醒。
// 提前量取自配置项 MAINTENANCE_REMIND_MINUTES（0 表示关闭）；每个维护窗口只提醒一次，
// 提醒标记落库，服务重启后不会重复发送。
func (hc *HealthChecker) maybeRemindUpcomingMaintenance(ctx context.Context, m *model.Maintenance, start, now time.Time) {
	if m.Reminded {
		return
	}
	lead := config.GlobalConfig.MaintenanceRemindLead()
	if lead <= 0 {
		return
	}

	until := start.Sub(now)
	if until <= 0 || until > lead {
		return
	}

	m.Reminded = true
	if err := hc.repo.UpdateMaintenance(ctx, m); err != nil {
		utils.Info("checker failed to mark maintenance #%d as reminded: %v", m.ID, err)
		return
	}

	utils.Info("checker reminding maintenance #%d (%s) starting at %s", m.ID, m.Title, m.ScheduledStart)
	notifyMaintenanceUpcoming(m, start)
}

// advanceRecurringMaintenance 把周期性维护推进到下一个窗口。
// 返回 (是否改动, 是否是周期维护)：第二个返回值为 false 时调用方按一次性维护处理。
func (hc *HealthChecker) advanceRecurringMaintenance(ctx context.Context, m *model.Maintenance, start, end time.Time, now time.Time) (bool, bool) {
	if !normalizeRecurrence(m) {
		return false, false
	}

	nextStart, nextEnd, ok := nextMaintenanceWindow(m, start, end)
	if !ok {
		utils.Info("checker maintenance #%d (%s) recurrence finished", m.ID, m.Title)
		return false, false
	}

	// 服务停机一段时间后可能一次跳过多个窗口，这里连续推进到第一个未来窗口，
	// 否则恢复运行后会立刻又进入一次「历史窗口」，状态显示会很怪。
	for nextEnd.Before(now) {
		nextStart, nextEnd, ok = nextMaintenanceWindow(m, nextStart, nextEnd)
		if !ok {
			utils.Info("checker maintenance #%d (%s) recurrence finished while catching up", m.ID, m.Title)
			return false, false
		}
	}

	m.ScheduledStart = toCSTString(nextStart)
	m.ScheduledEnd = toCSTString(nextEnd)
	m.Status = "scheduled"
	// 新窗口需要重新提醒，否则周期维护只会在第一次提醒
	m.Reminded = false
	if err := hc.repo.UpdateMaintenance(ctx, m); err != nil {
		utils.Info("checker failed to advance maintenance #%d: %v", m.ID, err)
		return false, true
	}

	utils.Info("checker advanced maintenance #%d (%s) to next window %s", m.ID, m.Title, m.ScheduledStart)
	return true, true
}

// toCSTString 维护计划的时间统一按北京时间（UTC+8）存储，
// 与前端 datetime-local 输入及管理端的展示口径保持一致。
func toCSTString(t time.Time) string {
	return t.In(time.FixedZone("CST", 8*3600)).Format("2006-01-02T15:04:05+08:00")
}

// --- 证书到期监控 ---

// 证书到期告警的两个等级（剩余天数）。等级只会往上走，
// 因此「剩余 25 天」只发一次 30 天级提醒，「剩余 5 天」再发一次 7 天级提醒。
const (
	certWarnDays     = 30
	certCriticalDays = 7
)

// certAlertLevel 根据剩余天数给出应处的告警等级（0 / 30 / 7）。
// 已经过期的证书按最高等级处理（跳过校验时探测仍会成功，必须单独提醒）。
func certAlertLevel(remainingDays float64) int {
	switch {
	case remainingDays < float64(certCriticalDays):
		return certCriticalDays
	case remainingDays < float64(certWarnDays):
		return certWarnDays
	default:
		return 0
	}
}

// certLevelRank 告警等级的严重程度排名（越大越严重）。
// 注意等级值是「剩余天数」，数字越小越紧急，不能直接比大小。
func certLevelRank(level int) int {
	switch level {
	case certCriticalDays:
		return 2
	case certWarnDays:
		return 1
	default:
		return 0
	}
}

// syncCertExpiry 把本次探测拿到的证书到期时间写回，并在等级升高时发送一次通知。
// expires 为零值（非 HTTPS 或本次探测失败）时保持数据库里已有的信息不变。
func (hc *HealthChecker) syncCertExpiry(ctx context.Context, svc *model.Service, expires time.Time) {
	if expires.IsZero() {
		return
	}

	value := expires.UTC().Format(time.RFC3339)
	renewed := value != svc.CertExpiresAt

	level := svc.CertNotifyLevel
	if renewed {
		// 续期或换证书后重新计数，否则新证书会沿用旧证书的告警等级而不再提醒
		level = 0
	}

	remainingDays := time.Until(expires).Hours() / 24
	nextLevel := certAlertLevel(remainingDays)
	// 只在告警等级变得更严重时通知；等级回退（例如续期后仍有 25 天）不重复通知
	shouldNotify := certLevelRank(nextLevel) > certLevelRank(level)
	if shouldNotify {
		level = nextLevel
	}

	if !renewed && !shouldNotify {
		return
	}

	svc.CertExpiresAt = value
	svc.CertNotifyLevel = level
	if err := hc.repo.UpdateServiceCert(ctx, svc.ID, value, level); err != nil {
		utils.Info("checker update cert info failed for service %d: %v", svc.ID, err)
	}

	if shouldNotify {
		utils.Info("checker cert alert for service %s: expires %s (%.1f days left, level %d)",
			svc.Name, value, remainingDays, level)
		notifyCertExpiring(svc.Name, expires, remainingDays)
	}
}

// notifyCertExpiring 发送证书即将到期的通知（邮件 + webhook）
func notifyCertExpiring(name string, expires time.Time, remainingDays float64) {
	subject, body := certMessage(name, expires, remainingDays)

	sendAlertMail(subject, body)
	notifyWebhook(utils.WebhookEvent{
		Event:   utils.WebhookEventCertExpiring,
		Service: name,
		URL:     "",
		Time:    expires.UTC().Format(time.RFC3339),
		Message: body,
	})
}

// sendAlertMail 发送告警邮件。未配置收件人时静默跳过，
// 因此调用方只需处理真正的发送失败。
func sendAlertMail(subject, body string) {
	if utils.GetSetting("notify_services") == "" {
		return
	}
	if err := utils.SendAlert(subject, body); err != nil {
		utils.Info("checker send alert failed: %v", err)
	}
}

// notifyAlert 发送服务异常通知。impact 用于判断是否穿透免打扰时段。
func (hc *HealthChecker) notifyAlert(ctx context.Context, svc *model.Service, ts, impact string) {
	subject, body := alertMessage(svc, ts)

	// 邮件与 webhook 渠道相互独立，各自按自己的开关与订阅范围投递。
	// webhook 不受免打扰影响：夜间仍能通过群机器人/手机收到，只是不往邮箱堆。
	notifyWebhook(utils.WebhookEvent{
		Event:   utils.WebhookEventDown,
		Service: svc.Name,
		URL:     svc.URL,
		Time:    ts,
		Message: body,
	})

	if mutedByQuietHours(impact) {
		utils.Info("checker suppressed alert email for %s (quiet hours)", svc.Name)
		return
	}
	sendAlertMail(subject, body)
	hc.notifySubscribers(ctx, svc, subject, body)
}

// mutedByQuietHours 免打扰时段是否应该拦下这封告警邮件。
// critical 级别穿透（选定策略），保证最严重的问题不会被静音。
func mutedByQuietHours(impact string) bool {
	if impact == "critical" {
		return false
	}
	return loadAlertPolicy().quietNow(time.Now())
}

// notifyResolved 发送恢复通知。downtimeSeconds 为本次不可用时长（秒），
// 0 或负数时正文退回不含时长的文案。恢复邮件同样受免打扰时段影响（webhook 不受影响）。
func (hc *HealthChecker) notifyResolved(ctx context.Context, svc *model.Service, downtimeSeconds int) {
	subject, body := resolvedMessage(svc, downtimeSeconds)

	notifyWebhook(utils.WebhookEvent{
		Event:   utils.WebhookEventUp,
		Service: svc.Name,
		Time:    time.Now().Format(time.RFC3339),
		Message: body,
	})

	if loadAlertPolicy().quietNow(time.Now()) {
		utils.Info("checker suppressed recovery email for %s (quiet hours)", svc.Name)
		return
	}
	sendAlertMail(subject, body)
	hc.notifySubscribers(ctx, svc, subject, body)
}

// notifySubscribers 按订阅偏好把通知投递给公开订阅者。
// 订阅者的 subscribed_services 为空表示订阅全部服务；每封邮件都带退订链接。
// 投递失败只记录日志，绝不影响探测主流程。
func (hc *HealthChecker) notifySubscribers(ctx context.Context, svc *model.Service, subject, body string) {
	subscribers, err := hc.repo.ListSubscribers(ctx)
	if err != nil {
		return
	}

	for _, sub := range subscribers {
		if !subscriberWantsService(sub, svc.ID) {
			continue
		}
		if err := utils.SendSubscriberMail(sub.Email, subject, body); err != nil {
			utils.Info("checker send subscriber mail failed for %s: %v", sub.Email, err)
		}
	}
}

// subscriberWantsService 判断订阅者是否订阅了该服务。
// 空列表 = 订阅全部；配置了列表但内容无法解析时按「不订阅」处理，
// 避免脏数据导致给不该收到的人发信。
func subscriberWantsService(sub *model.Subscriber, serviceID int64) bool {
	raw := strings.TrimSpace(sub.SubscribedServices)
	if raw == "" {
		return true
	}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if id, err := strconv.ParseInt(part, 10, 64); err == nil && id == serviceID {
			return true
		}
	}
	return false
}

// notifyMaintenanceUpcoming 发送维护计划即将开始的提醒。
// 与告警通知共用邮件收件人与 webhook 渠道，通过 webhook_events 里的
// maintenance 项单独订阅。
func notifyMaintenanceUpcoming(m *model.Maintenance, start time.Time) {
	subject, body := maintenanceMessage(m, start)
	startText := cstTime(start)

	sendAlertMail(subject, body)
	notifyWebhook(utils.WebhookEvent{
		Event:   utils.WebhookEventMaintenance,
		Service: m.Title,
		Time:    startText,
		Message: body,
	})
}

// notifyWebhook 发送 webhook 通知。未配置时 SendWebhookNotifier 会直接返回 nil，
// 因此这里只需处理真正发送失败的情况，且失败不影响探测主流程。
func notifyWebhook(evt utils.WebhookEvent) {
	if err := utils.SendWebhookNotifier(evt); err != nil {
		utils.Info("checker send webhook failed: %v", err)
	}
}

func (hc *HealthChecker) performCheck(ctx context.Context, svc *model.Service) probeResult {
	switch strings.ToLower(svc.Type) {
	case "http":
		return hc.checkHTTP(ctx, svc)
	default:
		return hc.checkTCP(ctx, svc)
	}
}

// maxProbeBodyBytes 单次探测最多读取的响应体字节数。
// 关键字匹配只需要一小段内容，限制上限可以避免大文件响应把内存吃满。
const maxProbeBodyBytes = 64 * 1024

// maxProbeSnippetRunes 探测失败时写入心跳 message 的响应体片段长度（字符）。
// 统计接口要按关键字复核历史心跳，因此失败样本必须留一小段正文。
const maxProbeSnippetRunes = 200

func (hc *HealthChecker) checkHTTP(ctx context.Context, svc *model.Service) probeResult {
	start := time.Now()

	// 单次请求超时按服务配置控制，客户端本身不设 Timeout，
	// 这样同一份 client 可以被不同超时的服务复用。
	reqCtx, cancel := context.WithTimeout(ctx, probeTimeout(svc))
	defer cancel()

	method, ok := model.NormalizeHTTPMethod(svc.HTTPMethod)
	if !ok {
		return probeResult{latency: int(time.Since(start).Milliseconds()), message: "请求方法无效: " + svc.HTTPMethod}
	}

	var bodyReader io.Reader
	if svc.HTTPBody != "" {
		bodyReader = strings.NewReader(svc.HTTPBody)
	}

	req, err := http.NewRequestWithContext(reqCtx, method, svc.URL, bodyReader)
	if err != nil {
		return probeResult{latency: int(time.Since(start).Milliseconds()), message: err.Error()}
	}
	req.Close = true
	if err := applyCustomHeaders(req, svc.HTTPHeaders); err != nil {
		return probeResult{latency: int(time.Since(start).Milliseconds()), message: err.Error()}
	}
	// 自定义请求头可以覆盖 UA，因此放在后面设置作为默认值
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "LumiPulse")
	}

	resp, err := hc.clientFor(svc).Do(req)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		return probeResult{latency: latency, message: err.Error()}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxProbeBodyBytes))
	if err != nil {
		return probeResult{
			status:  resp.StatusCode,
			latency: latency,
			message: fmt.Sprintf("读取响应体失败: %v", err),
			isUp:    false,
		}
	}

	matched := svc.MatchesProbe(resp.StatusCode, string(body))
	return probeResult{
		status:        resp.StatusCode,
		latency:       latency,
		message:       probeMessage(svc, resp.StatusCode, string(body), matched),
		isUp:          matched,
		certExpiresAt: peerCertExpiry(resp),
	}
}

// applyCustomHeaders 解析自定义请求头 JSON 并写入请求。
// 解析失败时返回错误，让本次探测显式失败，而不是静默用错误的头部去请求。
func applyCustomHeaders(req *http.Request, raw string) error {
	headers, err := model.ParseCustomHeaders(raw)
	if err != nil {
		return err
	}
	for k, v := range headers {
		if strings.TrimSpace(k) == "" {
			continue
		}
		req.Header.Set(k, v)
	}
	return nil
}

// probeMessage 生成写入心跳 message 的内容。**无论成功与否都会保留一小段响应体**：
// 统计接口（日志筛选、延迟分桶、热力图、分位数）需要按「期望关键字」复核历史心跳，
// 只能依赖这里的片段，否则配置了关键字之后成功样本会被误判为失败。
func probeMessage(svc *model.Service, status int, body string, matched bool) string {
	reason := fmt.Sprintf("HTTP %d", status)
	if !matched {
		if !svc.MatchesExpectStatus(status) {
			reason += " 不在期望状态码范围内"
		}
		if !svc.MatchesBody(body) {
			reason += " 响应内容未包含期望关键字"
		}
	}
	if snippet := bodySnippet(body); snippet != "" {
		reason += ": " + snippet
	}
	return reason
}

// bodySnippet 截取响应体片段，按字符截断避免把多字节字符切成乱码。
func bodySnippet(body string) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) > maxProbeSnippetRunes {
		return string(runes[:maxProbeSnippetRunes]) + "…"
	}
	return string(runes)
}

// peerCertExpiry 取对端叶子证书的到期时间。非 HTTPS（含被跳过校验的自签证书）
// 或证书链为空时返回零值，调用方据此跳过证书相关逻辑。
func peerCertExpiry(resp *http.Response) time.Time {
	if resp.TLS == nil || len(resp.TLS.PeerCertificates) == 0 {
		return time.Time{}
	}
	return resp.TLS.PeerCertificates[0].NotAfter
}

func (hc *HealthChecker) checkTCP(ctx context.Context, svc *model.Service) probeResult {
	start := time.Now()

	dialCtx, cancel := context.WithTimeout(ctx, probeTimeout(svc))
	defer cancel()

	dialer := net.Dialer{}
	conn, err := dialer.DialContext(dialCtx, "tcp", svc.URL)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		return probeResult{latency: latency, message: err.Error()}
	}
	conn.Close()
	return probeResult{status: 1, latency: latency, message: "TCP 连接成功", isUp: true}
}

func (hc *HealthChecker) cleanup(ctx context.Context) {
	dailyDays := config.GlobalConfig.DailyRetention()
	heartbeatDays := config.GlobalConfig.HeartbeatRetention()

	// 必须在删除旧每日明细之前归档：月报要覆盖的时间范围比每日明细的保留窗口更长
	hc.archiveFinishedMonths(ctx)

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

	// 每日收尾任务（月报邮件等）。回调内部自行判断是否需要真的发送。
	if hc.onDailyMaintenance != nil {
		hc.onDailyMaintenance(ctx, time.Now())
	}
}

// archiveFinishedMonths 把已结束月份的每日明细汇总进 ServiceMonthly。
// ServiceDaily 会被 DAILY_RETENTION_DAYS 清理，而月报需要长期可查，
// 因此每次清理前先把已结束的月份固化下来（已固化的月份跳过）。
func (hc *HealthChecker) archiveFinishedMonths(ctx context.Context) {
	now := time.Now().UTC()
	currentMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	const lookbackMonths = 6
	for i := 1; i <= lookbackMonths; i++ {
		month := currentMonth.AddDate(0, -i, 0)
		monthKey := month.Format("2006-01")

		stored, err := hc.repo.ListServiceMonthlies(ctx, monthKey)
		if err != nil {
			utils.Info("checker list monthly reports failed for %s: %v", monthKey, err)
			continue
		}
		// 已经固化的月份不再重算，避免重复累加
		closed := make(map[int64]bool, len(stored))
		for _, m := range stored {
			if m.ClosedAt != "" {
				closed[m.ServiceID] = true
			}
		}

		from := month.Format("2006-01-02")
		toExclusive := month.AddDate(0, 1, 0).Format("2006-01-02")

		aggregated, err := hc.repo.AggregateServiceMonthly(ctx, from, toExclusive)
		if err != nil {
			utils.Info("checker aggregate monthly reports failed for %s: %v", monthKey, err)
			continue
		}
		incidentCounts, err := hc.repo.CountIncidentsBetween(ctx, from, toExclusive)
		if err != nil {
			incidentCounts = nil
		}
		incidentDowntime, err := hc.repo.AggregateIncidentDowntime(ctx, from, toExclusive)
		if err != nil {
			incidentDowntime = nil
		}

		closedAt := now.Format("2006-01-02T15:04:05Z")
		archived := 0
		for _, m := range aggregated {
			if closed[m.ServiceID] {
				continue
			}
			m.Month = monthKey
			m.IncidentTotal = incidentCounts[m.ServiceID]
			m.IncidentDowntimeSecs = incidentDowntime[m.ServiceID]
			m.ClosedAt = closedAt
			if err := hc.repo.UpsertServiceMonthly(ctx, m, false); err != nil {
				utils.Info("checker archive monthly report failed for service %d %s: %v", m.ServiceID, monthKey, err)
				continue
			}
			archived++
		}
		if archived > 0 {
			utils.Info("checker archived %d service(s) for month %s", archived, monthKey)
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

func joinServiceIDs(ids []int64) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	return strings.Join(parts, ",")
}
