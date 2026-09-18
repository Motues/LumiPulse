package http

import (
	"context"
	"fmt"
	"strings"
	"time"

	"lumipluse-backend/internal/model"
	"lumipluse-backend/internal/pkg/i18n"
	"lumipluse-backend/internal/pkg/utils"
)

// SLAMonthlyEmailData 月报邮件的渲染数据。
// 把聚合结果先拍平成模板友好的结构，模板里就不用再做条件判断。
type SLAMonthlyEmailData struct {
	Lang       i18n.Lang
	Month      string
	MonthLabel string
	Intro      string
	FooterNote string
	SiteName   string
	Final      bool

	OverallUptime  string
	TotalProbes    int
	DowntimeProbes int
	Incidents      int
	DowntimeText   string
	AvgLatency     string
	HasData        bool

	Services []SLAMonthlyEmailService
}

// SLAMonthlyEmailService 月报邮件里的单个服务行
type SLAMonthlyEmailService struct {
	Name         string
	Uptime       string
	UptimeColor  string
	Probes       string
	Failures     string
	Incidents    string
	DowntimeText string
	AvgLatency   string
	CoveredDays  string
}

// buildSLAMonthlyEmail 依据月报生成邮件主题与 HTML 正文。
func buildSLAMonthlyEmail(report *model.MonthlySLAReport, lang i18n.Lang) (subject, body string) {
	siteName := strings.TrimSpace(utils.GetSetting("site_name"))
	if siteName == "" {
		siteName = "LumiPulse"
	}

	data := SLAMonthlyEmailData{
		Lang:       lang,
		Month:      report.Month,
		MonthLabel: lang.MonthLabel(report.Month),
		Intro:      lang.SLAIntro(report.Month),
		FooterNote: lang.SLAFooterNote(),
		SiteName:   siteName,
		Final:      report.Final,
	}

	data.HasData = report.Summary.TotalProbes > 0
	data.TotalProbes = report.Summary.TotalProbes
	data.DowntimeProbes = report.Summary.DowntimeProbes
	data.Incidents = report.Summary.Incidents
	data.DowntimeText = lang.Duration(report.Summary.DowntimeSeconds)
	if data.HasData {
		data.OverallUptime = fmt.Sprintf("%.3f%%", report.Summary.Uptime)
		data.AvgLatency = fmt.Sprintf("%.0f%s", report.Summary.AvgLatency, lang.SLALatencyUnit())
	} else {
		data.OverallUptime = lang.NoData()
		data.AvgLatency = "—"
	}

	for _, svc := range report.Services {
		row := SLAMonthlyEmailService{
			Name:         svc.Name,
			Probes:       fmt.Sprintf("%d", svc.TotalProbes),
			Failures:     fmt.Sprintf("%d", svc.DowntimeProbes),
			Incidents:    fmt.Sprintf("%d", svc.Incidents),
			DowntimeText: lang.Duration(svc.DowntimeSeconds),
			CoveredDays:  fmt.Sprintf("%d/%d", svc.CoveredDays, report.Days),
		}
		if svc.CoveredDays > 0 && svc.TotalProbes > 0 {
			row.Uptime = fmt.Sprintf("%.3f%%", svc.Uptime)
			row.UptimeColor = uptimeColor(svc.Uptime)
			row.AvgLatency = fmt.Sprintf("%.0f%s", svc.AvgLatency, lang.SLALatencyUnit())
		} else {
			row.Uptime = lang.NoData()
			row.UptimeColor = "#6b7280"
			row.AvgLatency = "—"
		}
		data.Services = append(data.Services, row)
	}

	return slaEmailSubject(siteName, data), renderSLAMonthlyEmail(data)
}

// slaEmailSubject 邮件主题，中英文的括号习惯不同
func slaEmailSubject(siteName string, d SLAMonthlyEmailData) string {
	if d.Lang == i18n.ZH {
		return fmt.Sprintf("%s - %s（%s）", siteName, d.Lang.SLATitle(), d.MonthLabel)
	}
	return fmt.Sprintf("%s - %s (%s)", siteName, d.Lang.SLATitle(), d.MonthLabel)
}

