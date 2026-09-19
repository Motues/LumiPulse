package http

import (
	"lumipluse-backend/internal/model"
	"lumipluse-backend/internal/pkg/i18n"
	"lumipluse-backend/internal/pkg/utils"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Login 管理员登录
func (h *Handler) Login(c *gin.Context) {
	ip := utils.GetClientIP(c)

	if utils.Limiter.IsIPBlocked(ip) {
		utils.Warn("Blocked IP attempted to login: %s", ip)
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "IP is blocked due to multiple failed attempts"})
		return
	}

	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request body"})
		return
	}

	if !utils.CheckAdminCredentials(req.Username, req.Password) {
		utils.Limiter.RecordAttempt(ip)
		utils.Warn("Login failed for IP: %s", ip)
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "Invalid username or password"})
		return
	}

	utils.Limiter.ResetAttempt(ip)
	token := utils.GenerateTempKey(req.Username)

	needsSetup := utils.IsDefaultAdmin()

	utils.Info("login successful: username=%s ip=%s", req.Username, utils.GetClientIP(c))
	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "Login successful",
		Data: gin.H{
			"token":      token,
			"needsSetup": needsSetup,
		},
	})
}

// Setup 首次登录设置用户名和密码
func (h *Handler) Setup(c *gin.Context) {
	if !utils.IsDefaultAdmin() {
		c.JSON(http.StatusBadRequest, model.APIResponse{
			Code:    400,
			Message: "Setup has already been completed",
		})
		return
	}

	var req model.SetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{
			Code:    400,
			Message: "Invalid request body",
		})
		return
	}

	if len(req.Username) < 2 {
		c.JSON(http.StatusBadRequest, model.APIResponse{
			Code:    400,
			Message: "用户名至少需要2个字符",
		})
		return
	}
	if len(req.Password) < 4 {
		c.JSON(http.StatusBadRequest, model.APIResponse{
			Code:    400,
			Message: "密码至少需要4个字符",
		})
		return
	}

	if err := utils.ChangeAdminPassword(req.Username, req.Password); err != nil {
		utils.Error("setup failed to save credentials: %v", err)
		c.JSON(http.StatusInternalServerError, model.APIResponse{
			Code:    500,
			Message: "Failed to save credentials",
		})
		return
	}

	utils.Info("admin setup completed: username=%s", req.Username)
	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "Setup completed, please login again",
	})
}

// adminName 当前管理员的用户名（未设置时用默认名）。
// 单管理员模型下无法按会话区分身份，这里统一作为「操作人」留痕使用。
func adminName() string {
	name := utils.GetSetting("admin_name")
	if name == "" {
		name = "lumi"
	}
	return name
}

// GetCurrentAdmin 获取当前管理员信息
func (h *Handler) GetCurrentAdmin(c *gin.Context) {
	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data: gin.H{
			"username": adminName(),
		},
	})
}

