package checker

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"lumipluse-backend/internal/model"
	"lumipluse-backend/internal/pkg/utils"
	"lumipluse-backend/internal/repository"
)

type serviceState struct {
	consecutiveFailures  int
	consecutiveSuccesses int
	autoIncidentID       int64 // 0 = no auto-incident
}

type HealthChecker struct {
	repo     repository.Repository
	interval time.Duration
	client   *http.Client
	states   map[int64]*serviceState
	mu       sync.Mutex
	stop     chan struct{}
	wg       sync.WaitGroup
}

func New(repo repository.Repository) *HealthChecker {
	return &HealthChecker{
		repo:     repo,
		interval: 1 * time.Minute,
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		states: make(map[int64]*serviceState),
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

	// Do first check immediately
	hc.checkAll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-hc.stop:
			return
		case <-ticker.C:
			hc.checkAll(ctx)

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

	today := time.Now().Format("2006-01-02")

	for _, svc := range services {
		if !svc.IsActive {
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
		} else {
			daily.DowntimeCount++
		}
		daily.TotalLatency += latency
		if err := hc.repo.UpdateServiceDaily(ctx, daily); err != nil {
			log.Printf("[checker] update daily failed for service %d: %v", svc.ID, err)
		}

		// Auto-incident logic
		hc.trackServiceState(ctx, svc, isUp)
	}
}

func (hc *HealthChecker) trackServiceState(ctx context.Context, svc *model.Service, isUp bool) {
	hc.mu.Lock()
	st, exists := hc.states[svc.ID]
	if !exists {
		st = &serviceState{}
		hc.states[svc.ID] = st
	}
	hc.mu.Unlock()

	if isUp {
		st.consecutiveFailures = 0
		st.consecutiveSuccesses++

		// After 5 consecutive successes, resolve any active incident
		if st.consecutiveSuccesses >= 5 {
			incID := st.autoIncidentID
			if incID == 0 {
				// Check for manually-created incidents too
				if existing, err := hc.repo.GetActiveIncidentByService(ctx, svc.ID); err == nil && existing != nil {
					incID = existing.ID
				}
			}

			if incID != 0 {
				now := time.Now().Format(time.RFC3339)

				update := &model.IncidentUpdate{
					IncidentID: incID,
					Status:     "resolved",
					Content:    fmt.Sprintf("%s 服务已恢复运行", svc.Name),
				}
				if err := hc.repo.CreateIncidentUpdate(ctx, update); err != nil {
					log.Printf("[checker] failed to create incident update: %v", err)
				}

				if inc, err := hc.repo.GetIncident(ctx, incID); err == nil {
					inc.Status = "resolved"
					inc.UpdatedAt = now
					hc.repo.UpdateIncident(ctx, inc)
				}

				svc.Status = "operational"
				hc.repo.UpdateService(ctx, svc)

				log.Printf("[checker] resolved incident #%d for service %s", incID, svc.Name)
				st.autoIncidentID = 0

				notifyResolved(svc.Name)
			}
		}
	} else {
		st.consecutiveSuccesses = 0
		st.consecutiveFailures++

		// After 5 consecutive failures, create or link to an incident
		if st.consecutiveFailures >= 5 && st.autoIncidentID == 0 {
			// Don't create a new incident if one already exists for this service
			if existing, err := hc.repo.GetActiveIncidentByService(ctx, svc.ID); err == nil && existing != nil {
				st.autoIncidentID = existing.ID
				log.Printf("[checker] linked existing incident #%d for service %s", existing.ID, svc.Name)
				return
			}

			now := time.Now().Format(time.RFC3339)

			inc := &model.Incident{
				ServiceID: svc.ID,
				Title:     fmt.Sprintf("%s 服务异常", svc.Name),
				Impact:    "major",
				Status:    "investigating",
				CreatedAt: now,
				UpdatedAt: now,
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

			st.autoIncidentID = inc.ID
			log.Printf("[checker] auto-created incident #%d for service %s", inc.ID, svc.Name)

			notifyAlert(svc.Name, svc.URL, now)
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
	// Clean up old daily records (90 days)
	beforeDaily := time.Now().AddDate(0, 0, -90).Format("2006-01-02")
	if err := hc.repo.DeleteOldServiceDailies(ctx, beforeDaily); err != nil {
		log.Printf("[checker] cleanup dailies failed: %v", err)
	} else {
		log.Println("[checker] cleaned up daily records older than 90 days")
	}

	// Clean up old heartbeats (7 days)
	beforeHB := time.Now().AddDate(0, 0, -7).UTC().Format("2006-01-02T15:04:05Z")
	if err := hc.repo.DeleteOldHeartbeats(ctx, beforeHB); err != nil {
		log.Printf("[checker] cleanup heartbeats failed: %v", err)
	} else {
		log.Println("[checker] cleaned up heartbeats older than 7 days")
	}
}
