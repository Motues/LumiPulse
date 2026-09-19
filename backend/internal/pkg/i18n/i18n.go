// Package i18n 负责后端生成文案的多语言（RSS / Atom 订阅源与邮件模板）。
//
// 前端文案由前端自己的 i18n 负责；这里只处理无法交给浏览器的部分：
// 订阅源正文、告警邮件、月报邮件，以及检查器自动创建的事件与进展文案。
// 语言取自配置文件（config.LANG），因此同一实例生成的对外内容语言一致。
package i18n

import (
	"fmt"
	"strings"
	"time"
)

// Lang 支持的语言
type Lang string

const (
	ZH Lang = "zh-CN"
	EN Lang = "en-US"
)

// Default 默认语言。项目原有文案全部是中文，保持向后兼容。
const Default = ZH

// Normalize 把任意输入（配置值、Accept-Language 片段）收敛到支持的语言。
// 第二个返回值表示是否识别成功。
func Normalize(raw string) (Lang, bool) {
	lower := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case lower == "":
		return "", false
	case strings.HasPrefix(lower, "zh"):
		return ZH, true
	case strings.HasPrefix(lower, "en"):
		return EN, true
	default:
		return "", false
	}
}

// isZH 判断当前语言是否为中文（其余一律按英文渲染）
func (l Lang) isZH() bool {
	return l != EN
}

// HTMLLang 返回 <html lang> / <language> 使用的标签
func (l Lang) HTMLLang() string {
	if l.isZH() {
		return "zh-CN"
	}
	return "en-US"
}

// --- 事件相关的枚举文案 ---

// StatusText 事件状态
func (l Lang) StatusText(status string) string {
	switch status {
	case "investigating":
		return l.pick("调查中", "Investigating")
	case "identified":
		return l.pick("已确认", "Identified")
	case "monitoring":
		return l.pick("监控中", "Monitoring")
	case "resolved":
		return l.pick("已解决", "Resolved")
	default:
		return status
	}
}

// ImpactText 影响等级
func (l Lang) ImpactText(impact string) string {
	switch impact {
	case "minor":
		return l.pick("轻微", "Minor")
	case "major":
		return l.pick("重大", "Major")
	case "critical":
		return l.pick("严重", "Critical")
	default:
		return impact
	}
}

// ServiceUp / ServiceDown 服务状态
func (l Lang) ServiceUp() string {
	return l.pick("正常", "Operational")
}

func (l Lang) ServiceDown() string {
	return l.pick("异常", "Degraded")
}

// --- 检查器自动生成的文案 ---

// IncidentTitle 自动创建的故障事件标题
func (l Lang) IncidentTitle(service string) string {
	return fmt.Sprintf(l.pick("服务 %s 异常", "%s service incident"), service)
}

// IncidentCreatedNote 事件创建时的进展说明
func (l Lang) IncidentCreatedNote(service string) string {
	return fmt.Sprintf(
		l.pick("检测到 %s 服务连续异常，正在排查中", "Continuous failures detected on %s, investigating"),
		service,
	)
}

// IncidentResolvedNote 事件恢复时的进展说明
func (l Lang) IncidentResolvedNote(service string) string {
	return fmt.Sprintf(l.pick("%s 服务已恢复运行", "%s is back to normal"), service)
}

// AlertSubject / AlertBody 服务异常通知
func (l Lang) AlertSubject(service string) string {
	return fmt.Sprintf(l.pick("服务异常告警: %s", "Service alert: %s"), service)
}

func (l Lang) AlertBody(service, url, ts string) string {
	if l.isZH() {
		return fmt.Sprintf("服务 %s (%s) 连续检测失败，已自动创建故障事件。\n\n检测时间: %s", service, url, ts)
	}
	return fmt.Sprintf(
		"Service %s (%s) failed consecutive probes and an incident was created automatically.\n\nDetected at: %s",
		service, url, ts,
	)
}

// ResolvedSubject / ResolvedBody 服务恢复通知
func (l Lang) ResolvedSubject(service string) string {
	return fmt.Sprintf(l.pick("服务恢复通知: %s", "Service recovered: %s"), service)
}

