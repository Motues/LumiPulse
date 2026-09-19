package utils

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// settingUnsubscribeSecret 退订签名密钥的设置项。
// 首次使用时自动生成并落库，因此升级后无需任何配置就有可用的退订链接。
const settingUnsubscribeSecret = "unsubscribe_secret"

// randomHex 生成 n 字节的随机十六进制串
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%0*x", n*2, 0)
	}
	return hex.EncodeToString(b)
}

// UnsubscribeSecret 返回退订签名密钥（不存在时生成并保存）。
func UnsubscribeSecret() string {
	if secret := strings.TrimSpace(GetSetting(settingUnsubscribeSecret)); secret != "" {
		return secret
	}

	secret := randomHex(32)
	if err := SetSetting(settingUnsubscribeSecret, secret); err != nil {
		// 写入失败时仍返回本次生成的密钥，至少保证当前进程内的链接可用
		Info("failed to persist unsubscribe secret: %v", err)
	}
	return secret
}

// normalizeEmail 统一邮箱大小写与空白，避免同一邮箱算出不同签名
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// SignUnsubscribe 生成退订令牌：HMAC-SHA256(密钥, 归一化邮箱)。
// 令牌与邮箱绑定，因此别人拿到某个人的链接也无法退订他人邮箱。
func SignUnsubscribe(email string) string {
	mac := hmac.New(sha256.New, []byte(UnsubscribeSecret()))
	mac.Write([]byte(normalizeEmail(email)))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyUnsubscribe 校验退订令牌，使用恒定时间比较避免时序侧信道
func VerifyUnsubscribe(email, token string) bool {
	token = strings.TrimSpace(token)
	if token == "" || strings.TrimSpace(email) == "" {
		return false
	}
	return hmac.Equal([]byte(SignUnsubscribe(email)), []byte(token))
}

// UnsubscribeURL 拼出退订/偏好管理页的绝对地址。
// 站点地址优先取 site_url，未配置时退回 allow_origin 的第一个来源；
// 都为空时返回空串（调用方据此省略链接）。
func UnsubscribeURL(email string) string {
	base := strings.TrimSpace(GetSetting("site_url"))
	if base == "" {
		for _, origin := range strings.Split(GetSetting("allow_origin"), ",") {
			origin = strings.TrimSpace(origin)
			// 允许配置里写具体页面路径，但退订链接只关心来源部分
			if origin != "" {
				base = origin
				break
			}
		}
	}
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return ""
	}
	return fmt.Sprintf("%s/unsubscribe?email=%s&token=%s", base, urlQueryEscape(email), SignUnsubscribe(email))
}

// urlQueryEscape 只转义查询串里必须处理的字符，避免引入额外依赖
func urlQueryEscape(s string) string {
	replacer := strings.NewReplacer(
		"%", "%25",
		" ", "%20",
		"+", "%2B",
		"&", "%26",
		"=", "%3D",
		"#", "%23",
		"?", "%3F",
	)
	return replacer.Replace(s)
}
