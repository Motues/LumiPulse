package http

import (
	"lumipluse-backend/internal/repository"
	"sync"
	"time"
)

type Handler struct {
	Repo    repository.Repository
	Version string

	cache      cacheEntry
	cacheMu    sync.RWMutex
	cacheTTL   time.Duration
	cacheInit  sync.Once
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
	h.cacheInit.Do(func() {
		h.cacheTTL = 25 * time.Second
	})
	h.cacheMu.Lock()
	defer h.cacheMu.Unlock()
	h.cache = cacheEntry{
		data:      data,
		expiresAt: time.Now().Add(h.cacheTTL),
	}
}
