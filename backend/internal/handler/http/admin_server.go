package http

import (
	"fmt"
	"lumipluse-backend/internal/model"
	"lumipluse-backend/internal/pkg/utils"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ListServers 获取所有服务器列表
func (h *Handler) ListServers(c *gin.Context) {
	servers, err := h.Repo.ListServers(c.Request.Context())
	if err != nil {
		utils.Warn("list servers failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to fetch servers"})
		return
	}
	if servers == nil {
		servers = []*model.Server{}
	}
	c.JSON(http.StatusOK, model.APIResponse{Code: 200, Message: "ok", Data: servers})
}

// CreateServer 创建服务器
func (h *Handler) CreateServer(c *gin.Context) {
	var req model.CreateServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request body"})
		return
	}

	svr := &model.Server{
		Name:               req.Name,
		Description:        req.Description,
		AutoMerge:          req.AutoMerge == nil || *req.AutoMerge,
		AutoMergeThreshold: 1,
	}
	if req.AutoMergeThreshold > 0 {
		svr.AutoMergeThreshold = req.AutoMergeThreshold
	}
	if err := h.Repo.CreateServer(c.Request.Context(), svr); err != nil {
		utils.Error("create server failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to create server"})
		return
	}

	auditLog("server.create", fmt.Sprintf("name=%s", svr.Name))

	c.JSON(http.StatusCreated, model.APIResponse{
		Code:    201,
		Message: "Server created",
		Data:    svr,
	})
}

// UpdateServer 修改服务器
func (h *Handler) UpdateServer(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid server id"})
		return
	}

	svr, err := h.Repo.GetServer(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Server not found"})
		return
	}

	var req model.UpdateServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request body"})
		return
	}

	if req.Name != "" {
		svr.Name = req.Name
	}
	if req.Description != "" {
		svr.Description = req.Description
	}
	if req.AutoMerge != nil {
		svr.AutoMerge = *req.AutoMerge
	}
	if req.AutoMergeThreshold > 0 {
		svr.AutoMergeThreshold = req.AutoMergeThreshold
	}

	if err := h.Repo.UpdateServer(c.Request.Context(), svr); err != nil {
		utils.Error("update server %d failed: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to update server"})
		return
	}

	auditLog("server.update", fmt.Sprintf("id=%d name=%s", svr.ID, svr.Name))

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "Server updated",
		Data:    svr,
	})
}

// DeleteServer 删除服务器
func (h *Handler) DeleteServer(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid server id"})
		return
	}

	svr, _ := h.Repo.GetServer(c.Request.Context(), id)

	if err := h.Repo.DeleteServer(c.Request.Context(), id); err != nil {
		utils.Error("delete server %d failed: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to delete server"})
		return
	}

	if svr != nil {
		auditLog("server.delete", fmt.Sprintf("id=%d name=%s", id, svr.Name))
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Code:    200,
		Message: "Server deleted",
	})
}