// UpdateAdminProfile 修改当前管理员的用户名和密码
func (h *Handler) UpdateAdminProfile(c *gin.Context) {
	var req struct {
		OldPassword string `json:"oldPassword" binding:"required"`
		NewUsername string `json:"newUsername"`
		NewPassword string `json:"newPassword"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request body"})
		return
	}

	// Verify old password
	currentName := utils.GetSetting("admin_name")
	if currentName == "" {
		currentName = "lumi"
	}
	if !utils.CheckAdminCredentials(currentName, req.OldPassword) {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "当前密码不正确"})
		return
	}

	// Determine final username and password
	finalName := currentName
	finalPass := req.OldPassword
	if req.NewUsername != "" {
		if len(req.NewUsername) < 2 {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "用户名至少需要2个字符"})
			return
		}
		finalName = req.NewUsername
	}
	if req.NewPassword != "" {
		if len(req.NewPassword) < 4 {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "密码至少需要4个字符"})
			return
		}
		finalPass = req.NewPassword
	}

	if err := utils.ChangeAdminPassword(finalName, finalPass); err != nil {
		utils.Error("update profile failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "保存失败"})
		return
	}

	utils.Info("profile updated: username=%s", finalName)
	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "已更新",
	})
}

// allowedSettingKeys 允许通过接口读写的设置项白名单。
// 提成包级变量：GET / PUT 原本各有一份字面量，两边一旦不同步就会出现
// 「读得到、存不回」的情况（通知页保存 webhook 配置时正是如此）。
var allowedSettingKeys = map[string]bool{
	"site_name":    true,
	"site_icon":    true,
	"admin_email":  true,
	"admin_name":   true,
	"allow_origin": true,
	// 站点对外访问地址：用于邮件里的退订链接等绝对 URL
	"site_url":        true,
	"smtp_host":       true,
	"smtp_port":       true,
	"smtp_user":       true,
	"smtp_pass":       true,
	"smtp_encryption": true,
	"email_enabled":   true,
	"notify_services": true,
	"notify_emails":   true,
	// 告警静默：同服务冷却窗口 + 免打扰时段（见 checker/alert_policy.go）
	"alert_cooldown_minutes": true,
	"quiet_hours_start":      true,
	"quiet_hours_end":        true,
	// 告警升级：未确认的活跃事件超过时限后再次通知（见 checker/escalation.go）
	"alert_escalation_enabled":        true,
	"alert_escalation_minutes":        true,
	"alert_escalation_repeat_minutes": true,
	// 通知模板：留空使用内置中英文案（见 checker/notify_template.go）
	"tpl_alert_subject":       true,
	"tpl_alert_body":          true,
	"tpl_resolved_subject":    true,
	"tpl_resolved_body":       true,
	"tpl_maintenance_subject": true,
	"tpl_maintenance_body":    true,
	"tpl_cert_subject":        true,
	"tpl_cert_body":           true,
	"tpl_escalation_subject":  true,
	"tpl_escalation_body":     true,
	// 报告邮件（月报见 sla_report.go，周报见 weekly_report.go）
	"sla_report_enabled":       true,
	"sla_report_emails":        true,
	"sla_report_language":      true,
	"weekly_report_enabled":    true,
	"weekly_report_emails":     true,
	"weekly_report_weekday":    true,
	"show_admin_footer_button": true,
	"sub_enable_email":         true,
	"sub_enable_rss":           true,
	"sub_enable_atom":          true,
	"custom_footer":            true,
	// Webhook 通知渠道
	"webhook_enabled":          true,
	"webhook_type":             true,
	"webhook_url":              true,
	"webhook_secret":           true,
	"webhook_events":           true,
	"webhook_telegram_chat_id": true,
}

// sensitiveSettingKeys 敏感设置项：读取时置空，写入时空值表示「保持不变」，
// 避免前端只回显空值就把已保存的密钥/密码覆盖掉。
var sensitiveSettingKeys = map[string]bool{
	"admin_password": true,
	"smtp_pass":      true,
	"webhook_secret": true,
}

// readOnlySettingKeys 服务端自己维护的状态标记：可以在设置接口里读出来回显，
// 但不接受写入（否则「上次发送时间」之类的去重标记会被前端整包提交覆盖）。
var readOnlySettingKeys = map[string]bool{
	"last_sla_report_month":   true,
	"weekly_report_last_sent": true,
}

// GetSettings 获取系统设置
func (h *Handler) GetSettings(c *gin.Context) {
	all := utils.GetAllSettings()

	keys := make([]string, 0, len(allowedSettingKeys)+len(readOnlySettingKeys))
	for k := range allowedSettingKeys {
		keys = append(keys, k)
	}
	for k := range readOnlySettingKeys {
		keys = append(keys, k)
	}

	filtered := make(map[string]string)
	for _, key := range keys {
		if val, ok := all[key]; ok {
			if sensitiveSettingKeys[key] {
				filtered[key] = ""
			} else {
				filtered[key] = val
			}
		}
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "Settings fetched",
		Data:    filtered,
	})
}

// UpdateSettings 更新系统设置
func (h *Handler) UpdateSettings(c *gin.Context) {
	var body map[string]string
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request body"})
		return
	}

	for key := range body {
		// 服务端状态标记允许出现在请求里（GET 会回显、前端整包提交），但不接受写入
		if readOnlySettingKeys[key] {
			continue
		}
		if !allowedSettingKeys[key] {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Setting \"" + key + "\" is not allowed"})
			return
		}
	}
	// Validate: enabling email subscription requires SMTP to be configured
	if v, ok := body["sub_enable_email"]; ok && v == "true" {
		host := utils.GetSetting("smtp_host")
		port := utils.GetSetting("smtp_port")
		user := utils.GetSetting("smtp_user")
		pass := utils.GetSetting("smtp_pass")
		if host == "" || port == "" || user == "" || pass == "" {
			c.JSON(http.StatusBadRequest, model.APIResponse{
				Code:    400,
				Message: "请先在「通知管理」中配置完整的 SMTP 信息并启用邮件通知",
			})
			return
		}
	}

	for key, value := range body {
		// 敏感项留空表示「不修改」：前端只回显空值，直接写入会把已保存的密码清掉
		if sensitiveSettingKeys[key] && value == "" {
			continue
		}
		if err := utils.SetSetting(key, value); err != nil {
			utils.Error("Failed to update setting %s: %v", key, err)
		}
	}

	utils.Info("settings updated by admin")
	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "Settings updated",
	})
}

// TestEmail 发送测试邮件验证SMTP配置
func (h *Handler) TestEmail(c *gin.Context) {
	var req struct {
		To string `json:"to" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请输入收件地址"})
		return
	}

	host := utils.GetSetting("smtp_host")
	port := utils.GetSetting("smtp_port")
	user := utils.GetSetting("smtp_user")
	pass := utils.GetSetting("smtp_pass")

	if host == "" || port == "" || user == "" || pass == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "SMTP 配置不完整，请先保存配置"})
		return
	}

	lang := i18n.Current()
	subject := lang.TestEmailSubject()
	body := `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="margin:0;padding:0;background:#f5f5f5;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">
<table width="100%" cellpadding="0" cellspacing="0" style="padding:40px 16px"><tr><td align="center">
<table width="480" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;overflow:hidden;box-shadow:0 1px 3px rgba(0,0,0,.08)">
<tr><td style="padding:32px 32px 0"><div style="font-size:20px;font-weight:700;color:#1a1a2e">` + subject + `</div></td></tr>
<tr><td style="padding:16px 32px 32px;font-size:14px;line-height:1.7;color:#555">
` + lang.TestEmailBody() + `
</td></tr>
<tr><td style="padding:16px 32px;border-top:1px solid #eee;font-size:12px;color:#999;text-align:center">
` + lang.EmailFooter() + `
</td></tr>
</table>
</td></tr></table>
</body>
</html>`

	if err := utils.SendHTMLMail(req.To, subject, body); err != nil {
		utils.Error("test email to %s failed: %v", req.To, err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "发送失败: " + err.Error()})
		return
	}

	utils.Info("test email sent to %s", req.To)
	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "测试邮件发送成功",
	})
}

// TestWebhook 发送一条测试 webhook，验证地址与签名配置是否可用。
// 直接使用已保存的设置，因此前端需要先保存再测试（与测试邮件的行为一致）。
func (h *Handler) TestWebhook(c *gin.Context) {
	if utils.GetSetting("webhook_url") == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "webhook 地址未配置，请先保存配置"})
		return
	}

	// 测试消息统一用 down 事件，便于接收端核对签名与字段格式
	lang := i18n.Current()
	evt := utils.WebhookEvent{
		Event:   utils.WebhookEventDown,
		Service: lang.TestWebhookService(),
		URL:     "https://example.com/health",
		Time:    time.Now().Format(time.RFC3339),
		Message: lang.TestWebhookMessage(),
	}

	if err := utils.SendWebhookNow(evt); err != nil {
		utils.Error("test webhook failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "发送失败: " + err.Error()})
		return
	}

	utils.Info("test webhook sent")
	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "测试 webhook 发送成功",
	})
}