// uptimeColor 可用率对应的强调色，与前端阈值保持一致
func uptimeColor(uptime float64) string {
	switch {
	case uptime >= 99.9:
		return "#34a761"
	case uptime >= 99:
		return "#fda305"
	default:
		return "#df2d2a"
	}
}

// renderSLAMonthlyEmail 渲染 HTML 邮件。
// 用 HTML 表格而不是 CSS flex，邮件客户端的兼容性最好。
//
// 注意：格式串里的 %% 是字面百分号，转义后的 Sprintf 参数顺序与下方
// args 变量一一对应，新增字段时请同时更新两处。
func renderSLAMonthlyEmail(d SLAMonthlyEmailData) string {
	var rows strings.Builder
	for _, s := range d.Services {
		rows.WriteString(fmt.Sprintf(`<tr>
  <td style="padding:10px 12px;border-top:1px solid #eee;font-weight:600;color:#1a1a2e">%s</td>
  <td style="padding:10px 12px;border-top:1px solid #eee;text-align:right;font-weight:600;color:%s">%s</td>
  <td style="padding:10px 12px;border-top:1px solid #eee;text-align:right;color:#555">%s</td>
  <td style="padding:10px 12px;border-top:1px solid #eee;text-align:right;color:#555">%s</td>
  <td style="padding:10px 12px;border-top:1px solid #eee;text-align:right;color:#555">%s</td>
  <td style="padding:10px 12px;border-top:1px solid #eee;text-align:right;color:#555">%s</td>
  <td style="padding:10px 12px;border-top:1px solid #eee;text-align:right;color:#999;font-size:12px">%s</td>
</tr>`, s.Name, s.UptimeColor, s.Uptime, s.Probes, s.Failures, s.Incidents, s.DowntimeText, s.CoveredDays))
	}

	var body strings.Builder
	body.WriteString(`<!DOCTYPE html>
<html lang="` + d.Lang.HTMLLang() + `">
<head><meta charset="utf-8"></head>
<body style="margin:0;padding:0;background:#f5f5f5;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">
  <table width="100%" cellpadding="0" cellspacing="0" style="padding:40px 16px">
    <tr><td align="center">
      <table width="640" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:12px;overflow:hidden;box-shadow:0 1px 3px rgba(0,0,0,.08)">
        <tr><td style="padding:32px 32px 0">
          <div style="font-size:20px;font-weight:700;color:#1a1a2e">` + d.SiteName + `</div>
          <div style="font-size:13px;color:#999;margin-top:4px">` + d.MonthLabel + ` &middot; ` + d.Lang.SLATitle() + `</div>
        </td></tr>
        <tr><td style="padding:16px 32px 0;font-size:14px;line-height:1.7;color:#555">` + d.Intro + `</td></tr>
        <tr><td style="padding:16px 32px 24px">
          <table width="100%" cellpadding="0" cellspacing="0" style="border-collapse:collapse"><tr>`)

	// 四张汇总卡片
	cards := []struct {
		label string
		value string
		sub   string
	}{
		{d.Lang.SLAOverallUptime(), d.OverallUptime, fmt.Sprintf("%s %d", d.Lang.SLAProbes(), d.TotalProbes)},
		{d.Lang.SLAIncidents(), fmt.Sprintf("%d", d.Incidents), fmt.Sprintf("%s %d", d.Lang.SLAFailedProbes(), d.DowntimeProbes)},
		{d.Lang.SLADowntime(), d.DowntimeText, ""},
		{d.Lang.SLAAvgLatency(), d.AvgLatency, d.MonthLabel},
	}
	for _, c := range cards {
		body.WriteString(`<td width="25%" style="padding:12px;background:#fafafa;border-radius:8px;vertical-align:top">`)
		body.WriteString(`<div style="font-size:12px;color:#999">` + c.label + `</div>`)
		body.WriteString(`<div style="font-size:18px;font-weight:700;color:#1a1a2e;margin-top:4px">` + c.value + `</div>`)
		if c.sub != "" {
			body.WriteString(`<div style="font-size:11px;color:#bbb;margin-top:2px">` + c.sub + `</div>`)
		}
		body.WriteString(`</td>`)
	}

	body.WriteString(`</tr></table>
        </td></tr>
        <tr><td style="padding:0 32px 8px">
          <div style="font-size:14px;font-weight:600;color:#1a1a2e;margin-bottom:8px">` + d.Lang.SLAServiceTitle() + `</div>
        </td></tr>
        <tr><td style="padding:0 32px 24px">
          <table width="100%" cellpadding="0" cellspacing="0" style="border-collapse:collapse;font-size:13px">
            <tr style="color:#999;font-size:12px">
              <th align="left" style="padding:6px 12px;font-weight:500">` + d.Lang.SLAColService() + `</th>
              <th align="right" style="padding:6px 12px;font-weight:500">` + d.Lang.SLAOverallUptime() + `</th>
              <th align="right" style="padding:6px 12px;font-weight:500">` + d.Lang.SLAProbes() + `</th>
              <th align="right" style="padding:6px 12px;font-weight:500">` + d.Lang.SLAFailedProbes() + `</th>
              <th align="right" style="padding:6px 12px;font-weight:500">` + d.Lang.SLAIncidents() + `</th>
              <th align="right" style="padding:6px 12px;font-weight:500">` + d.Lang.SLADowntime() + `</th>
              <th align="right" style="padding:6px 12px;font-weight:500">` + d.Lang.SLAAvgLatency() + `</th>
            </tr>`)
	body.WriteString(rows.String())
	body.WriteString(`
          </table>
        </td></tr>
        <tr><td style="padding:16px 32px;border-top:1px solid #eee;font-size:12px;color:#999;line-height:1.6">`)
	body.WriteString(d.FooterNote)
	body.WriteString(`<br>`)
	body.WriteString(d.Lang.EmailFooter())
	body.WriteString(`
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`)

	return body.String()
}

