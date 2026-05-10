package utils

import (
	"crypto/rand"
	"fmt"
)

// GenerateApiKey 生成 API 密钥，格式：lp_<32位随机十六进制>
// 返回 (完整密钥, 前8位前缀)
func GenerateApiKey() (string, string) {
	b := make([]byte, 32)
	rand.Read(b)
	key := fmt.Sprintf("lp_%x", b)
	prefix := key[:8] // 前8位用于显示
	return key, prefix
}
