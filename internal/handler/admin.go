package handler

import (
	"net/http"
	"strconv"

	"elderly-fitness/internal/service"

	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	adminService *service.AdminService
	userService  *service.UserService
	commentRepo  interface {
		ListAllComments(page, pageSize int, status *int, keyword string) ([]interface{}, int64, error)
		UpdateCommentStatus(id uint, status int) error
		Delete(id uint) error
	}
}

func NewAdminHandler(adminService *service.AdminService, userService *service.UserService) *AdminHandler {
	return &AdminHandler{
		adminService: adminService,
		userService:  userService,
	}
}

// AdminLogin 管理员登录
func (h *AdminHandler) AdminLogin(c *gin.Context) {
	var req service.AdminLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "参数错误",
		})
		return
	}

	admin, token, err := h.adminService.AdminLogin(&req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    401,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "登录成功",
		Data: gin.H{
			"token": token,
			"admin": gin.H{
				"id":       admin.ID,
				"username": admin.Username,
				"nickname": admin.Nickname,
				"avatar":   admin.Avatar,
				"role":     admin.Role,
			},
		},
	})
}

// GetAdminInfo 获取管理员信息
func (h *AdminHandler) GetAdminInfo(c *gin.Context) {
	adminID, _ := c.Get("userID")
	id := adminID.(uint)

	admin, err := h.adminService.GetAdminByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    404,
			Message: "管理员不存在",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"id":       admin.ID,
			"username": admin.Username,
			"nickname": admin.Nickname,
			"avatar":   admin.Avatar,
			"role":     admin.Role,
		},
	})
}

// GetUsers 获取用户列表
func (h *AdminHandler) GetUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	keyword := c.Query("keyword")

	var gender *int
	if g := c.Query("gender"); g != "" {
		gv, _ := strconv.Atoi(g)
		gender = &gv
	}

	users, total, err := h.userService.GetUsers(page, pageSize, keyword, gender)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: "获取用户列表失败",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"list":     users,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		},
	})
}

// GetUserStats 获取用户统计数据
func (h *AdminHandler) GetUserStats(c *gin.Context) {
	idStr := c.Param("id")
	id, _ := strconv.ParseUint(idStr, 10, 32)

	stats, err := h.userService.GetUserStatistics(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: "获取用户统计失败",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"totalCount":    stats.TotalExercises,
			"totalDuration": stats.TotalMinutes,
			"totalCalories": 0, // 暂不支持热量统计
		},
	})
}

// GetComments 获取评论列表
func (h *AdminHandler) GetComments(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	keyword := c.Query("keyword")

	var status *int
	if s := c.Query("status"); s != "" {
		sv, _ := strconv.Atoi(s)
		status = &sv
	}

	comments, total, err := h.userService.GetComments(page, pageSize, status, keyword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: "获取评论列表失败",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"list":     comments,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		},
	})
}

// UpdateCommentStatus 更新评论状态
func (h *AdminHandler) UpdateCommentStatus(c *gin.Context) {
	idStr := c.Param("id")
	id, _ := strconv.ParseUint(idStr, 10, 32)

	var req struct {
		Status int `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "参数错误",
		})
		return
	}

	if err := h.userService.UpdateCommentStatus(uint(id), req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: "更新状态失败",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "更新成功",
	})
}

// DeleteComment 删除评论
func (h *AdminHandler) DeleteComment(c *gin.Context) {
	idStr := c.Param("id")
	id, _ := strconv.ParseUint(idStr, 10, 32)

	if err := h.userService.DeleteComment(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: "删除失败",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "删除成功",
	})
}

// GetDashboardStats 获取仪表盘统计
func (h *AdminHandler) GetDashboardStats(c *gin.Context) {
	stats, err := h.userService.GetDashboardStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: "获取统计失败",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}
