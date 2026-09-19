package http

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"lumipluse-backend/internal/model"
	"lumipluse-backend/internal/pkg/i18n"
	"lumipluse-backend/internal/pkg/utils"
)

// 周报相关设置项（后台「通知管理」页配置，读取走设置缓存）。
const (
	settingWeeklyEnabled = "weekly_report_enabled"
	settingWeeklyEmails  = "weekly_report_emails"
	// settingWeeklyWeekday 发送日：1=周一 … 7=周日（默认 1）
	settingWeeklyWeekday = "weekly_report_weekday"
	// settingWeeklyLastSent 已经发送过的窗口结束日期（YYYY-MM-DD），用于去重
	settingWeeklyLastSent = "weekly_report_last_sent"
)

// weeklyReportPeriod 周报固定统计「过去 7 天」：结束日期为发送当天（不含），起始为 7 天前。
const weeklyReportPeriod = 7

// weeklySendWeekday 解析发送日配置，返回 1~7（周一~周日），无效时退回周一。
func weeklySendWeekday() int {
	day, err := strconv.Atoi(strings.TrimSpace(utils.GetSetting(settingWeeklyWeekday)))
	if err != nil || day < 1 || day > 7 {
		return 1
	}
	return day
}

// isoWeekday 返回 time.Weekday 对应的 1~7（周一=1 … 周日=7）
func isoWeekday(t time.Time) int {
	wd := int(t.Weekday())
	if wd == 0 {
		return 7
	}
	return wd
}

// weeklyWindow 返回本次周报的统计窗口 [from, toExclusive)。
// 固定为「过去 7 天」：toExclusive = 当天 00:00，from = 7 天前 00:00。
func weeklyWindow(now time.Time) (from, toExclusive time.Time) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return today.AddDate(0, 0, -weeklyReportPeriod), today
}

// buildSLAReportForRange 汇总任意日期区间的可用率与事件，口径与月报完全一致
// （逐服务装配复用 assembleSLAReport）。
func (h *Handler) buildSLAReportForRange(ctx context.Context, from, toExclusive time.Time, label string) (*model.MonthlySLAReport, error) {
	services, err := h.Repo.ListServices(ctx)
	if err != nil {
		return nil, err
	}

	fromStr := from.Format("2006-01-02")
	toStr := toExclusive.Format("2006-01-02")

	aggregated, err := h.Repo.AggregateServiceMonthly(ctx, fromStr, toStr)
	if err != nil {
		return nil, err
	}
	incidentCounts, err := h.Repo.CountIncidentsBetween(ctx, fromStr, toStr)
	if err != nil {
		incidentCounts = nil
	}
	incidentDowntime, err := h.Repo.AggregateIncidentDowntime(ctx, fromStr, toStr)
	if err != nil {
		incidentDowntime = nil
	}
	coveredDays, err := h.Repo.CountCoveredDaysBetween(ctx, fromStr, toStr)
	if err != nil {
		coveredDays = nil
	}

	byService := make(map[int64]*model.ServiceMonthly, len(aggregated))
	for _, m := range aggregated {
		m.Month = fromStr
		m.IncidentTotal = incidentCounts[m.ServiceID]
		m.IncidentDowntimeSecs = incidentDowntime[m.ServiceID]
		byService[m.ServiceID] = m
	}

	days := int(toExclusive.Sub(from).Hours()/24 + 0.5)
	if days <= 0 {
		days = weeklyReportPeriod
	}

	report := &model.MonthlySLAReport{
		Month: fromStr,
		Label: label,
		Days:  days,
		Final: true, // 窗口已经结束，数据不会再变
	}

	return assembleSLAReport(services, byService, coveredDays, report), nil
}

// SendWeeklyReportIfDue 到了配置的星期几时，发送「过去 7 天」的周报摘要。
//
// 触发条件全部自检，因此可以由检查器的每日任务无条件调用：
// 未启用、未配置收件人、今天不是发送日、本次窗口已发送过、窗口内无数据时都会直接跳过。
func (h *Handler) SendWeeklyReportIfDue(ctx context.Context, now time.Time) {
	if utils.GetSetting(settingWeeklyEnabled) != "true" {
		return
	}

	if isoWeekday(now) != weeklySendWeekday() {
		return
	}

	// 收件人默认复用告警邮箱，也可单独配置
	recipients := strings.TrimSpace(utils.GetSetting(settingWeeklyEmails))
	if recipients == "" {
		recipients = strings.TrimSpace(utils.GetSetting("notify_emails"))
	}
	if recipients == "" {
		utils.Info("weekly report enabled but no recipients configured, skipping")
		return
	}

	from, toExclusive := weeklyWindow(now)
	// 以窗口结束日期作为去重键：同一天内多次触发（进程重启）只发一次
	windowKey := toExclusive.Format("2006-01-02")
	if utils.GetSetting(settingWeeklyLastSent) == windowKey {
		return
	}

	lastDay := toExclusive.AddDate(0, 0, -1)
	label := i18n.Resolve(utils.GetSetting("sla_report_language")).DateRangeLabel(
		from.Format("2006-01-02"), lastDay.Format("2006-01-02"))

	report, err := h.buildSLAReportForRange(ctx, from, toExclusive, label)
	if err != nil {
		utils.Info("weekly report build failed for %s: %v", windowKey, err)
		return
	}
	if report.Summary.TotalProbes == 0 {
		utils.Info("weekly report for %s..%s has no data, skipping", from.Format("2006-01-02"), lastDay.Format("2006-01-02"))
		return
	}

	// 上一周期用于环比，取不到就退化为不展示环比
	prevFrom := from.AddDate(0, 0, -weeklyReportPeriod)
	previous, err := h.buildSLAReportForRange(ctx, prevFrom, from, "")
	if err != nil {
		previous = nil
	}

	lang := i18n.Resolve(utils.GetSetting("sla_report_language"))
	subject, body := buildWeeklyEmail(report, previous, lang,
		from.Format("2006-01-02"), lastDay.Format("2006-01-02"))

	var errs []string
	for _, to := range strings.Split(recipients, ",") {
		to = strings.TrimSpace(to)
		if to == "" {
			continue
		}
		if err := utils.SendHTMLMail(to, subject, body); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", to, err))
		}
	}
	if len(errs) > 0 {
		utils.Info("weekly report send failed: %s", strings.Join(errs, "; "))
		// 发送失败不记录已发送标记，下个清理周期会自动重试
		return
	}

	if err := utils.SetSetting(settingWeeklyLastSent, windowKey); err != nil {
		utils.Info("failed to record weekly report window: %v", err)
	}
	utils.Info("weekly report for %s sent to %s", label, recipients)
}

// SendScheduledReportsIfDue 检查器的每日任务入口：按周期发送月报与周报。
// 两者各自自检触发条件，因此可以无条件调用。
func (h *Handler) SendScheduledReportsIfDue(ctx context.Context, now time.Time) {
	h.SendMonthlySLAReportIfDue(ctx, now)
	h.SendWeeklyReportIfDue(ctx, now)
}
