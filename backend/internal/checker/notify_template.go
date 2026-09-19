package checker

import (
	"strconv"
	"strings"
	"time"

	"lumipluse-backend/internal/model"
	"lumipluse-backend/internal/pkg/i18n"
	"lumipluse-backend/internal/pkg/utils"
)

// 通知模板：设置页可自定义每类通知的主题与正文，留空则用内置的中/英文案。
// 邮件与 webhook 共用同一份渲染结果（webhook 用正文作为 message）。
const (
	settingTplAlertSubject       = "tpl_alert_subject"
	settingTplAlertBody          = "tpl_alert_body"
	settingTplResolvedSubject    = "tpl_resolved_subject"
	settingTplResolvedBody       = "tpl_resolved_body"
	settingTplMaintenanceSubject = "tpl_maintenance_subject"
	settingTplMaintenanceBody    = "tpl_maintenance_body"
	settingTplCertSubject        = "tpl_cert_subject"
	settingTplCertBody           = "tpl_cert_body"
	settingTplEscalationSubject  = "tpl_escalation_subject"
	settingTplEscalationBody     = "tpl_escalation_body"
)

// cstTime 模板里的时间统一按北京时间渲染
func cstTime(t time.Time) string {
	return t.In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04")
}

// renderTemplate 选取「自定义模板 / 内置文案」并渲染变量。
// fallback 内部同样可以带 {{var}}，这样管理员只需要覆盖其中一部分文案时，
// 也能把内置文本复制过去再改。
func renderTemplate(settingKey, fallback string, vars map[string]string) string {
	tpl := strings.TrimSpace(utils.GetSetting(settingKey))
	if tpl == "" {
		tpl = fallback
	}
	return i18n.RenderTemplate(tpl, vars)
}

// templateVars 公共变量（站点名）
func templateVars() map[string]string {
	return map[string]string{"site": emailSiteName()}
}

// emailSiteName 邮件里使用的站点名，未配置时退回产品名
func emailSiteName() string {
	if name := strings.TrimSpace(utils.GetSetting("site_name")); name != "" {
		return name
	}
	return "LumiPulse"
}

// alertMessage 服务异常通知的主题与正文
func alertMessage(svc *model.Service, ts string) (subject, body string) {
	lang := i18n.Current()
	vars := templateVars()
	vars["service"] = svc.Name
	vars["url"] = svc.URL
	vars["time"] = ts

	subject = renderTemplate(settingTplAlertSubject, lang.AlertSubject(svc.Name), vars)
	body = renderTemplate(settingTplAlertBody, lang.AlertBody(svc.Name, svc.URL, ts), vars)
	return subject, body
}

// resolvedMessage 服务恢复通知的主题与正文
func resolvedMessage(svc *model.Service, downtimeSeconds int) (subject, body string) {
	lang := i18n.Current()

	durationText := ""
	if downtimeSeconds > 0 {
		durationText = lang.Duration(downtimeSeconds)
	}

	vars := templateVars()
	vars["service"] = svc.Name
	vars["url"] = svc.URL
	vars["duration"] = durationText
	vars["time"] = cstTime(time.Now())

	subject = renderTemplate(settingTplResolvedSubject, lang.ResolvedSubject(svc.Name), vars)
	body = renderTemplate(settingTplResolvedBody, lang.ResolvedBody(svc.Name, durationText), vars)
	return subject, body
}

// maintenanceMessage 维护计划即将开始的提醒
func maintenanceMessage(m *model.Maintenance, start time.Time) (subject, body string) {
	lang := i18n.Current()
	startText := cstTime(start)
	countdown := lang.Duration(int(time.Until(start).Seconds()))

	vars := templateVars()
	vars["title"] = m.Title
	vars["start"] = startText
	vars["countdown"] = countdown

	subject = renderTemplate(settingTplMaintenanceSubject, lang.MaintenanceUpcomingSubject(m.Title), vars)
	body = renderTemplate(settingTplMaintenanceBody, lang.MaintenanceUpcomingBody(m.Title, startText, countdown), vars)
	return subject, body
}

// certMessage HTTPS 证书即将到期的提醒
func certMessage(name string, expires time.Time, remainingDays float64) (subject, body string) {
	lang := i18n.Current()
	expiresText := cstTime(expires)

	remainingText := lang.CertExpired()
	if remainingDays > 0 {
		remainingText = lang.Duration(int(time.Until(expires).Seconds()))
	}

	vars := templateVars()
	vars["service"] = name
	vars["expires"] = expiresText
	vars["remaining"] = remainingText

	subject = renderTemplate(settingTplCertSubject, lang.CertExpiringSubject(name), vars)
	body = renderTemplate(settingTplCertBody, lang.CertExpiringBody(name, expiresText, remainingText), vars)
	return subject, body
}

// escalationMessage 告警升级通知（事件长时间未被确认时再次通知）。
// svc 可能为占位对象（服务已删除），此时用事件标题兜底服务名。
func escalationMessage(inc *model.Incident, svc *model.Service, unhandledSeconds int) (subject, body string) {
	lang := i18n.Current()

	service := incidentServiceName(inc, svc)
	url := incidentServiceURL(svc)
	unhandled := lang.Duration(unhandledSeconds)

	vars := templateVars()
	vars["service"] = service
	vars["title"] = inc.Title
	vars["url"] = url
	vars["unhandled"] = unhandled
	vars["count"] = strconv.Itoa(inc.EscalationCount)
	vars["time"] = cstTime(time.Now())

	subject = renderTemplate(settingTplEscalationSubject, lang.EscalationSubject(inc.Title), vars)
	body = renderTemplate(settingTplEscalationBody, lang.EscalationBody(inc.Title, unhandled, inc.EscalationCount), vars)
	return subject, body
}