// EscalationSubject / EscalationBody 告警升级通知（事件长时间未被确认）
func (l Lang) EscalationSubject(title string) string {
	return fmt.Sprintf(l.pick("【未确认】告警升级: %s", "[Unacknowledged] Escalation: %s"), title)
}

// EscalationBody 升级通知正文。unhandled 为可读的未处理时长，count 为第几次升级。
func (l Lang) EscalationBody(title, unhandled string, count int) string {
	if l.isZH() {
		return fmt.Sprintf(
			"故障事件「%s」已持续 %s 仍未被确认，特此升级提醒（第 %d 次），请尽快处理。",
			title, unhandled, count,
		)
	}
	return fmt.Sprintf(
		"Incident \"%s\" has been unacknowledged for %s. Escalation #%d — please take action.",
		title, unhandled, count,
	)
}

// IncidentAcknowledgedNote 人工确认事件时的进展说明
func (l Lang) IncidentAcknowledgedNote(by string) string {
	if by == "" {
		return l.pick("事件已被确认，正在处理中", "Incident acknowledged, handling in progress")
	}
	return fmt.Sprintf(l.pick("事件已被 %s 确认，正在处理中", "Incident acknowledged by %s"), by)
}

// IncidentUnacknowledgedNote 取消确认时的进展说明
func (l Lang) IncidentUnacknowledgedNote() string {
	return l.pick("事件已取消确认，恢复为待处理状态", "Incident acknowledgement removed, back to unhandled")
}

// ResolvedBody 恢复通知正文。duration 为可读时长（例如「3 分钟」），
// 为空时退回不含时长的文案，保持向后兼容。
func (l Lang) ResolvedBody(service, duration string) string {
	if duration == "" {
		return fmt.Sprintf(l.pick("服务 %s 已恢复运行。", "Service %s is back to normal."), service)
	}
	if l.isZH() {
		return fmt.Sprintf("服务 %s 已恢复运行，本次不可用时长约 %s。", service, duration)
	}
	return fmt.Sprintf("Service %s is back to normal. Downtime: %s.", service, duration)
}

// MaintenanceUpcomingSubject / MaintenanceUpcomingBody 维护计划即将开始提醒
func (l Lang) MaintenanceUpcomingSubject(title string) string {
	return fmt.Sprintf(l.pick("维护提醒: %s", "Maintenance reminder: %s"), title)
}

// MaintenanceUpcomingBody 维护提醒正文。start 为可读的开始时间，countdown 为可读的剩余时长。
func (l Lang) MaintenanceUpcomingBody(title, start, countdown string) string {
	if l.isZH() {
		return fmt.Sprintf("维护计划「%s」将于 %s 开始（约 %s 后），请提前做好准备。", title, start, countdown)
	}
	return fmt.Sprintf("Maintenance \"%s\" starts at %s (in about %s). Please prepare in advance.", title, start, countdown)
}

// CertExpiringSubject / CertExpiringBody HTTPS 证书即将到期提醒
func (l Lang) CertExpiringSubject(service string) string {
	return fmt.Sprintf(l.pick("证书即将到期: %s", "Certificate expiring: %s"), service)
}

// CertExpiringBody 证书提醒正文。expiresAt 为可读的到期时间，remaining 为可读的剩余时长
// （已过期时传入 CertExpired()）。
func (l Lang) CertExpiringBody(service, expiresAt, remaining string) string {
	if l.isZH() {
		return fmt.Sprintf("服务 %s 的 HTTPS 证书将于 %s 到期（%s），请及时续期。", service, expiresAt, remaining)
	}
	return fmt.Sprintf("The HTTPS certificate of %s expires at %s (%s). Please renew it soon.", service, expiresAt, remaining)
}

// CertExpired 证书已过期的剩余时长文案
func (l Lang) CertExpired() string {
	return l.pick("已过期", "expired")
}

// --- 邮件与订阅源框架文案 ---

// EmailFooter 邮件页脚
func (l Lang) EmailFooter() string {
	return l.pick("LumiPulse &mdash; 服务监控系统", "LumiPulse &mdash; Service monitoring")
}

