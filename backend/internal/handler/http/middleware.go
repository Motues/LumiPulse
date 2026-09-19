package http

import (
	"context"
	"lumipluse-backend/internal/pkg/utils"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// API 密钥的权限范围
const (
	// ApiKeyScopeRead 只允许读取（GET / HEAD / OPTIONS）
	ApiKeyScopeRead = "read"
	// ApiKeyScopeWrite 允许除高危操作外的全部请求
	ApiKeyScopeWrite = "write"
)

// normalizeApiKeyScope 收敛范围取值，非法值一律按最小权限 read 处理
func normalizeApiKeyScope(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), ApiKeyScopeWrite) {
		return ApiKeyScopeWrite
	}
	return ApiKeyScopeRead
}

// maxApiKeyRateLimit 每分钟限流上限，避免误填一个大数字后形同不限
const maxApiKeyRateLimit = 6000

// normalizeRateLimit 收敛限流取值
func normalizeRateLimit(raw int) int {
	if raw < 0 {
		return 0
	}
	if raw > maxApiKeyRateLimit {
		return maxApiKeyRateLimit
	}
	return raw
}

// sessionOnlyRoute 高危操作只允许会话登录（浏览器）调用，API 密钥一律拒绝：
// 密钥泄漏的影响面通常远大于一次会话泄漏，这几类操作没有自动化调用的合理场景。
func sessionOnlyRoute(method, path string) bool {
	clean := strings.TrimSuffix(path, "/")
	switch {
	case method == http.MethodDelete && strings.HasPrefix(clean, "/api/v1/admin/services/"):
		return true // 删除服务
	case method == http.MethodPost && clean == "/api/v1/admin/import":
		return true // 导入覆盖数据
	case method == http.MethodPut && clean == "/api/v1/admin/profile":
		return true // 修改管理员账号 / 密码
	}
	return false
}

// isReadOnlyMethod 是否属于只读方法
func isReadOnlyMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

// rateWindow 单个密钥当前这一分钟的计数窗口
type rateWindow struct {
	start time.Time
	count int
}

// apiKeyRateLimiter 进程内的密钥限流器（按密钥 ID 计每分钟请求数）。
// 状态只存内存：重启后重新计数，对限流来说可以接受；也避免为了限流引入额外写库。
type apiKeyRateLimiter struct {
	mu     sync.Mutex
	window map[int64]*rateWindow
}

// allow 记一次请求，返回是否放行
func (l *apiKeyRateLimiter) allow(keyID int64, limit int) bool {
	if limit <= 0 {
		return true
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.window == nil {
		l.window = make(map[int64]*rateWindow)
	}
	now := time.Now()
	w := l.window[keyID]
	if w == nil || now.Sub(w.start) >= time.Minute {
		w = &rateWindow{start: now}
		l.window[keyID] = w
	}
	w.count++
	return w.count <= limit
}

// apiKeyExpiryLayouts 过期时间的可接受格式。契约是 RFC3339，但历史数据与
// 手工调用接口时可能是无时区的 datetime-local 形式，一并兼容。
var apiKeyExpiryLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02 15:04:05",
}

// parseAPIKeyExpiry 解析密钥过期时间。无时区的值按北京时间（UTC+8）解释，
// 与前端展示口径一致；无法解析时返回 false（调用方按已过期处理）。
func parseAPIKeyExpiry(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range apiKeyExpiryLayouts {
		if layout == time.RFC3339 {
			if t, err := time.Parse(layout, raw); err == nil {
				return t, true
			}
			continue
		}
		if t, err := time.ParseInLocation(layout, raw, time.FixedZone("CST", 8*3600)); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// AuthMiddleware checks Bearer token (session token or API key)
func (h *Handler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "Invalid token",
			})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "Invalid token",
			})
			return
		}

		// Check session token first：会话登录视为管理员本人操作，不受密钥范围限制
		if utils.IsTokenValid(token) {
			c.Next()
			return
		}

		// Check API key
		key, err := h.Repo.GetApiKeyByKey(c.Request.Context(), token)
		if err != nil || !key.IsActive {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "Invalid token",
			})
			return
		}

		// Check expiration
		if key.ExpiresAt != "" {
			expiresAt, ok := parseAPIKeyExpiry(key.ExpiresAt)
			if !ok || time.Now().UTC().After(expiresAt) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"code":    401,
					"message": "API key expired",
				})
				return
			}
		}

		// 高危操作不接受 API 密钥
		if sessionOnlyRoute(c.Request.Method, c.Request.URL.Path) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    403,
				"message": "该操作仅允许在管理后台登录后执行，API 密钥不可调用",
			})
			return
		}

		// 只读密钥只能读
		if normalizeApiKeyScope(key.Scope) == ApiKeyScopeRead && !isReadOnlyMethod(c.Request.Method) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    403,
				"message": "该 API 密钥为只读范围，不允许写操作",
			})
			return
		}

		// 每分钟限流
		if !h.apiKeyLimiter.allow(key.ID, normalizeRateLimit(key.RateLimitPerMinute)) {
			c.Header("Retry-After", "60")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "API 密钥请求过于频繁，请稍后重试",
			})
			return
		}

		// Track usage async
		ip := utils.GetClientIP(c)
		go h.Repo.UpdateApiKeyLastUsed(context.Background(), key.ID, ip)

		c.Next()
	}
}
