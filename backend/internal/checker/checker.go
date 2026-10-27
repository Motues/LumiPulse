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
	// resolveAfterSuccesses 连续成功多少次才判定故障恢复。恢复判定比故障判定
	// （probe_task.trigger_count）更保守，避免抖动导致事件反复开关。
	resolveAfterSuccesses = 5
	// mergeWindow 自动合并的时间窗口：只有这个时间窗内发生的故障才会被合并。
	// 否则一次很久以前没关掉的活跃事件会把之后所有无关故障都吸收成子事件。
	mergeWindow = 30 * time.Minute
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
	// certAlerts 已渲染但尚未投递的证书到期提醒。同一聚合窗口内多个服务的提醒
	// 会合并成一封邮件，由调度循环在窗口结束后统一投递（见 queueCertAlert / flushCertAlerts）。
	certAlerts []pendingCertAlert
	mu         sync.Mutex
	// incidentMu 串行化自动事件的「创建 → 合并 → 解决」。
	// 探测是并发执行的：同一秒内多个服务同时达到失败阈值时，两次自动合并会
	// 分别把对方挂成子事件（A→B 与 B→A），形成循环父子——两个事件都不再是根
	// 事件，事件列表与仪表盘会把它们全部隐藏，affected_services 也会互相覆盖。
	// 因此所有会改变「父子关系 / affected_services / 解决状态」的写操作都必须持有它。
	incidentMu sync.Mutex
	stop       chan struct{}
	wg         sync.WaitGroup
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
			// 退出前把还压在聚合窗口里的证书提醒发出去，否则 level 已落库、不会再补发
			hc.flushCertAlerts(true)
			return
		case <-hc.stop:
			hc.flushCertAlerts(true)
			return
		case <-ticker.C:
			hc.checkAll(ctx)
			hc.reconcileMaintenanceStatus(ctx)
			// 证书提醒按聚合窗口合并投递：窗口内累积的服务在这里一次性发出
			hc.flushCertAlerts(false)

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
		shouldResolve := st.consecutiveSuccesses >= resolveAfterSuccesses
		incID := st.autoIncidentID
		hc.mu.Unlock()

		if shouldResolve {
			if incID == 0 {
				incID = hc.findActiveIncidentID(ctx, svc.ID)
			}
			if incID != 0 {
				hc.tryResolveIncident(ctx, svc, incID)
			}
		}
	} else {
		st.consecutiveSuccesses = 0
		st.consecutiveFailures++
		failCount := st.consecutiveFailures
		incID := st.autoIncidentID
		hc.mu.Unlock()

		// 绑定的事件可能已被人工解决/删除。定期校验一次并解除绑定，
		// 否则服务一直故障、事件列表里却永远没有活跃事件。
		// 采用「每 resolveAfterSuccesses 次失败校验一次」节流，避免每次探测都查库。
		if incID != 0 && failCount%resolveAfterSuccesses == 0 && !hc.incidentIsActive(ctx, incID) {
			staleID := incID
			hc.mu.Lock()
			if st.autoIncidentID == staleID {
				st.autoIncidentID = 0
			}
			hc.mu.Unlock()
			incID = 0
			utils.Info("checker incident #%d for service %s is no longer active", staleID, svc.Name)
		}

		if failCount >= triggerCount && incID == 0 {
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

// incidentIsActive 判断事件是否还存在且未解决。
func (hc *HealthChecker) incidentIsActive(ctx context.Context, incID int64) bool {
	inc, err := hc.repo.GetIncident(ctx, incID)
	return err == nil && inc.Status != "resolved"
}

// findActiveIncidentID 找出当前覆盖该服务的活跃根事件。
// 只在进程重启（内存状态丢失）后作为兜底使用：
//   - 只看根事件，子事件必须随父事件一起解决，不能单独"解决"并单独发恢复通知；
//   - 同时判断 affected_services，保证"只是被合并进父事件"的服务也能推动解决。
func (hc *HealthChecker) findActiveIncidentID(ctx context.Context, serviceID int64) int64 {
	incidents, err := hc.repo.ListActiveIncidents(ctx)
	if err != nil {
		return 0
	}
	for _, inc := range incidents {
		if inc.ParentID != nil {
			continue
		}
		for _, id := range incidentServiceIDs(inc) {
			if id == serviceID {
				return inc.ID
			}
		}
	}
	return 0
}

// rootIncident 沿着 parent_id 上溯到根事件。
// 限制跳数与 visited 集合，防止历史脏数据里的循环父子把这里变成死循环。
func (hc *HealthChecker) rootIncident(ctx context.Context, inc *model.Incident) *model.Incident {
	if inc == nil || inc.ParentID == nil {
		return inc
	}

	visited := map[int64]bool{inc.ID: true}
	for depth := 0; inc.ParentID != nil && depth < 8; depth++ {
		parentID := *inc.ParentID
		if visited[parentID] {
			break // 循环引用：停在当前节点，交给解决逻辑收口
		}
		visited[parentID] = true

		parent, err := hc.repo.GetIncident(ctx, parentID)
		if err != nil {
			break
		}
		inc = parent
	}
	return inc
}

// tryResolveIncident 尝试解决事件。对于合并事件，只有所有受影响服务都恢复时才解决。
// 本函数必须是幂等的：同一个事件被重复调用时只清理内存状态，绝不能再发一次恢复通知。
func (hc *HealthChecker) tryResolveIncident(ctx context.Context, svc *model.Service, incID int64) {
	inc, err := hc.repo.GetIncident(ctx, incID)
	if err != nil {
		// 事件可能已被人工删除：清掉内存指针，否则每次探测成功都会重试并刷恢复通知。
		hc.forgetIncidentID(incID, svc.ID)
		utils.Info("checker incident #%d not found for resolve: %v", incID, err)
		return
	}

	// 人工合并可能让内存里的指针仍然指向子事件；解决时一律上溯到根事件，
	// 避免子事件先被单独解决、父事件随后又发一次恢复通知。
	inc = hc.rootIncident(ctx, inc)

	// 已经解决的事件不再重复解决、重复发信；只清掉可能残留的内存指针。
	if inc.Status == "resolved" {
		hc.clearIncidentState(inc, svc.ID)
		return
	}

	affectedIDs := incidentServiceIDs(inc)

	// If merged incident with multiple services, check if ALL are recovered
	if len(affectedIDs) > 1 {
		allRecovered := true
		for _, svcID := range affectedIDs {
			hc.mu.Lock()
			st, ok := hc.states[svcID]
			if ok && st.consecutiveSuccesses < resolveAfterSuccesses {
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

// forgetIncidentID 只在服务当前绑定的正是 incID 时解除绑定，避免误清其他事件。
func (hc *HealthChecker) forgetIncidentID(incID int64, serviceIDs ...int64) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	for _, id := range serviceIDs {
		st, ok := hc.states[id]
		if !ok || st.autoIncidentID != incID {
			continue
		}
		st.autoIncidentID = 0
		st.consecutiveSuccesses = 0
	}
}

// clearIncidentState 清掉与事件相关的内存指针（并按需重置连续成功计数），
// 保证已解决/已删除的事件不会在后续探测里被反复"解决"。
func (hc *HealthChecker) clearIncidentState(inc *model.Incident, extraIDs ...int64) {
	ids := make([]int64, 0, len(extraIDs)+3)
	ids = append(ids, extraIDs...)
	if inc != nil {
		ids = append(ids, inc.ServiceID)
		ids = append(ids, incidentServiceIDs(inc)...)
	}

	hc.mu.Lock()
	defer hc.mu.Unlock()
	for _, id := range ids {
		st, ok := hc.states[id]
		if !ok {
			continue
		}
		st.autoIncidentID = 0
		st.consecutiveSuccesses = 0
	}
}

func (hc *HealthChecker) resolveIncident(ctx context.Context, svc *model.Service, inc *model.Incident) {
	hc.incidentMu.Lock()

	// 解决前重新读取：并发探测或人工操作可能已经解决过同一个事件。
	fresh, err := hc.repo.GetIncident(ctx, inc.ID)
	if err != nil {
		hc.incidentMu.Unlock()
		hc.clearIncidentState(inc, svc.ID)
		utils.Info("checker incident #%d not found for resolve: %v", inc.ID, err)
		return
	}
	if fresh.Status == "resolved" {
		hc.incidentMu.Unlock()
		hc.clearIncidentState(fresh, svc.ID)
		return
	}

	now := time.Now().Format(time.RFC3339)

	update := &model.IncidentUpdate{
		IncidentID: fresh.ID,
		Status:     "resolved",
		Content:    i18n.Current().IncidentResolvedNote(svc.Name),
	}
	if err := hc.repo.CreateIncidentUpdate(ctx, update); err != nil {
		utils.Info("checker failed to create incident update: %v", err)
	}

	fresh.Status = "resolved"
	fresh.UpdatedAt = now
	if err := hc.repo.UpdateIncident(ctx, fresh); err != nil {
		// 状态没写成功时不要发恢复通知，否则下一轮探测还会重复"解决"并发信。
		hc.incidentMu.Unlock()
		utils.Info("checker failed to resolve incident #%d: %v", fresh.ID, err)
		return
	}

	// 子事件跟随父事件一起解决，避免留下永远 investigating 的"幽灵事件"。
	if err := hc.repo.UpdateChildrenStatus(ctx, fresh.ID, "resolved"); err != nil {
		utils.Info("checker failed to resolve children of incident #%d: %v", fresh.ID, err)
	}
	children, err := hc.repo.ListChildIncidents(ctx, fresh.ID)
	if err != nil {
		children = nil
	}

	// 恢复所有受影响服务（合并事件的 affected_services 可能包含多个服务）。
	affectedIDs := incidentServiceIDs(fresh)
	restoreIDs := append([]int64{}, affectedIDs...)
	for _, child := range children {
		restoreIDs = appendIfMissing(restoreIDs, child.ServiceID)
	}
	for _, svcID := range restoreIDs {
		if s, err := hc.repo.GetService(ctx, svcID); err == nil {
			s.Status = "operational"
			if err := hc.repo.UpdateService(ctx, s); err != nil {
				utils.Info("checker failed to restore service %d: %v", svcID, err)
			}
		}
	}

	// 清状态：受影响服务 + 子事件服务 + 当前探测服务都解除事件绑定。
	// 这一步不能只依赖 affected_services——历史数据里它可能为空或缺服务，
	// 一旦漏清，该事件就会在每次探测成功时被重复"解决"。
	stateIDs := append([]int64{}, restoreIDs...)
	for _, child := range children {
		stateIDs = append(stateIDs, incidentServiceIDs(child)...)
	}
	stateIDs = append(stateIDs, svc.ID)

	hc.mu.Lock()
	for _, id := range stateIDs {
		if st, ok := hc.states[id]; ok {
			st.autoIncidentID = 0
			// 重置连续成功计数：同一个事件不会再被下一次探测重复"解决"。
			st.consecutiveSuccesses = 0
		}
	}
	hc.mu.Unlock()
	hc.incidentMu.Unlock()

	utils.Info("checker resolved incident #%d for service %s", fresh.ID, svc.Name)
	hc.dataChanged()

	// 恢复通知带上本次不可用时长：从事件创建到判定恢复的间隔
	downtimeSeconds := int(time.Since(parseTime(fresh.CreatedAt)).Seconds())
	if downtimeSeconds < 0 {
		downtimeSeconds = 0
	}
	hc.notifyResolved(ctx, svc, downtimeSeconds)
}

func (hc *HealthChecker) createIncident(ctx context.Context, svc *model.Service, task *model.ProbeTask, st *probeTaskState) {
	now := time.Now().Format(time.RFC3339)

	hc.incidentMu.Lock()
	// 并发兜底：同一个服务的两次探测可能同时越过失败阈值，这里再查一次，
	// 保证一次故障只创建一个事件（而不是在列表里出现两条重复事件）。
	if existing, err := hc.repo.GetActiveIncidentByService(ctx, svc.ID); err == nil && existing != nil {
		hc.mu.Lock()
		st.autoIncidentID = existing.ID
		hc.mu.Unlock()
		hc.incidentMu.Unlock()
		utils.Info("checker linked existing incident #%d for service %s", existing.ID, svc.Name)
		return
	}

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
		hc.incidentMu.Unlock()
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
	hc.incidentMu.Unlock()

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
			hc.tryAutoMerge(ctx, inc, task, srv)
		}
	}
}

// tryAutoMerge 检查同服务器是否有其他活跃事件，有则自动合并。
// 只有在「同服务器同时异常的服务数达到 auto_merge_threshold」且「事件创建时间
// 落在 mergeWindow 内」时才合并，避免把很久以前遗留的活跃事件当成合并目标。
func (hc *HealthChecker) tryAutoMerge(ctx context.Context, newInc *model.Incident, task *model.ProbeTask, srv *model.Server) {
	if task.ServerID == nil || srv == nil {
		return
	}

	hc.incidentMu.Lock()
	defer hc.incidentMu.Unlock()

	// 重新读取：创建与合并之间，另一个协程可能已经把本事件合并或解决了。
	fresh, err := hc.repo.GetIncident(ctx, newInc.ID)
	if err != nil {
		utils.Info("checker merge reload incident #%d failed: %v", newInc.ID, err)
		return
	}
	if fresh.Status == "resolved" || fresh.ParentID != nil {
		return
	}

	siblings, err := hc.repo.ListIncidentsByServer(ctx, *task.ServerID)
	if err != nil {
		utils.Info("checker list incidents by server error: %v", err)
		return
	}

	// auto_merge_threshold：同服务器上同时异常的服务数达到阈值才自动合并。
	// 统计口径是"受影响的服务集合"，合并事件只按其中的服务计一次。
	threshold := srv.AutoMergeThreshold
	if threshold < 1 {
		threshold = 1
	}
	affected := make(map[int64]bool)
	for _, sib := range siblings {
		for _, id := range incidentServiceIDs(sib) {
			affected[id] = true
		}
	}
	if len(affected) < threshold {
		return
	}

	for _, sib := range siblings {
		if sib.ID == fresh.ID || sib.Status == "resolved" || sib.ParentID != nil {
			continue
		}
		if !withinMergeWindow(fresh, sib) {
			continue
		}

		// 目标也重新读取，避免用列表里的旧快照覆盖 affected_services（丢服务）。
		target, err := hc.repo.GetIncident(ctx, sib.ID)
		if err != nil || target.Status == "resolved" || target.ParentID != nil || target.ID == fresh.ID {
			continue
		}
		hc.mergeIncidents(ctx, target, fresh)
		return
	}
}

// withinMergeWindow 判断两个事件是否属于同一次故障（创建时间距离不超过 mergeWindow）。
func withinMergeWindow(a, b *model.Incident) bool {
	diff := parseTime(a.CreatedAt).Sub(parseTime(b.CreatedAt))
	if diff < 0 {
		diff = -diff
	}
	return diff <= mergeWindow
}

// mergeIncidents 将 source 作为子事件合并到 target。
// 调用方必须持有 hc.incidentMu；函数内部会再做一次单向校验，确保不会出现
// A→B 与 B→A 同时成立（循环父子）的情况。
func (hc *HealthChecker) mergeIncidents(ctx context.Context, target, source *model.Incident) {
	if target.ID == source.ID || target.ParentID != nil || source.ParentID != nil {
		return
	}

	// Set source as child of target
	source.ParentID = &target.ID
	if err := hc.repo.UpdateIncident(ctx, source); err != nil {
		utils.Info("checker merge incident #%d failed: %v", source.ID, err)
		return
	}

	// Merge affected services into target for status tracking.
	// affected_services 为空的历史事件按 service_id 兜底，否则会"丢服务"并导致
	// 解决时清不掉内存指针（恢复邮件刷屏的根因）。
	targetSvcs := incidentServiceIDs(target)
	sourceSvcs := incidentServiceIDs(source)
	merged := appendIfMissing(targetSvcs, sourceSvcs...)
	target.AffectedServices = joinServiceIDs(merged)
	if err := hc.repo.UpdateIncident(ctx, target); err != nil {
		// 回滚父子关系，避免留下"子事件建立了、父事件却没记录受影响服务"的半成品。
		utils.Info("checker merge incident #%d into #%d failed: %v", source.ID, target.ID, err)
		source.ParentID = nil
		hc.repo.UpdateIncident(ctx, source)
		return
	}

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
	// certAlertBatchWindow 证书到期提醒的聚合窗口。多个服务集中续期时，窗口内
	// 触发的提醒会合并成一封邮件，避免一次收到十几封内容雷同的信。
	// 证书提醒本身提前 30 / 7 天，延迟几分钟投递没有任何影响。
	certAlertBatchWindow = 5 * time.Minute
)

// pendingCertAlert 已渲染、等待聚合投递的一条证书到期提醒。
// subject / body 在入队时按当时的模板与语言渲染好，投递时不再重新渲染。
type pendingCertAlert struct {
	serviceID int64
	subject   string
	body      string
	queuedAt  time.Time
}

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
		hc.notifyCertExpiring(svc.ID, svc.Name, expires, remainingDays)
	}
}

// notifyCertExpiring 投递一次证书到期提醒。
// webhook 立即按服务单独推送（事件粒度保持不变），邮件进入聚合缓冲，
// 与同一窗口内其他服务的提醒合并成一封发送（见 flushCertAlerts）。
func (hc *HealthChecker) notifyCertExpiring(serviceID int64, name string, expires time.Time, remainingDays float64) {
	subject, body := certMessage(name, expires, remainingDays)

	notifyWebhook(utils.WebhookEvent{
		Event:   utils.WebhookEventCertExpiring,
		Service: name,
		URL:     "",
		Time:    expires.UTC().Format(time.RFC3339),
		Message: body,
	})

	hc.queueCertAlert(serviceID, subject, body)
}

// queueCertAlert 把一条证书提醒放入聚合缓冲。
// 同一服务在窗口内只保留最新一条（例如窗口内先续期、随后又被判定为即将到期），
// 并保留最早的入队时间，保证窗口一定会到期而不是被反复刷新。
func (hc *HealthChecker) queueCertAlert(serviceID int64, subject, body string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	for i := range hc.certAlerts {
		if hc.certAlerts[i].serviceID == serviceID {
			hc.certAlerts[i].subject = subject
			hc.certAlerts[i].body = body
			return
		}
	}

	hc.certAlerts = append(hc.certAlerts, pendingCertAlert{
		serviceID: serviceID,
		subject:   subject,
		body:      body,
		queuedAt:  time.Now(),
	})
	utils.Info("checker queued cert alert for %s (pending: %d)", subject, len(hc.certAlerts))
}

// flushCertAlerts 投递聚合缓冲里的证书提醒。窗口未到时直接返回；
// force 为 true（进程退出）时忽略窗口立即发送，避免已经落库的提醒被丢掉。
// 只有一条时沿用原有的单服务邮件（主题与正文和聚合前完全一致）；
// 多条时合并成一封：主题是「N 个服务的证书即将到期」的汇总文案，
// 正文是各服务模板渲染结果的拼接，因此自定义证书模板里保留 {{service}} 才能分辨服务。
func (hc *HealthChecker) flushCertAlerts(force bool) {
	hc.mu.Lock()
	if len(hc.certAlerts) == 0 {
		hc.mu.Unlock()
		return
	}
	if !force && time.Since(hc.certAlerts[0].queuedAt) < certAlertBatchWindow {
		hc.mu.Unlock()
		return
	}
	pending := hc.certAlerts
	hc.certAlerts = nil
	hc.mu.Unlock()

	if len(pending) > 1 {
		utils.Info("checker sending combined cert alert for %d services", len(pending))
	}
	subject, body := certDigestMessage(pending)
	sendAlertMail(subject, body)
}

// certDigestMessage 把一次投递里的证书提醒合成邮件主题与正文。
// 只有一条时原样返回该服务的主题与正文（与聚合前的单服务邮件完全一致）。
func certDigestMessage(pending []pendingCertAlert) (subject, body string) {
	if len(pending) == 1 {
		return pending[0].subject, pending[0].body
	}

	bodies := make([]string, 0, len(pending))
	for _, p := range pending {
		bodies = append(bodies, p.body)
	}
	return i18n.Current().CertExpiringDigestSubject(len(pending)), strings.Join(bodies, "\n\n")
}

// sendAlertMail 发送告警邮件（维护提醒 / 证书到期等没有单一服务上下文的通知）。
// 未启用邮件或未配置监控服务时静默跳过，因此调用方只需处理真正的发送失败。
func sendAlertMail(subject, body string) {
	if !utils.IsEmailEnabled() {
		return
	}
	if utils.GetSetting("notify_services") == "" {
		return
	}
	if err := utils.SendAlert(subject, body); err != nil {
		utils.Info("checker send alert failed: %v", err)
	}
}

// allowServiceEmail 判断某个服务的告警邮件是否应该发送：
// 邮件通知的总开关 + 「监控服务」白名单（notify_services 是逗号分隔的服务 ID）。
// 之前这里只判断了白名单是否为空，导致没被勾选的服务也会发信。
func allowServiceEmail(serviceID int64) bool {
	return serviceEmailAllowed(serviceID, utils.IsEmailEnabled(), utils.GetSetting("notify_services"))
}

// serviceEmailAllowed 是 allowServiceEmail 的纯函数版本，便于单元测试。
// 白名单为空表示未选择任何监控服务，按「不发送」处理（与历史行为一致）。
func serviceEmailAllowed(serviceID int64, emailEnabled bool, notifyServices string) bool {
	if !emailEnabled {
		return false
	}
	raw := strings.TrimSpace(notifyServices)
	if raw == "" {
		return false
	}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if id, err := strconv.ParseInt(part, 10, 64); err == nil && id == serviceID {
			return true
		}
	}
	return false
}

// sendServiceAlertMail 发送与服务相关的告警邮件（故障 / 恢复 / 升级）。
// 不在「监控服务」白名单里的服务只发 webhook，不发邮件。
func sendServiceAlertMail(serviceID int64, subject, body string) {
	if !allowServiceEmail(serviceID) {
		return
	}
	if err := utils.SendAlert(subject, body); err != nil {
		utils.Info("checker send alert failed for service %d: %v", serviceID, err)
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
	sendServiceAlertMail(svc.ID, subject, body)
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
	sendServiceAlertMail(svc.ID, subject, body)
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

// incidentServiceIDs 返回事件关联的服务 ID。
// affected_services 为空的历史数据回退到 service_id，避免事件"关联不到任何服务"，
// 进而导致解决时漏清内存指针、恢复通知重复发送。
func incidentServiceIDs(inc *model.Incident) []int64 {
	if inc == nil {
		return nil
	}
	ids := parseAffectedServices(inc.AffectedServices)
	if len(ids) == 0 && inc.ServiceID > 0 {
		ids = []int64{inc.ServiceID}
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