// EmailBrand 邮件抬头品牌名
func (l Lang) EmailBrand() string {
	return "LumiPulse"
}

// UnsubscribeHint / UnsubscribeAction 订阅邮件底部的退订区块文案
func (l Lang) UnsubscribeHint() string {
	return l.pick("您收到这封邮件是因为订阅了本状态页的服务通知。", "You are receiving this email because you subscribed to status updates.")
}

func (l Lang) UnsubscribeAction() string {
	return l.pick("退订或修改订阅偏好", "Unsubscribe or manage preferences")
}

// ConfirmUnsubscribed 退订成功后的提示文案
func (l Lang) ConfirmUnsubscribed() string {
	return l.pick("已退订，您不会再收到状态通知。", "You have been unsubscribed and will no longer receive status notifications.")
}

// ConfirmPreferencesSaved 偏好保存成功后的提示文案
func (l Lang) ConfirmPreferencesSaved() string {
	return l.pick("订阅偏好已更新。", "Your subscription preferences have been updated.")
}

// FeedTitle 订阅源标题
func (l Lang) FeedTitle(siteName string) string {
	return fmt.Sprintf(l.pick("%s - 状态更新", "%s - Status updates"), siteName)
}

// FeedDescription 订阅源描述
func (l Lang) FeedDescription(siteName string) string {
	if l.isZH() {
		return fmt.Sprintf("%s 系统状态与故障事件更新", siteName)
	}
	return fmt.Sprintf("Status and incident updates for %s", siteName)
}

// FeedStatusLabel / FeedImpactLabel 订阅源条目里的字段名
func (l Lang) FeedStatusLabel() string {
	return l.pick("状态", "Status")
}

func (l Lang) FeedImpactLabel() string {
	return l.pick("影响", "Impact")
}

// TestEmailSubject 测试邮件主题
func (l Lang) TestEmailSubject() string {
	return l.pick("LumiPulse 邮件通知", "LumiPulse email notification")
}

// TestEmailBody 测试邮件正文（HTML 片段）
func (l Lang) TestEmailBody() string {
	if l.isZH() {
		return "这是一封来自 LumiPulse 的测试邮件。<br><br>如果收到此邮件，说明您的 SMTP 配置正确。"
	}
	return "This is a test email from LumiPulse.<br><br>Receiving it means your SMTP configuration works."
}

// TestWebhookService / TestWebhookMessage 测试 webhook 的载荷文案
func (l Lang) TestWebhookService() string {
	return l.pick("LumiPulse 测试通知", "LumiPulse test notification")
}

func (l Lang) TestWebhookMessage() string {
	return l.pick(
		"这是一条来自 LumiPulse 的测试 webhook，收到即表示地址配置正确。",
		"This is a test webhook from LumiPulse. Receiving it means the endpoint is configured correctly.",
	)
}

// --- 月度 SLA 报告 ---

// MonthLabel 把 YYYY-MM 渲染成展示用月份
func (l Lang) MonthLabel(month string) string {
	parts := strings.SplitN(month, "-", 2)
	if len(parts) != 2 {
		return month
	}
	if l.isZH() {
		return fmt.Sprintf("%s 年 %s 月", parts[0], parts[1])
	}
	if t, err := time.Parse("2006-01", month); err == nil {
		return t.Format("January 2006")
	}
	return month
}

// NoData 表示该服务当月没有数据
func (l Lang) NoData() string {
	return l.pick("无数据", "No data")
}

