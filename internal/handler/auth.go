package handler

import (
	"net/http"

	"elderly-fitness/internal/service"

	"github.com/gin-gonic/gin"
)

// Response 统一响应格式
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// AuthHandler 认证处理器
type AuthHandler struct {
	authService *service.AuthService
	smsService  *service.SMSService
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(authService *service.AuthService, smsService *service.SMSService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		smsService:  smsService,
	}
}

// LoginRequest 登录请求
type LoginRequest struct {
	Code string `json:"code" binding:"required"`
}

// Login 微信登录
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "参数错误",
		})
		return
	}

	result, err := h.authService.WeChatLogin(req.Code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: "登录失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "登录成功",
		Data:    result,
	})
}

// SendSMSCodeRequest 发送验证码请求
type SendSMSCodeRequest struct {
	Phone   string `json:"phone" binding:"required"`
	Purpose string `json:"purpose"` // login, register, reset_password
}

// SendSMSCode 发送短信验证码
func (h *AuthHandler) SendSMSCode(c *gin.Context) {
	var req SendSMSCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "参数错误",
		})
		return
	}

	// 验证手机号格式
	if len(req.Phone) != 11 {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "手机号格式不正确",
		})
		return
	}

	// 默认为登录验证码
	purpose := req.Purpose
	if purpose == "" {
		purpose = "login"
	}

	if err := h.smsService.SendCode(req.Phone, purpose); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "验证码发送成功",
	})
}

// SMSLoginRequest 短信登录请求
type SMSLoginRequest struct {
	Phone string `json:"phone" binding:"required"`
	Code  string `json:"code" binding:"required"`
}

// SMSLogin 短信验证码登录
func (h *AuthHandler) SMSLogin(c *gin.Context) {
	var req SMSLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "参数错误",
		})
		return
	}

	result, err := h.authService.SMSLogin(req.Phone, req.Code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: "登录失败: " + err.Error(),
		})
		return
	}

	message := "登录成功"
	if result.IsNew {
		message = "注册成功"
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: message,
		Data:    result,
	})
}

// GetUserInfo 获取用户信息
func (h *AuthHandler) GetUserInfo(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    401,
			Message: "未登录",
		})
		return
	}

	user, err := h.authService.GetUserByID(userID.(uint))
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    404,
			Message: "用户不存在",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    user,
	})
}

// UpdateUserInfo 更新用户信息
func (h *AuthHandler) UpdateUserInfo(c *gin.Context) {
	userID, _ := c.Get("userID")

	var req struct {
		NickName     string `json:"nick_name"`
		AvatarURL    string `json:"avatar_url"`
		Age          int    `json:"age"`
		Phone        string `json:"phone"`
		HealthStatus string `json:"health_status"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "参数错误",
		})
		return
	}

	if err := h.authService.UpdateUserInfo(userID.(uint), &service.UserInfoUpdate{
		NickName:     req.NickName,
		AvatarURL:    req.AvatarURL,
		Age:          req.Age,
		Phone:        req.Phone,
		HealthStatus: req.HealthStatus,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: "更新失败",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "更新成功",
	})
}
