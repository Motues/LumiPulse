package http

import (
	"lumipluse-backend/internal/model"
	"net/http"

	"github.com/gin-gonic/gin"
)

type DashboardStats struct {
	TotalServices      int                   `json:"totalServices"`
	OperationalCount   int                   `json:"operationalCount"`
	DegradedCount      int                   `json:"degradedCount"`
	OutageCount        int                   `json:"outageCount"`
	ActiveIncidents    int                   `json:"activeIncidents"`
	ActiveMaintenances int                   `json:"activeMaintenances"`
	Services           []model.ServiceSummary `json:"services"`
	RecentIncidents    []*model.Incident     `json:"recentIncidents"`
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

	total := len(services)
	operational := 0
	degraded := 0
	outage := 0
	summaries := make([]model.ServiceSummary, 0, total)

	for _, svc := range services {
		uptime := h.calcUptime(c, svc.ID, 90)
		latency := 0
		if hb, err := h.Repo.GetLatestHeartbeat(c.Request.Context(), svc.ID); err == nil {
			latency = hb.Latency
		}
		summaries = append(summaries, model.ServiceSummary{
			ID:       svc.ID,
			Name:     svc.Name,
			Status:   svc.Status,
			URL:      svc.URL,
			Type:     svc.Type,
			Uptime:   uptime,
			Latency:  latency,
			Interval: svc.Interval,
		})

		switch svc.Status {
		case "operational":
			operational++
		case "degraded":
			degraded++
		case "outage":
			outage++
		}
	}

	for i := range activeIncidents {
		updates, _ := h.Repo.ListIncidentUpdates(c.Request.Context(), activeIncidents[i].ID)
		activeIncidents[i].Updates = updates
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
			TotalServices:      total,
			OperationalCount:   operational,
			DegradedCount:      degraded,
			OutageCount:        outage,
			ActiveIncidents:    significantCount,
			ActiveMaintenances: len(activeMaints),
			Services:           summaries,
			RecentIncidents:    activeIncidents,
		},
	})
}
