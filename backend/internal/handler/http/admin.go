package http

import (
	"log"
	"lumipluse-backend/internal/model"
	"lumipluse-backend/internal/pkg/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Login 管理员登录
func (h *Handler) Login(c *gin.Context) {
	ip := utils.GetClientIP(c)

	if utils.Limiter.IsIPBlocked(ip) {
		log.Printf("[WARN] Blocked IP attempted to login: %s", ip)
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
		log.Printf("[WARN] Login failed for IP: %s", ip)
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "Invalid username or password"})
		return
	}

	utils.Limiter.ResetAttempt(ip)
	token := utils.GenerateTempKey(req.Username)

	needsSetup := utils.IsDefaultAdmin()

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
		c.JSON(http.StatusInternalServerError, model.APIResponse{
			Code:    500,
			Message: "Failed to save credentials",
		})
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "Setup completed, please login again",
	})
}

// GetCurrentAdmin 获取当前管理员信息
func (h *Handler) GetCurrentAdmin(c *gin.Context) {
	name := utils.GetSetting("admin_name")
	if name == "" {
		name = "lumi"
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data: gin.H{
			"username": name,
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
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "保存失败"})
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "已更新",
	})
}

// GetSettings 获取系统设置
func (h *Handler) GetSettings(c *gin.Context) {
	all := utils.GetAllSettings()

	allowedSettings := map[string]bool{
		"site_name":       true,
		"site_icon":       true,
		"admin_email":     true,
		"admin_name":      true,
		"allow_origin":    true,
		"smtp_host":       true,
		"smtp_port":       true,
		"smtp_user":       true,
		"smtp_encryption": true,
		"email_enabled":   true,
		"notify_services": true,
		"notify_emails":   true,
	}

	sensitiveKeys := map[string]bool{
		"admin_password": true,
		"smtp_pass":      true,
	}

	keys := make([]string, 0, len(allowedSettings))
	for k := range allowedSettings {
		keys = append(keys, k)
	}

	filtered := make(map[string]string)
	for _, key := range keys {
		if val, ok := all[key]; ok {
			if sensitiveKeys[key] {
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
	allowedSettings := map[string]bool{
		"site_name":       true,
		"site_icon":       true,
		"admin_email":     true,
		"admin_name":      true,
		"allow_origin":    true,
		"smtp_host":       true,
		"smtp_port":       true,
		"smtp_user":       true,
		"smtp_pass":       true,
		"smtp_encryption": true,
		"email_enabled":   true,
		"notify_services": true,
		"notify_emails":   true,
	}

	var body map[string]string
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request body"})
		return
	}

	for key := range body {
		if !allowedSettings[key] {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Setting \"" + key + "\" is not allowed"})
			return
		}
	}

	for key, value := range body {
		if err := utils.SetSetting(key, value); err != nil {
			log.Printf("[ERROR] Failed to update setting %s: %v", key, err)
		}
	}

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

	subject := "LumiPulse 邮件通知"
	body := `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="margin:0;padding:0;background:#f5f5f5;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">
<table width="100%" cellpadding="0" cellspacing="0" style="padding:40px 16px"><tr><td align="center">
<table width="480" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;overflow:hidden;box-shadow:0 1px 3px rgba(0,0,0,.08)">
<tr><td style="padding:32px 32px 0"><div style="font-size:20px;font-weight:700;color:#1a1a2e">LumiPulse 邮件通知</div></td></tr>
<tr><td style="padding:16px 32px 32px;font-size:14px;line-height:1.7;color:#555">
这是一封来自 LumiPulse 的测试邮件。<br><br>
如果收到此邮件，说明您的 SMTP 配置正确。
</td></tr>
<tr><td style="padding:16px 32px;border-top:1px solid #eee;font-size:12px;color:#999;text-align:center">
LumiPulse &mdash; 服务监控系统
</td></tr>
</table>
</td></tr></table>
</body>
</html>`

	if err := utils.SendHTMLMail(req.To, subject, body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "发送失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "测试邮件发送成功",
	})
}
