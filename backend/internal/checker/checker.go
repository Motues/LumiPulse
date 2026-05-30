package checker

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

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
	autoIncidentID       int64 // 0 = no auto-incident
}

type HealthChecker struct {
	repo     repository.Repository
	interval time.Duration
	client   *http.Client
	states   map[int64]*probeTaskState // key = serviceID
	mu       sync.Mutex
	stop     chan struct{}
	wg       sync.WaitGroup
}

func New(repo repository.Repository, insecureSkipVerify bool) *HealthChecker {
	return &HealthChecker{
		repo:     repo,
		interval: 1 * time.Minute,
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureSkipVerify},
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		states: make(map[int64]*probeTaskState),
		stop:   make(chan struct{}),
	}
}

func (hc *HealthChecker) Start(ctx context.Context) {
	hc.wg.Add(1)
	go hc.loop(ctx)
	log.Println("[checker] started (interval: 1m)")
}

func (hc *HealthChecker) Stop() {
	close(hc.stop)
	hc.wg.Wait()
	log.Println("[checker] stopped")
}

func (hc *HealthChecker) loop(ctx context.Context) {
	defer hc.wg.Done()

	hc.cleanup(ctx)
	lastCleanup := time.Now()

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

			if time.Since(lastCleanup) > 24*time.Hour {
				hc.cleanup(ctx)
				lastCleanup = time.Now()
			}
		}
	}
}

func (hc *HealthChecker) checkAll(ctx context.Context) {
	services, err := hc.repo.ListServices(ctx)
	if err != nil {
		log.Printf("[checker] list services error: %v", err)
		return
	}

	// Load probe tasks and build serviceID -> probeTask map
	probeTasks, err := hc.repo.ListProbeTasks(ctx)
	if err != nil {
		log.Printf("[checker] list probe tasks error: %v", err)
		return
	}
	taskMap := make(map[int64]*model.ProbeTask, len(probeTasks))
	for _, t := range probeTasks {
		taskMap[t.ServiceID] = t
	}

	today := time.Now().Format("2006-01-02")

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

	for _, svc := range services {
		if !svc.IsActive {
			continue
		}

		// Check if service has an active probe task
		task, hasTask := taskMap[svc.ID]
		if !hasTask || !task.IsActive {
			continue
		}

		status, latency, message := hc.performCheck(svc)

		// Record heartbeat
		hb := &model.Heartbeat{
			ServiceID: svc.ID,
			Status:    status,
			Latency:   latency,
			Message:   message,
		}
		if err := hc.repo.CreateHeartbeat(ctx, hb); err != nil {
			log.Printf("[checker] create heartbeat failed for service %d: %v", svc.ID, err)
		}

		// Update daily record
		daily, err := hc.repo.GetOrCreateServiceDaily(ctx, svc.ID, today)
		if err != nil {
			log.Printf("[checker] get/create daily failed for service %d: %v", svc.ID, err)
			continue
		}

		isUp := (status >= 200 && status < 400) || status == 1
		if isUp {
			daily.UptimeCount++
		} else if !inMaint[svc.ID] {
			daily.DowntimeCount++
		}
		daily.TotalLatency += latency
		if err := hc.repo.UpdateServiceDaily(ctx, daily); err != nil {
			log.Printf("[checker] update daily failed for service %d: %v", svc.ID, err)
		}

		// Auto-incident logic with probe task config
		hc.trackProbeState(ctx, svc, task, isUp)
	}
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
				log.Printf("[checker] skipping incident for service %s (under maintenance)", svc.Name)
				return
			}

			// Don't create a new incident if one already exists for this service
			if existing, err := hc.repo.GetActiveIncidentByService(ctx, svc.ID); err == nil && existing != nil {
				hc.mu.Lock()
				st.autoIncidentID = existing.ID
				hc.mu.Unlock()
				log.Printf("[checker] linked existing incident #%d for service %s", existing.ID, svc.Name)
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
		log.Printf("[checker] incident #%d not found for resolve: %v", incID, err)
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
		log.Printf("[checker] failed to create incident update: %v", err)
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

	log.Printf("[checker] resolved incident #%d for service %s", inc.ID, svc.Name)
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
		log.Printf("[checker] failed to create incident: %v", err)
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

	log.Printf("[checker] auto-created incident #%d for service %s", inc.ID, svc.Name)
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
		log.Printf("[checker] list incidents by server error: %v", err)
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

	log.Printf("[checker] merged incident #%d as child of #%d", source.ID, target.ID)
}

func (hc *HealthChecker) reconcileMaintenanceStatus(ctx context.Context) {
	maintenances, err := hc.repo.ListMaintenances(ctx)
	if err != nil {
		log.Printf("[checker] list maintenances error: %v", err)
		return
	}

	now := time.Now().UTC()

	for _, m := range maintenances {
		if m.Status != "scheduled" && m.Status != "in_progress" {
			continue
		}

		start := parseTime(m.ScheduledStart)
		end := parseTime(m.ScheduledEnd)

		if m.Status == "scheduled" && !now.Before(start) {
			m.Status = "in_progress"
			if err := hc.repo.UpdateMaintenance(ctx, m); err != nil {
				log.Printf("[checker] failed to update maintenance #%d to in_progress: %v", m.ID, err)
			} else {
				log.Printf("[checker] auto-updated maintenance #%d (%s) to in_progress", m.ID, m.Title)
			}
		} else if m.Status == "in_progress" && !now.Before(end) {
			m.Status = "completed"
			if err := hc.repo.UpdateMaintenance(ctx, m); err != nil {
				log.Printf("[checker] failed to update maintenance #%d to completed: %v", m.ID, err)
			} else {
				log.Printf("[checker] auto-updated maintenance #%d (%s) to completed", m.ID, m.Title)
			}
		}
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
		log.Printf("[checker] send alert failed: %v", err)
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
		log.Printf("[checker] send alert failed: %v", err)
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

	resp, err := hc.client.Do(req)
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
	beforeDaily := time.Now().AddDate(0, 0, -90).Format("2006-01-02")
	if err := hc.repo.DeleteOldServiceDailies(ctx, beforeDaily); err != nil {
		log.Printf("[checker] cleanup dailies failed: %v", err)
	} else {
		log.Println("[checker] cleaned up daily records older than 90 days")
	}

	beforeHB := time.Now().AddDate(0, 0, -7).UTC().Format("2006-01-02T15:04:05Z")
	if err := hc.repo.DeleteOldHeartbeats(ctx, beforeHB); err != nil {
		log.Printf("[checker] cleanup heartbeats failed: %v", err)
	} else {
		log.Println("[checker] cleaned up heartbeats older than 7 days")
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
