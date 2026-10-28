package utils

import (
	"crypto/rand"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"lumipluse-backend/internal/config"

	"github.com/gin-gonic/gin"
)

// LoginLimiter 处理登录失败计数与 IP 封禁
type LoginLimiter struct {
	mu            sync.Mutex
	attempts      map[string]int
	blockedUntil  map[string]time.Time
	lastSeen      map[string]time.Time
	maxAttempts   int
	blockDuration time.Duration
}

// maxTrackedIPs 限流器最多记住多少个来源 IP。
// 转发头可以被伪造，不设上限时随机 IP 会把这两张表撑爆（内存泄漏）。
const maxTrackedIPs = 10000

var Limiter = &LoginLimiter{
	attempts:      make(map[string]int),
	blockedUntil:  make(map[string]time.Time),
	lastSeen:      make(map[string]time.Time),
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
//
// 优先级：Cloudflare -> X-Real-IP -> X-Forwarded-For -> RemoteAddr。
// 转发头只有来自可信代理（配置项 TRUSTED_PROXIES）时才采信：这些头是客户端
// 可以随手伪造的，无条件信任会让登录失败封禁形同虚设（每次换一个假 IP 即可
// 无限重试），审计日志里的来源 IP 也会失真。
func GetClientIP(c *gin.Context) string {
	remote := remoteIP(c)
	if !trustForwardedHeaders(remote) {
		return remote
	}

	// 优先级：Cloudflare -> X-Real-IP -> X-Forwarded-For -> RemoteAddr
	if ip := strings.TrimSpace(c.GetHeader("CF-Connecting-IP")); ip != "" {
		return ip
	}
	if ip := strings.TrimSpace(c.GetHeader("X-Real-IP")); ip != "" {
		return ip
	}
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	return remote
}

// remoteIP 取 TCP 对端地址（去掉端口）
func remoteIP(c *gin.Context) string {
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return c.Request.RemoteAddr
	}
	return host
}

// trustForwardedHeaders 判断是否采信代理转发头。
// 未配置 TRUSTED_PROXIES 时保持历史行为（信任）；配置后只信任来自这些
// IP / CIDR 网段的请求。
func trustForwardedHeaders(remote string) bool {
	raw := ""
	if config.GlobalConfig != nil {
		raw = strings.TrimSpace(config.GlobalConfig.TrustedProxies)
	}
	if raw == "" {
		return true
	}

	ip := net.ParseIP(remote)
	if ip == nil {
		return false
	}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "/") {
			if _, network, err := net.ParseCIDR(part); err == nil && network.Contains(ip) {
				return true
			}
			continue
		}
		if candidate := net.ParseIP(part); candidate != nil && candidate.Equal(ip) {
			return true
		}
	}
	return false
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

	if l.lastSeen == nil {
		l.lastSeen = make(map[string]time.Time)
	}
	now := time.Now()
	l.attempts[ip]++
	l.lastSeen[ip] = now

	// 来源 IP 来自客户端可控的请求头，必须给记账表设上限，否则随机 IP 会撑爆内存
	if len(l.lastSeen) > maxTrackedIPs {
		l.pruneLocked(now)
	}

	if l.attempts[ip] >= l.maxAttempts {
		l.blockedUntil[ip] = now.Add(l.blockDuration)
		return true
	}
	return false
}

// pruneLocked 清理长期没有出现过的 IP 记录（调用方需持有锁）
func (l *LoginLimiter) pruneLocked(now time.Time) {
	for ip, seen := range l.lastSeen {
		if now.Sub(seen) <= 2*l.blockDuration {
			continue
		}
		delete(l.lastSeen, ip)
		delete(l.attempts, ip)
		delete(l.blockedUntil, ip)
	}
}

// ResetAttempt 登录成功后重置
func (l *LoginLimiter) ResetAttempt(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, ip)
	delete(l.blockedUntil, ip)
	delete(l.lastSeen, ip)
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
