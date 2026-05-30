package http

import (
	"lumipluse-backend/internal/model"
	"lumipluse-backend/internal/pkg/utils"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// AdminListSubscribers 获取订阅者列表
func (h *Handler) AdminListSubscribers(c *gin.Context) {
	subscribers, err := h.Repo.ListSubscribers(c.Request.Context())
	if err != nil {
		utils.Error("list subscribers failed: %v", err)
		c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: "获取订阅者列表失败"})
		return
	}
	if subscribers == nil {
		subscribers = []*model.Subscriber{}
	}
	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "ok", Data: subscribers})
}

// AdminDeleteSubscriber 删除订阅者
func (h *Handler) AdminDeleteSubscriber(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: "无效的ID"})
		return
	}
	if err := h.Repo.DeleteSubscriber(c.Request.Context(), id); err != nil {
		utils.Error("delete subscriber %d failed: %v", id, err)
		c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: "删除失败"})
		return
	}
	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "删除成功"})
}
