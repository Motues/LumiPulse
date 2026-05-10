package http

import (
	"lumipluse-backend/internal/model"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// AdminListLogs 获取监控日志（分页）
func (h *Handler) AdminListLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	serviceID, _ := strconv.ParseInt(c.DefaultQuery("serviceId", "0"), 10, 64)
	statusFilter := c.DefaultQuery("status", "all")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 200 {
		limit = 50
	}
	if statusFilter != "success" && statusFilter != "failure" {
		statusFilter = "all"
	}

	logs, total, err := h.Repo.ListHeartbeats(c.Request.Context(), serviceID, statusFilter, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{
			Code:    500,
			Message: "Failed to fetch logs",
		})
		return
	}

	totalPage := (total + int64(limit) - 1) / int64(limit)
	if total == 0 {
		totalPage = 0
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data: gin.H{
			"logs":       logs,
			"pagination": model.Pagination{Page: page, Limit: limit, TotalPage: totalPage},
		},
	})
}
