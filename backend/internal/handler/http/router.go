package http

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, h *Handler) {
	// Public API - no auth required
	v1 := r.Group("/api/v1")
	{
		v1.GET("/health", h.Health)
		v1.GET("/summary", h.GetSummary)
		v1.GET("/services", h.ListServices)
		// 批量每日统计：首页一次性取回所有服务的状态矩阵，避免 N+1 请求
		v1.GET("/daily-stats", h.GetBatchDailyStats)
		// 公开服务接口一律用随机 hash 定位，不暴露自增 ID
		v1.GET("/services/:hash/history", h.GetServiceHistory)
		v1.GET("/services/:hash/latency", h.GetServiceLatency)
		v1.GET("/services/:hash/daily-stats", h.GetDailyStats)
		v1.GET("/incidents", h.ListIncidents)
		// 公开详情使用随机 hash 访问，不暴露自增 ID
		v1.GET("/incidents/:hash", h.GetPublicIncident)
		v1.GET("/maintenances", h.ListMaintenances)
		v1.GET("/site-config", h.GetSiteConfig)
		v1.POST("/subscribe", h.Subscribe)
	}

	// Feed routes at root level
	r.GET("/feed/rss", h.GetRSSFeed)
	r.GET("/feed/atom", h.GetAtomFeed)

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
			// 批量每日统计（含未在首页展示的服务），避免管理端按服务逐个请求
			auth.GET("/daily-stats", h.AdminBatchDailyStats)

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
			auth.PUT("/services/reorder", h.AdminReorderServices)
			auth.PUT("/services/:id", h.UpdateService)
			auth.DELETE("/services/:id", h.DeleteService)

			// Incidents
			auth.GET("/incidents", h.AdminListIncidents)
			auth.GET("/incidents/:id", h.GetAdminIncident)
			auth.POST("/incidents", h.CreateIncident)
			auth.POST("/incidents/:id/updates", h.CreateIncidentUpdate)
			auth.PUT("/incidents/:id/updates/:updateId", h.UpdateIncidentUpdate)
			auth.DELETE("/incidents/:id/updates/:updateId", h.DeleteIncidentUpdate)
			auth.PATCH("/incidents/:id", h.UpdateIncident)
			auth.DELETE("/incidents/:id", h.DeleteIncident)
			auth.POST("/incidents/:id/merge", h.MergeIncident)
			auth.POST("/incidents/:id/split", h.SplitIncident)

			// Servers
			auth.GET("/servers", h.ListServers)
			auth.POST("/servers", h.CreateServer)
			auth.PUT("/servers/:id", h.UpdateServer)
			auth.DELETE("/servers/:id", h.DeleteServer)

			// Probe Tasks
			auth.GET("/probe-tasks", h.ListProbeTasks)
			auth.POST("/probe-tasks", h.CreateProbeTask)
			auth.PUT("/probe-tasks/:id", h.UpdateProbeTask)
			auth.DELETE("/probe-tasks/:id", h.DeleteProbeTask)

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

			// Subscribers
			auth.GET("/subscribers", h.AdminListSubscribers)
			auth.DELETE("/subscribers/:id", h.AdminDeleteSubscriber)
			// Data export/import
			auth.GET("/export", h.AdminExport)
			auth.POST("/import", h.AdminImport)
		}
	}
}