// SendMonthlySLAReportIfDue 在检测到「进入新月份」时发送上一个自然月的 SLA 月报。
//
// 触发条件全部自检，因此可以由检查器的每日任务无条件调用：
// 未启用、未配置收件人、本月已发送过、上月无数据时都会直接跳过。
func (h *Handler) SendMonthlySLAReportIfDue(ctx context.Context, now time.Time) {
	if utils.GetSetting("sla_report_enabled") != "true" {
		return
	}

	// 收件人默认复用告警邮箱，也可单独配置
	recipients := strings.TrimSpace(utils.GetSetting("sla_report_emails"))
	if recipients == "" {
		recipients = strings.TrimSpace(utils.GetSetting("notify_emails"))
	}
	if recipients == "" {
		utils.Info("sla report enabled but no recipients configured, skipping")
		return
	}

	currentMonth := now.Format("2006-01")
	if utils.GetSetting("last_sla_report_month") == currentMonth {
		return // 本月已经发过
	}

	lastMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
	report, err := h.buildMonthlySLACtx(ctx, lastMonth)
	if err != nil {
		utils.Info("sla report build failed for %s: %v", lastMonth.Format("2006-01"), err)
		return
	}
	if report.Summary.TotalProbes == 0 {
		utils.Info("sla report for %s has no data, skipping", report.Month)
		return
	}

	lang := i18n.Resolve(utils.GetSetting("sla_report_language"))
	subject, body := buildSLAMonthlyEmail(report, lang)

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
		utils.Info("sla report send failed: %s", strings.Join(errs, "; "))
		// 发送失败不记录已发送标记，下个清理周期会自动重试
		return
	}

	// 记录已发送月份，避免重复投递
	if err := utils.SetSetting("last_sla_report_month", currentMonth); err != nil {
		utils.Info("failed to record sla report month: %v", err)
	}
	utils.Info("sla report for %s sent to %s", report.Month, recipients)
}
