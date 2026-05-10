package http

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, h *Handler) {
	// Public API - no auth required
	v1 := r.Group("/api/v1")
	{
		v1.GET("/summary", h.GetSummary)
		v1.GET("/services", h.ListServices)
		v1.GET("/services/:id/history", h.GetServiceHistory)
		v1.GET("/services/:id/daily-stats", h.GetDailyStats)
		v1.GET("/incidents", h.ListIncidents)
		v1.GET("/maintenances", h.ListMaintenances)
		v1.GET("/site-config", h.GetSiteConfig)
	}

	// Admin API
	admin := r.Group("/api/v1/admin")
	{
		admin.POST("/login", h.Login)
		admin.POST("/setup", h.Setup)

		auth := admin.Group("/")
		auth.Use(h.AuthMiddleware())
		{
			// Dashboard
			auth.GET("/stats", h.AdminStats)

			// Logs
			auth.GET("/logs", h.AdminListLogs)

			// Settings
			auth.GET("/settings", h.GetSettings)
			auth.PUT("/settings", h.UpdateSettings)

			// Admin profile
			auth.GET("/current-user", h.GetCurrentAdmin)
			auth.PUT("/profile", h.UpdateAdminProfile)

			// Notifications
			auth.POST("/test-email", h.TestEmail)

			// Services
			auth.GET("/services", h.AdminListServices)
			auth.POST("/services", h.CreateService)
			auth.PUT("/services/:id", h.UpdateService)
			auth.DELETE("/services/:id", h.DeleteService)

			// Incidents
			auth.GET("/incidents", h.AdminListIncidents)
			auth.POST("/incidents", h.CreateIncident)
			auth.POST("/incidents/:id/updates", h.CreateIncidentUpdate)
			auth.PATCH("/incidents/:id", h.UpdateIncident)
			auth.DELETE("/incidents/:id", h.DeleteIncident)

			// Maintenances
			auth.GET("/maintenances", h.AdminListMaintenances)
			auth.POST("/maintenances", h.CreateMaintenance)
			auth.PUT("/maintenances/:id", h.UpdateMaintenance)
			auth.DELETE("/maintenances/:id", h.DeleteMaintenance)

			// ApiKeys
			auth.GET("/api-keys", h.ListApiKeys)
			auth.POST("/api-keys", h.CreateApiKey)
			auth.PUT("/api-keys/:id", h.UpdateApiKey)
			auth.DELETE("/api-keys/:id", h.DeleteApiKey)
		}
	}
}
