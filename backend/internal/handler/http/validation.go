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

// validateServiceTimeout 校验单次探测超时。0 表示使用默认值（10 秒），
// 因此这里只约束显式填写的范围。
func validateServiceTimeout(seconds int) string {
	if seconds == 0 {
		return ""
	}
	if seconds < 1 {
		return "探测超时不能小于1秒"
	}
	if seconds > 300 {
		return "探测超时不能超过300秒"
	}
	return ""
}

// validateExpectKeyword 校验期望响应关键字。留空表示不做关键字匹配。
func validateExpectKeyword(keyword string) string {
	if len([]rune(strings.TrimSpace(keyword))) > 200 {
		return "期望关键字不能超过200个字符"
	}
	return ""
}

// validateExpectKeywordPtr 指针版本，nil（未传）表示保持原值。
func validateExpectKeywordPtr(keyword *string) string {
	if keyword == nil {
		return ""
	}
	return validateExpectKeyword(*keyword)
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