// Duration 把秒数渲染成可读时长
func (l Lang) Duration(seconds int) string {
	if seconds <= 0 {
		return "—"
	}
	if seconds < 60 {
		return fmt.Sprintf(l.pick("%d 秒", "%ds"), seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf(l.pick("%d 分钟", "%dm"), seconds/60)
	}
	hours := float64(seconds) / 3600
	if hours < 24 {
		return fmt.Sprintf(l.pick("%.1f 小时", "%.1fh"), hours)
	}
	return fmt.Sprintf(l.pick("%.1f 天", "%.1fd"), hours/24)
}

// 月报邮件中的标签文案
func (l Lang) SLATitle() string {
	return l.pick("月度 SLA 报告", "Monthly SLA report")
}

func (l Lang) SLASubtitle() string {
	return l.pick("整站可用率与故障汇总", "Site-wide availability and incident summary")
}

func (l Lang) SLAOverallUptime() string {
	return l.pick("整站可用率", "Overall uptime")
}

func (l Lang) SLAIncidents() string {
	return l.pick("故障事件", "Incidents")
}

func (l Lang) SLADowntime() string {
	return l.pick("不可用时长", "Downtime")
}

func (l Lang) SLAAvgLatency() string {
	return l.pick("平均响应", "Avg response")
}

func (l Lang) SLAProbes() string {
	return l.pick("探测次数", "Probes")
}

func (l Lang) SLAFailedProbes() string {
	return l.pick("失败探测", "Failed probes")
}

func (l Lang) SLAServiceTitle() string {
	return l.pick("服务可用率明细", "Per-service uptime")
}

func (l Lang) SLAColService() string {
	return l.pick("服务", "Service")
}

func (l Lang) SLALatencyUnit() string {
	return "ms"
}

// SLAIntro 邮件开头的一句话
func (l Lang) SLAIntro(month string) string {
	if l.isZH() {
		return fmt.Sprintf("以下是 %s 的服务可用率汇总。", l.MonthLabel(month))
	}
	return fmt.Sprintf("Here is the availability summary for %s.", l.MonthLabel(month))
}

// SLAFooterNote 邮件结尾说明
func (l Lang) SLAFooterNote() string {
	return l.pick(
		"统计口径：可用率按探测次数加权；每次探测按服务自己的成功判定统计（默认 200 ≤ status &lt; 400 或 TCP 连通，服务可自定义期望状态码与响应关键字）。",
		"Methodology: uptime is weighted by probe count; each probe is judged by the service's own criteria (by default 200 &le; status &lt; 400 or a successful TCP check; services may define expected status codes and response keywords).",
	)
}

// --- 周报摘要（复用月报的版式与统计口径）---

// WeeklyTitle 周报标题
func (l Lang) WeeklyTitle() string {
	return l.pick("周报摘要", "Weekly summary")
}

// dayLabel 把 YYYY-MM-DD 渲染成展示用日期
func (l Lang) dayLabel(day string) string {
	t, err := time.Parse("2006-01-02", day)
	if err != nil {
		return day
	}
	if l.isZH() {
		return t.Format("2006 年 01 月 02 日")
	}
	return t.Format("Jan 2, 2006")
}

// DateRangeLabel 把统计区间渲染成展示用文案（含首尾两天）
func (l Lang) DateRangeLabel(from, to string) string {
	return fmt.Sprintf("%s - %s", l.dayLabel(from), l.dayLabel(to))
}

// ShortDateRange 统计区间的紧凑写法，用于汇总卡片里的小字
func (l Lang) ShortDateRange(from, to string) string {
	f, errF := time.Parse("2006-01-02", from)
	t, errT := time.Parse("2006-01-02", to)
	if errF != nil || errT != nil {
		return fmt.Sprintf("%s - %s", from, to)
	}
	if l.isZH() {
		return fmt.Sprintf("%s ~ %s", f.Format("01-02"), t.Format("01-02"))
	}
	return fmt.Sprintf("%s - %s", f.Format("Jan 2"), t.Format("2"))
}

// WeeklyIntro 周报开头的一句话
func (l Lang) WeeklyIntro(from, to string) string {
	if l.isZH() {
		return fmt.Sprintf("以下是 %s 至 %s 的服务可用率汇总。", l.dayLabel(from), l.dayLabel(to))
	}
	return fmt.Sprintf("Here is the availability summary for %s - %s.", l.dayLabel(from), l.dayLabel(to))
}

// SLACompareLabel 环比上一周期的前缀文案
func (l Lang) SLACompareLabel() string {
	return l.pick("较上周：", "vs last week: ")
}

// pick 按语言二选一
func (l Lang) pick(zh, en string) string {
	if l.isZH() {
		return zh
	}
	return en
}
