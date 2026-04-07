package handler

import (
	"net/http"
	"strconv"

	"elderly-fitness/internal/model"
	"elderly-fitness/internal/service"

	"github.com/gin-gonic/gin"
)

// ExerciseHandler 锻炼相关处理器
type ExerciseHandler struct {
	exerciseService *service.ExerciseService
}

// NewExerciseHandler 创建锻炼处理器
func NewExerciseHandler(exerciseService *service.ExerciseService) *ExerciseHandler {
	return &ExerciseHandler{
		exerciseService: exerciseService,
	}
}

// GetMuscleGroups 获取肌肉部位分组
func (h *ExerciseHandler) GetMuscleGroups(c *gin.Context) {
	groups, err := h.exerciseService.GetMuscleGroups()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: "获取肌肉分组失败",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    groups,
	})
}

// GetExercises 获取锻炼列表
func (h *ExerciseHandler) GetExercises(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	muscleGroupIDStr := c.Query("muscle_group_id")
	difficulty := c.Query("difficulty")
	keyword := c.Query("keyword")

	var muscleGroupID uint
	if muscleGroupIDStr != "" {
		id, err := strconv.ParseUint(muscleGroupIDStr, 10, 32)
		if err == nil {
			muscleGroupID = uint(id)
		}
	}

	exercises, total, err := h.exerciseService.GetExercisesList(page, pageSize, muscleGroupID, difficulty, keyword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: "获取锻炼列表失败",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"list":     exercises,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		},
	})
}

// GetExerciseDetail 获取锻炼详情
func (h *ExerciseHandler) GetExerciseDetail(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "参数错误",
		})
		return
	}

	exercise, err := h.exerciseService.GetExerciseByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    404,
			Message: "锻炼不存在",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    exercise,
	})
}

// GetRecommendExercises 获取推荐锻炼
func (h *ExerciseHandler) GetRecommendExercises(c *gin.Context) {
	userID, _ := c.Get("userID")
	
	exercises, err := h.exerciseService.GetRecommendExercises(userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: "获取推荐失败",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    exercises,
	})
}

// RecordExercise 记录锻炼
func (h *ExerciseHandler) RecordExercise(c *gin.Context) {
	c.JSON(http.StatusForbidden, Response{
		Code:    403,
		Message: "请先创建或加入家庭，并通过家庭目标完成锻炼",
	})
}

// CreateExercise 创建锻炼动作
func (h *ExerciseHandler) CreateExercise(c *gin.Context) {
	var req model.Exercise
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	if err := h.exerciseService.CreateExercise(&req); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: "创建失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "创建成功",
		Data:    req,
	})
}

// UpdateExercise 更新锻炼动作
func (h *ExerciseHandler) UpdateExercise(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "参数错误",
		})
		return
	}

	var req model.Exercise
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	req.ID = uint(id)
	if err := h.exerciseService.UpdateExercise(&req); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: "更新失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "更新成功",
		Data:    req,
	})
}

// DeleteExercise 删除锻炼动作
func (h *ExerciseHandler) DeleteExercise(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "参数错误",
		})
		return
	}

	if err := h.exerciseService.DeleteExercise(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: "删除失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "删除成功",
	})
}
