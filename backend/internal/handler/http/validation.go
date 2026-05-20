package http

import (
	"net/url"
	"strings"
)

type validationErrors []string

func (v validationErrors) Error() string {
	return strings.Join(v, "; ")
}

func (v validationErrors) HasErrors() bool {
	return len(v) > 0
}

func validateServiceName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "服务名称不能为空"
	}
	if len(name) > 100 {
		return "服务名称不超过100个字符"
	}
	return ""
}

func validateServiceURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "URL 不能为空"
	}
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "tcp") {
		return "URL 格式无效 (需要 http://, https:// 或 tcp:// 开头)"
	}
	return ""
}

func validateServiceInterval(interval int) string {
	if interval < 10 {
		return "检测间隔不能小于10秒"
	}
	if interval > 3600 {
		return "检测间隔不能超过3600秒"
	}
	return ""
}

func validateEmail(email string) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return "邮箱地址不能为空"
	}
	if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		return "邮箱地址格式无效"
	}
	return ""
}
