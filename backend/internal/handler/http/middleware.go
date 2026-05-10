package http

import (
	"context"
	"lumipluse-backend/internal/pkg/utils"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

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

		// Check session token first
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
			t, parseErr := time.Parse(time.RFC3339, key.ExpiresAt)
			if parseErr != nil || time.Now().UTC().After(t) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"code":    401,
					"message": "API key expired",
				})
				return
			}
		}

		// Track usage async
		ip := utils.GetClientIP(c)
		go h.Repo.UpdateApiKeyLastUsed(context.Background(), key.ID, ip)

		c.Next()
	}
}
