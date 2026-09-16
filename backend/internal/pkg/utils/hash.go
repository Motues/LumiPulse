package utils

import (
	"crypto/rand"
	"fmt"
	"time"
)

// GeneratePublicHash 生成用于公开访问的事件标识（32 位随机十六进制）。
// 公开页面使用它替代自增 ID，避免对外暴露数据库主键与记录规模。
func GeneratePublicHash() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand 失败时退化为时间戳，至少保证返回一个非空且基本唯一的值
		return fmt.Sprintf("%032x", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", b)
}
