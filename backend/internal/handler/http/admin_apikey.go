package http

import (
	"lumipluse-backend/internal/model"
	"lumipluse-backend/internal/pkg/utils"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ListApiKeys 获取所有 API 密钥（不返回完整密钥）
func (h *Handler) ListApiKeys(c *gin.Context) {
	keys, err := h.Repo.ListApiKeys(c.Request.Context())
	if err != nil {
		utils.Warn("list api keys failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "获取密钥列表失败"})
		return
	}

	// 不返回完整密钥
	type safeKey struct {
		ID                 int64  `json:"id"`
		Name               string `json:"name"`
		MaskedKey          string `json:"maskedKey"`
		ExpiresAt          string `json:"expiresAt"`
		LastUsedAt         string `json:"lastUsedAt,omitempty"`
		LastUsedIP         string `json:"lastUsedIP,omitempty"`
		IsActive           bool   `json:"isActive"`
		Scope              string `json:"scope"`
		RateLimitPerMinute int    `json:"rateLimitPerMinute"`
		CreatedAt          string `json:"createdAt"`
	}
	result := make([]safeKey, len(keys))
	for i, k := range keys {
		masked := k.Key
		if len(masked) > 10 {
			masked = masked[:6] + "****" + masked[len(masked)-4:]
		}
		result[i] = safeKey{
			ID:                 k.ID,
			Name:               k.Name,
			MaskedKey:          masked,
			ExpiresAt:          k.ExpiresAt,
			LastUsedAt:         k.LastUsedAt,
			LastUsedIP:         k.LastUsedIP,
			IsActive:           k.IsActive,
			Scope:              normalizeApiKeyScope(k.Scope),
			RateLimitPerMinute: normalizeRateLimit(k.RateLimitPerMinute),
			CreatedAt:          k.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data:    result,
	})
}

// CreateApiKey 创建 API 密钥（完整密钥仅在此返回）
func (h *Handler) CreateApiKey(c *gin.Context) {
	var req model.CreateApiKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request body"})
		return
	}

	key, prefix := utils.GenerateApiKey()

	k := &model.ApiKey{
		Name:      req.Name,
		Key:       key,
		KeyPrefix: prefix,
		ExpiresAt: req.ExpiresAt,
		// 未指定范围时按最小权限处理
		Scope:              normalizeApiKeyScope(req.Scope),
		RateLimitPerMinute: normalizeRateLimit(req.RateLimitPerMinute),
	}

	if err := h.Repo.CreateApiKey(c.Request.Context(), k); err != nil {
		utils.Error("create api key failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "创建密钥失败"})
		return
	}

	c.JSON(http.StatusCreated, model.APIResponse{
		Code:    201,
		Message: "密钥创建成功",
		Data: gin.H{
			"id":                 k.ID,
			"name":               k.Name,
			"key":                key,
			"keyPrefix":          prefix,
			"expiresAt":          k.ExpiresAt,
			"scope":              k.Scope,
			"rateLimitPerMinute": k.RateLimitPerMinute,
			"createdAt":          k.CreatedAt,
		},
	})
}

// UpdateApiKey 更新 API 密钥（名称 / 权限范围 / 每分钟限流）
func (h *Handler) UpdateApiKey(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid key id"})
		return
	}

	var req model.UpdateApiKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请提供密钥名称"})
		return
	}

	existing, err := h.Repo.GetApiKey(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "密钥不存在"})
		return
	}

	scope := normalizeApiKeyScope(existing.Scope)
	if req.Scope != nil {
		scope = normalizeApiKeyScope(*req.Scope)
	}
	rateLimit := normalizeRateLimit(existing.RateLimitPerMinute)
	if req.RateLimitPerMinute != nil {
		rateLimit = normalizeRateLimit(*req.RateLimitPerMinute)
	}

	if err := h.Repo.UpdateApiKey(c.Request.Context(), id, req.Name, scope, rateLimit); err != nil {
		utils.Error("update api key %d failed: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "更新密钥失败"})
		return
	}

	auditLog("apikey.update", "id="+idStr+" scope="+scope)

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "密钥已更新",
		Data: gin.H{
			"id":                 id,
			"name":               req.Name,
			"scope":              scope,
			"rateLimitPerMinute": rateLimit,
		},
	})
}

// DeleteApiKey 删除 API 密钥
func (h *Handler) DeleteApiKey(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid key id"})
		return
	}

	if err := h.Repo.DeleteApiKey(c.Request.Context(), id); err != nil {
		utils.Error("delete api key %d failed: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "删除密钥失败"})
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "密钥已删除",
	})
}
