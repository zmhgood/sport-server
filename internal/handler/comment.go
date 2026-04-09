package handler

import (
	"net/http"
	"strconv"

	"elderly-fitness/internal/service"

	"github.com/gin-gonic/gin"
)

type CommentHandler struct {
	commentService *service.CommentService
}

func NewCommentHandler(commentService *service.CommentService) *CommentHandler {
	return &CommentHandler{
		commentService: commentService,
	}
}

// GetComments 获取评论列表
func (h *CommentHandler) GetComments(c *gin.Context) {
	exerciseIDStr := c.Query("exercise_id")
	if exerciseIDStr == "" {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "缺少exercise_id参数",
		})
		return
	}

	exerciseID, _ := strconv.ParseUint(exerciseIDStr, 10, 32)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	// 获取用户ID（可能未登录）
	var userID uint
	if id, exists := c.Get("userID"); exists {
		userID = id.(uint)
	}

	result, err := h.commentService.GetCommentList(uint(exerciseID), userID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: "获取评论失败",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// CreateComment 创建评论
func (h *CommentHandler) CreateComment(c *gin.Context) {
	userID, _ := c.Get("userID")

	var req service.CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "参数错误",
		})
		return
	}

	comment, err := h.commentService.CreateComment(userID.(uint), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "评论成功",
		Data:    comment,
	})
}

// DeleteComment 删除评论
func (h *CommentHandler) DeleteComment(c *gin.Context) {
	userID, _ := c.Get("userID")

	commentIDStr := c.Param("id")
	commentID, _ := strconv.ParseUint(commentIDStr, 10, 32)

	if err := h.commentService.DeleteComment(uint(commentID), userID.(uint)); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "删除成功",
	})
}

// ToggleLike 点赞/取消点赞
func (h *CommentHandler) ToggleLike(c *gin.Context) {
	userID, _ := c.Get("userID")

	commentIDStr := c.Param("id")
	commentID, _ := strconv.ParseUint(commentIDStr, 10, 32)

	isLiked, err := h.commentService.ToggleLike(uint(commentID), userID.(uint))
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: err.Error(),
		})
		return
	}

	message := "取消点赞"
	if isLiked {
		message = "点赞成功"
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: message,
		Data: gin.H{
			"is_liked": isLiked,
		},
	})
}

// GetUserComments 获取用户的评论
func (h *CommentHandler) GetUserComments(c *gin.Context) {
	userID, _ := c.Get("userID")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	result, err := h.commentService.GetUserComments(userID.(uint), page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: "获取评论失败",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// GetAllComments 获取所有评论（管理后台用）
func (h *CommentHandler) GetAllComments(c *gin.Context) {
	status := -1
	if statusStr := c.Query("status"); statusStr != "" {
		status, _ = strconv.Atoi(statusStr)
	}
	keyword := c.Query("keyword")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	result, err := h.commentService.GetAllComments(status, keyword, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: "获取评论失败",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// UpdateCommentStatus 更新评论状态（管理后台用）
func (h *CommentHandler) UpdateCommentStatus(c *gin.Context) {
	commentIDStr := c.Param("id")
	commentID, err := strconv.ParseUint(commentIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "参数错误",
		})
		return
	}

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

	if err := h.commentService.UpdateCommentStatus(uint(commentID), req.Status); err != nil {
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

// AdminDeleteComment 管理员删除评论
func (h *CommentHandler) AdminDeleteComment(c *gin.Context) {
	commentIDStr := c.Param("id")
	commentID, err := strconv.ParseUint(commentIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "参数错误",
		})
		return
	}

	if err := h.commentService.AdminDeleteComment(uint(commentID)); err != nil {
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
