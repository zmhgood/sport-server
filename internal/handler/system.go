package handler

import (
	"net/http"

	"elderly-fitness/internal/service"

	"github.com/gin-gonic/gin"
)

type SystemHandler struct {
	systemService *service.SystemService
}

func NewSystemHandler(systemService *service.SystemService) *SystemHandler {
	return &SystemHandler{systemService: systemService}
}

// GetConfigs 获取所有配置
func (h *SystemHandler) GetConfigs(c *gin.Context) {
	configs, err := h.systemService.GetAllConfigs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Code: 500, Message: "获取配置失败"})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    configs,
	})
}

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	Key   string `json:"key" binding:"required"`
	Value string `json:"value"`
}

// UpdateConfig 更新配置
func (h *SystemHandler) UpdateConfig(c *gin.Context) {
	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "参数错误"})
		return
	}

	if err := h.systemService.SetConfig(req.Key, req.Value); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Code: 500, Message: "保存失败"})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "保存成功"})
}

// UpdateConfigsRequest 批量更新配置请求
type UpdateConfigsRequest struct {
	Configs map[string]string `json:"configs" binding:"required"`
}

// UpdateConfigs 批量更新配置
func (h *SystemHandler) UpdateConfigs(c *gin.Context) {
	var req UpdateConfigsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "参数错误: " + err.Error()})
		return
	}

	for key, value := range req.Configs {
		if err := h.systemService.SetConfig(key, value); err != nil {
			c.JSON(http.StatusInternalServerError, Response{Code: 500, Message: "保存失败: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "保存成功"})
}
