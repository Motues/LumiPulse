package http

import (
	"lumipluse-backend/internal/repository"
	"sync"
	"time"
)

// summaryCacheTTL 公开总览的缓存时长。写操作会主动失效缓存，
// 因此这里只需覆盖突发读流量即可。
const summaryCacheTTL = 25 * time.Second

type Handler struct {
	Repo    repository.Repository
	Version string

	cache   cacheEntry
	cacheMu sync.RWMutex

	// apiKeyLimiter API 密钥的每分钟限流状态（进程内，见 middleware.go）
	apiKeyLimiter apiKeyRateLimiter
}

type cacheEntry struct {
	data      interface{}
	expiresAt time.Time
}

func (h *Handler) getCached() interface{} {
	h.cacheMu.RLock()
	defer h.cacheMu.RUnlock()
	if time.Now().Before(h.cache.expiresAt) {
		return h.cache.data
	}
	return nil
}

func (h *Handler) setCache(data interface{}) {
	h.cacheMu.Lock()
	defer h.cacheMu.Unlock()
	h.cache = cacheEntry{
		data:      data,
		expiresAt: time.Now().Add(summaryCacheTTL),
	}
}

// InvalidateSummary 主动失效公开总览缓存。
// 服务 / 事件 / 维护计划发生任何变化时都必须调用，
// 否则公开页最长会有 summaryCacheTTL 的数据延迟。
// 检查器（checker）通过 SetOnDataChange 注入本方法。
func (h *Handler) InvalidateSummary() {
	h.cacheMu.Lock()
	defer h.cacheMu.Unlock()
	h.cache = cacheEntry{}
}
