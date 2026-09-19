package checker

import (
	"context"
	"strconv"
	"strings"
	"time"

	"lumipluse-backend/internal/model"
	"lumipluse-backend/internal/pkg/utils"
)

// 告警升级相关设置项（可在「通知管理 → 告警升级」配置，读取走内存缓存，改完立即生效）。
const (
	// settingEscalationEnabled 是否启用告警升级，默认关闭。
	settingEscalationEnabled = "alert_escalation_enabled"
	// settingEscalationMinutes 事件创建后多久仍未确认则升级通知（分钟）。
	settingEscalationMinutes = "alert_escalation_minutes"
	// settingEscalationRepeatMinutes 两次升级之间的最小间隔（分钟）。
	// 留空或 0 表示只升级一次。
	settingEscalationRepeatMinutes = "alert_escalation_repeat_minutes"
)

const (
	defaultEscalationMinutes = 15
	// maxEscalationMinutes 升级时限上限（7 天），避免误填一个大数字后形同关闭
	maxEscalationMinutes = 7 * 24 * 60
	// escalationScanInterval 扫描未确认事件的节流间隔。
	// 升级时限以分钟计，没必要每个调度心跳都查一次数据库。
	escalationScanInterval = 30 * time.Second
)

// escalationPolicy 一次升级判定所用的策略快照
type escalationPolicy struct {
	enabled bool
	// after 事件创建后多久仍未确认就升级
	after time.Duration
	// repeat 两次升级之间的最小间隔；0 表示只升级一次
	repeat time.Duration
}

// loadEscalationPolicy 从设置里读取升级策略
func loadEscalationPolicy() escalationPolicy {
	var p escalationPolicy
	if strings.EqualFold(strings.TrimSpace(utils.GetSetting(settingEscalationEnabled)), "true") {
		p.enabled = true
	}

	minutes := defaultEscalationMinutes
	if v, err := strconv.Atoi(strings.TrimSpace(utils.GetSetting(settingEscalationMinutes))); err == nil && v > 0 {
		minutes = v
	}
	if minutes > maxEscalationMinutes {
		minutes = maxEscalationMinutes
	}
	p.after = time.Duration(minutes) * time.Minute

	if v, err := strconv.Atoi(strings.TrimSpace(utils.GetSetting(settingEscalationRepeatMinutes))); err == nil && v > 0 {
		if v > maxEscalationMinutes {
			v = maxEscalationMinutes
		}
		p.repeat = time.Duration(v) * time.Minute
	}
	return p
}

// escalateIncidents 扫描未确认的活跃事件，对超过升级时限的再次发送通知。
// 由调度循环节流调用；任何失败都只记日志，绝不影响探测主流程。
func (hc *HealthChecker) escalateIncidents(ctx context.Context) {
	policy := loadEscalationPolicy()
	if !policy.enabled {
		return
	}

	incidents, err := hc.repo.ListActiveIncidents(ctx)
	if err != nil {
		utils.Info("checker escalation list incidents error: %v", err)
		return
	}

	now := time.Now()
	for _, inc := range incidents {
		// 合并事件的子事件不单独升级：父事件的通知里已经覆盖
		if inc.ParentID != nil {
			continue
		}
		if !hc.shouldEscalate(ctx, inc, policy, now) {
			continue
		}
		hc.sendEscalation(ctx, inc, now)
	}
}

// shouldEscalate 判断这个事件此刻是否应当升级通知。
// 同时把「本次升级」的计数与时间写回事件，保证不会重复轰炸。
func (hc *HealthChecker) shouldEscalate(ctx context.Context, inc *model.Incident, policy escalationPolicy, now time.Time) bool {
	created := parseTime(inc.CreatedAt)
	unhandled := now.Sub(created)
	if unhandled < policy.after {
		return false
	}

	if inc.EscalationCount > 0 {
		// repeat 为 0 表示只升级一次，此后不再打扰
		if policy.repeat <= 0 {
			return false
		}
		// 空值（理论上不会出现：计数 > 0 时一定有记录）按「刚升级过」处理，避免立刻重发
		if strings.TrimSpace(inc.LastEscalatedAt) == "" {
			return false
		}
		if sinceLast := now.Sub(parseTime(inc.LastEscalatedAt)); sinceLast < policy.repeat {
			return false
		}
	}

	inc.EscalationCount++
	inc.LastEscalatedAt = now.UTC().Format(time.RFC3339)
	if err := hc.repo.UpdateIncident(ctx, inc); err != nil {
		utils.Info("checker failed to record escalation for incident #%d: %v", inc.ID, err)
		// 记录失败时依然继续通知：宁可重复一次，也不要静默漏掉升级
	}
	return true
}

// sendEscalation 发送一次升级通知（webhook + 邮件 + 订阅者）。
func (hc *HealthChecker) sendEscalation(ctx context.Context, inc *model.Incident, now time.Time) {
	svc := hc.loadService(ctx, inc)
	unhandledSeconds := int(now.Sub(parseTime(inc.CreatedAt)).Seconds())
	if unhandledSeconds < 0 {
		unhandledSeconds = 0
	}

	subject, body := escalationMessage(inc, svc, unhandledSeconds)

	notifyWebhook(utils.WebhookEvent{
		Event:   utils.WebhookEventDown,
		Service: incidentServiceName(inc, svc),
		URL:     incidentServiceURL(svc),
		Time:    now.Format(time.RFC3339),
		Message: body,
	})

	utils.Info("checker escalated incident #%d (%s), escalation #%d", inc.ID, inc.Title, inc.EscalationCount)

	// critical 事件穿透免打扰，其余按免打扰时段拦下邮件（与首次告警口径一致）
	if mutedByQuietHours(inc.Impact) {
		utils.Info("checker suppressed escalation email for incident #%d (quiet hours)", inc.ID)
		return
	}
	sendAlertMail(subject, body)
	hc.notifySubscribers(ctx, svc, subject, body)
}

// loadService 取事件关联的服务。服务可能已被删除，此时返回占位对象，
// 让通知文案仍然可读，而不是整体静默。
func (hc *HealthChecker) loadService(ctx context.Context, inc *model.Incident) *model.Service {
	if inc == nil {
		return &model.Service{Name: "未知服务"}
	}
	if svc, err := hc.repo.GetService(ctx, inc.ServiceID); err == nil && svc != nil {
		return svc
	}
	return &model.Service{Name: "未知服务", ID: inc.ServiceID}
}

// incidentServiceName 通知里展示的服务名
func incidentServiceName(inc *model.Incident, svc *model.Service) string {
	if svc != nil && strings.TrimSpace(svc.Name) != "" {
		return svc.Name
	}
	if inc != nil && strings.TrimSpace(inc.Title) != "" {
		return inc.Title
	}
	return "未知服务"
}

// incidentServiceURL 通知里的服务地址，服务已删除时为空
func incidentServiceURL(svc *model.Service) string {
	if svc == nil {
		return ""
	}
	return svc.URL
}
