package utils

import (
	"crypto/rand"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// LoginLimiter 处理登录失败计数与 IP 封禁
type LoginLimiter struct {
	mu            sync.Mutex
	attempts      map[string]int
	blockedUntil  map[string]time.Time
	maxAttempts   int
	blockDuration time.Duration
}

var Limiter = &LoginLimiter{
	attempts:      make(map[string]int),
	blockedUntil:  make(map[string]time.Time),
	maxAttempts:   5,                // 最大尝试次数
	blockDuration: 30 * time.Minute, // 封禁时长
}

// TokenStore 模拟存储登录生成的临时密钥
var TokenStore = struct {
	sync.RWMutex
	Map map[string]time.Time // key: token, value: expiration time
}{Map: make(map[string]time.Time)}

// sessionIdleTimeout 会话空闲超时：每次使用会顺延（滑动续期）
const sessionIdleTimeout = 20 * time.Minute

// GetClientIP 获取真实 IP
func GetClientIP(c *gin.Context) string {
	// 优先级：Cloudflare -> X-Real-IP -> X-Forwarded-For -> RemoteAddr
	if ip := c.GetHeader("CF-Connecting-IP"); ip != "" {
		return ip
	}
	if ip := c.GetHeader("X-Real-IP"); ip != "" {
		return ip
	}
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	ip, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
	return ip
}

// IsIPBlocked 检查 IP 是否被封禁
func (l *LoginLimiter) IsIPBlocked(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if until, ok := l.blockedUntil[ip]; ok {
		if time.Now().Before(until) {
			return true
		}
		// 封禁到期，清理
		delete(l.blockedUntil, ip)
		delete(l.attempts, ip)
	}
	return false
}

// RecordAttempt 记录失败尝试
func (l *LoginLimiter) RecordAttempt(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.attempts[ip]++
	if l.attempts[ip] >= l.maxAttempts {
		l.blockedUntil[ip] = time.Now().Add(l.blockDuration)
		return true
	}
	return false
}

// ResetAttempt 登录成功后重置
func (l *LoginLimiter) ResetAttempt(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, ip)
	delete(l.blockedUntil, ip)
}

// GenerateTempKey 使用 CSPRNG 生成临时密钥
func GenerateTempKey(name string) string {
	b := make([]byte, 32)
	rand.Read(b)
	token := fmt.Sprintf("%x", b)

	// 设置空闲过期时间，使用时会滑动续期
	expiration := time.Now().Add(sessionIdleTimeout)

	TokenStore.Lock()
	TokenStore.Map[token] = expiration
	TokenStore.Unlock()

	return token
}

// IsTokenValid 验证密钥有效性；有效时顺延过期时间（滑动续期）。
// 这样管理员持续操作期间不会被中途踢出，而空闲超时仍然生效。
func IsTokenValid(token string) bool {
	TokenStore.Lock()
	defer TokenStore.Unlock()

	expiration, ok := TokenStore.Map[token]
	if !ok {
		return false
	}

	// 检查是否过期
	if time.Now().After(expiration) {
		delete(TokenStore.Map, token)
		return false
	}

	// 滑动续期
	TokenStore.Map[token] = time.Now().Add(sessionIdleTimeout)
	return true
}

// CleanupExpiredTokens 清理已过期的会话，避免 TokenStore 无限增长
func CleanupExpiredTokens() int {
	now := time.Now()
	removed := 0

	TokenStore.Lock()
	defer TokenStore.Unlock()
	for token, expiration := range TokenStore.Map {
		if now.After(expiration) {
			delete(TokenStore.Map, token)
			removed++
		}
	}
	return removed
}

// RevokeToken 主动注销会话
func RevokeToken(token string) {
	TokenStore.Lock()
	delete(TokenStore.Map, token)
	TokenStore.Unlock()
}
