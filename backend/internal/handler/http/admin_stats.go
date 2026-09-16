package http

import (
	"lumipluse-backend/internal/model"
	"net/http"

	"github.com/gin-gonic/gin"
)

type DashboardStats struct {
	TotalServices           int                    `json:"totalServices"`
	OperationalCount        int                    `json:"operationalCount"`
	DegradedCount           int                    `json:"degradedCount"`
	OutageCount             int                    `json:"outageCount"`
	ActiveIncidents         int                    `json:"activeIncidents"`
	ActiveMaintenances      int                    `json:"activeMaintenances"`
	Services                []model.ServiceSummary `json:"services"`
	RecentIncidents         []*model.Incident      `json:"recentIncidents"`
	RecentIncidentsTotal    int64                  `json:"recentIncidentsTotal"`
	RecentIncidentsResolved int64                  `json:"recentIncidentsResolved"`
}

func (h *Handler) AdminStats(c *gin.Context) {
	services, err := h.Repo.ListServices(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to fetch services"})
		return
	}

	activeIncidents, _ := h.Repo.ListActiveIncidents(c.Request.Context())
	if activeIncidents == nil {
		activeIncidents = []*model.Incident{}
	}
	activeMaints, _ := h.Repo.ListActiveMaintenances(c.Request.Context())
	recentTotal, recentResolved, _ := h.Repo.CountRecentIncidents(c.Request.Context(), 30)

	total := len(services)
	operational := 0
	degraded := 0
	outage := 0

	for _, svc := range services {
		switch svc.Status {
		case "operational":
			operational++
		case "degraded":
			degraded++
		case "outage":
			outage++
		}
	}

	// 复用统一的摘要组装逻辑（批量取在线率与最新延迟，避免 N+1）
	statusMap := make(map[int64]string, len(services))
	for _, svc := range services {
		statusMap[svc.ID] = svc.Status
	}
	summaries := h.buildServiceSummaries(c, services, func(id int64) string {
		return statusMap[id]
	})

	// 批量取各事件的进展更新
	if len(activeIncidents) > 0 {
		incIDs := make([]int64, len(activeIncidents))
		for i, inc := range activeIncidents {
			incIDs[i] = inc.ID
		}
		if updatesMap, err := h.Repo.BatchListIncidentUpdates(c.Request.Context(), incIDs); err == nil {
			for _, inc := range activeIncidents {
				updates := updatesMap[inc.ID]
				if updates == nil {
					updates = []*model.IncidentUpdate{}
				}
				inc.Updates = updates
			}
		}
	}

	// Only count non-monitoring incidents in the badge (monitoring = already resolved in practice)
	significantCount := 0
	for _, inc := range activeIncidents {
		if inc.Status != "monitoring" {
			significantCount++
		}
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data: DashboardStats{
			TotalServices:           total,
			OperationalCount:        operational,
			DegradedCount:           degraded,
			OutageCount:             outage,
			ActiveIncidents:         significantCount,
			ActiveMaintenances:      len(activeMaints),
			Services:                summaries,
			RecentIncidents:         activeIncidents,
			RecentIncidentsTotal:    recentTotal,
			RecentIncidentsResolved: recentResolved,
		},
	})
}
