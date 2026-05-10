package utils

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
)

func SendMail(to, subject, body string) error {
	return sendMail(to, subject, body, false)
}

func SendHTMLMail(to, subject, htmlBody string) error {
	return sendMail(to, subject, htmlBody, true)
}

func sendMail(to, subject, body string, isHTML bool) error {
	host := GetSetting("smtp_host")
	port := GetSetting("smtp_port")
	user := GetSetting("smtp_user")
	pass := GetSetting("smtp_pass")
	encryption := GetSetting("smtp_encryption")

	if host == "" || port == "" || user == "" || pass == "" {
		return fmt.Errorf("SMTP 配置不完整")
	}

	addr := host + ":" + port

	contentType := "text/plain; charset=UTF-8"
	if isHTML {
		contentType = "text/html; charset=UTF-8"
	}

	msg := []byte("To: " + to + "\r\n" +
		"From: " + user + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: " + contentType + "\r\n" +
		"\r\n" +
		body)

	switch strings.ToLower(encryption) {
	case "tls":
		return sendMailTLS(addr, user, pass, to, msg)
	default:
		auth := smtp.PlainAuth("", user, pass, host)
		return smtp.SendMail(addr, auth, user, []string{to}, msg)
	}
}

func SendAlert(subject, body string) error {
	raw := GetSetting("notify_emails")
	if raw == "" {
		return nil
	}
	emails := strings.Split(raw, ",")
	var errs []string
	for _, email := range emails {
		email = strings.TrimSpace(email)
		if email == "" {
			continue
		}
		htmlBody := buildStyledEmail(subject, body)
		if err := SendHTMLMail(email, subject, htmlBody); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", email, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("发送失败: %s", strings.Join(errs, "; "))
	}
	return nil
}

func buildStyledEmail(title, content string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="margin:0;padding:0;background:#f5f5f5;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">
  <table width="100%%" cellpadding="0" cellspacing="0" style="padding:40px 16px">
    <tr>
      <td align="center">
        <table width="480" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:12px;overflow:hidden;box-shadow:0 1px 3px rgba(0,0,0,.08)">
          <tr>
            <td style="padding:32px 32px 0">
              <div style="font-size:20px;font-weight:700;color:#1a1a2e">%s</div>
            </td>
          </tr>
          <tr>
            <td style="padding:16px 32px 32px;font-size:14px;line-height:1.7;color:#555">%s</td>
          </tr>
          <tr>
            <td style="padding:16px 32px;border-top:1px solid #eee;font-size:12px;color:#999;text-align:center">
              LumiPulse &mdash; 服务监控系统
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, title, content)
}

func sendMailTLS(addr, user, pass, to string, msg []byte) error {
	host := addr[:strings.IndexByte(addr, ':')]
	tlsCfg := &tls.Config{InsecureSkipVerify: false, ServerName: host}
	conn, err := tls.Dial("tcp", addr, tlsCfg)
	if err != nil {
		tlsCfg.InsecureSkipVerify = true
		conn, err = tls.Dial("tcp", addr, tlsCfg)
		if err != nil {
			return fmt.Errorf("TLS 连接失败: %w", err)
		}
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("SMTP 客户端创建失败: %w", err)
	}
	defer client.Close()

	auth := smtp.PlainAuth("", user, pass, host)
	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("认证失败: %w", err)
	}
	if err = client.Mail(user); err != nil {
		return fmt.Errorf("发件人错误: %w", err)
	}
	if err = client.Rcpt(to); err != nil {
		return fmt.Errorf("收件人错误: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("写入数据失败: %w", err)
	}
	if _, err = w.Write(msg); err != nil {
		return fmt.Errorf("写入邮件内容失败: %w", err)
	}
	return w.Close()
}
