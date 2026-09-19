package http

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"lumipluse-backend/internal/model"
	"lumipluse-backend/internal/pkg/utils"

	"github.com/gin-gonic/gin"
)

// 订阅者自助管理（退订 / 修改偏好）。
//
// 保护方式是「邮箱 + 签名令牌」：令牌 = HMAC-SHA256(密钥, 归一化邮箱)，
// 与邮箱绑定，因此别人拿到链接也无法退订他人邮箱。
// 令牌不落库（可由密钥随时重算），因此升级不需要数据迁移。

// subscriptionView 返回给退订页的订阅信息
type subscriptionView struct {
	Email    string  `json:"email"`
	Services []int64 `json:"services"` // 空数组 = 订阅全部服务
	SiteName string  `json:"siteName"`
}

// parseSubscribedServices 把逗号分隔的服务 ID 串解析成数组
func parseSubscribedServices(raw string) []int64 {
	result := make([]int64, 0)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if id, err := strconv.ParseInt(part, 10, 64); err == nil && id > 0 {
			result = append(result, id)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

// resolveSubscription 校验签名并取出订阅记录。
// 校验失败或不存在时已写出响应，返回 false 表示调用方应直接结束。
func (h *Handler) resolveSubscription(c *gin.Context, email, token string) (*model.Subscriber, bool) {
	email = strings.TrimSpace(email)
	token = strings.TrimSpace(token)

	if email == "" {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: "缺少邮箱地址"})
		return nil, false
	}
	if !utils.VerifyUnsubscribe(email, token) {
		// 统一按 403 返回，避免通过错误信息区分「邮箱不存在」与「签名不对」
		c.JSON(http.StatusForbidden, model.APIResponse{Code: 403, Message: "退订链接无效或已过期"})
		return nil, false
	}

	sub, err := h.Repo.GetSubscriberByEmail(c.Request.Context(), email)
	if err != nil || sub == nil {
		c.JSON(http.StatusNotFound, model.APIResponse{Code: 404, Message: "该邮箱没有订阅记录"})
		return nil, false
	}
	return sub, true
}

// GetSubscription 退订页读取当前订阅偏好
func (h *Handler) GetSubscription(c *gin.Context) {
	sub, ok := h.resolveSubscription(c, c.Query("email"), c.Query("token"))
	if !ok {
		return
	}

	siteName := strings.TrimSpace(utils.GetSetting("site_name"))
	if siteName == "" {
		siteName = "LumiPulse"
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "ok",
		Data: subscriptionView{
			Email:    sub.Email,
			Services: parseSubscribedServices(sub.SubscribedServices),
			SiteName: siteName,
		},
	})
}

// UpdateSubscription 退订页保存订阅偏好（空数组 = 订阅全部）
func (h *Handler) UpdateSubscription(c *gin.Context) {
	var req struct {
		Email    string  `json:"email" binding:"required"`
		Token    string  `json:"token" binding:"required"`
		Services []int64 `json:"services"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{Code: 400, Message: "Invalid request body"})
		return
	}

	// 令牌既可以在查询串里，也可以在请求体里（前端统一走请求体）
	sub, ok := h.resolveSubscription(c, req.Email, req.Token)
	if !ok {
		return
	}

	parts := make([]string, 0, len(req.Services))
	for _, id := range req.Services {
		if id > 0 {
			parts = append(parts, strconv.FormatInt(id, 10))
		}
	}
	if err := h.Repo.UpdateSubscriberServices(c.Request.Context(), sub.Email, strings.Join(parts, ",")); err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: "保存失败，请稍后重试"})
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "订阅偏好已更新"})
}

// Unsubscribe 退订：直接删除订阅记录
func (h *Handler) Unsubscribe(c *gin.Context) {
	sub, ok := h.resolveSubscription(c, c.Query("email"), c.Query("token"))
	if !ok {
		return
	}

	if err := h.Repo.DeleteSubscriber(c.Request.Context(), sub.ID); err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{Code: 500, Message: "退订失败，请稍后重试"})
		return
	}
	utils.Info("subscriber unsubscribed: %s", sub.Email)

	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "已退订"})
}
