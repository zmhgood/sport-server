package handler

import (
	"net/http"
	"strconv"

	"elderly-fitness/internal/service"

	"github.com/gin-gonic/gin"
)

type GoalHandler struct {
	goalService *service.GoalService
}

func NewGoalHandler(goalService *service.GoalService) *GoalHandler {
	return &GoalHandler{goalService: goalService}
}

// CreateGoal 创建目标
func (h *GoalHandler) CreateGoal(c *gin.Context) {
	userID, _ := c.Get("userID")

	var req service.CreateGoalInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "参数错误: " + err.Error()})
		return
	}

	// 从查询参数获取family_id
	familyIDStr := c.Query("family_id")
	familyID, err := strconv.ParseUint(familyIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "family_id参数错误"})
		return
	}

	goal, err := h.goalService.CreateGoal(uint(familyID), userID.(uint), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "创建成功", Data: goal})
}

// GetGoal 获取目标详情
func (h *GoalHandler) GetGoal(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "参数错误"})
		return
	}

	goal, err := h.goalService.GetGoal(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, Response{Code: 404, Message: "目标不存在"})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "success", Data: goal})
}

// GetFamilyGoals 获取家庭目标列表
func (h *GoalHandler) GetFamilyGoals(c *gin.Context) {
	familyIDStr := c.Query("family_id")
	familyID, err := strconv.ParseUint(familyIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "family_id参数错误"})
		return
	}

	goals, err := h.goalService.GetFamilyGoals(uint(familyID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Code: 500, Message: "获取失败"})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "success", Data: goals})
}

// GetMyGoals 获取我的目标列表
func (h *GoalHandler) GetMyGoals(c *gin.Context) {
	userID, _ := c.Get("userID")

	goals, err := h.goalService.GetUserGoalsWithProgress(userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Code: 500, Message: "获取失败"})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "success", Data: goals})
}

// UpdateGoal 更新目标
func (h *GoalHandler) UpdateGoal(c *gin.Context) {
	userID, _ := c.Get("userID")

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "参数错误"})
		return
	}

	var req service.UpdateGoalInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "参数错误"})
		return
	}

	goal, err := h.goalService.UpdateGoal(uint(id), userID.(uint), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "更新成功", Data: goal})
}

// DeleteGoal 删除目标
func (h *GoalHandler) DeleteGoal(c *gin.Context) {
	userID, _ := c.Get("userID")

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "参数错误"})
		return
	}

	if err := h.goalService.DeleteGoal(uint(id), userID.(uint)); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "删除成功"})
}

// AddGoalMember 添加目标成员
func (h *GoalHandler) AddGoalMember(c *gin.Context) {
	userID, _ := c.Get("userID")

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "参数错误"})
		return
	}

	var req struct {
		UserID uint `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "参数错误"})
		return
	}

	if err := h.goalService.AddGoalMember(uint(id), userID.(uint), req.UserID); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "添加成功"})
}

// AddGoalExercise 添加目标动作
func (h *GoalHandler) AddGoalExercise(c *gin.Context) {
	userID, _ := c.Get("userID")

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "参数错误"})
		return
	}

	var req struct {
		ExerciseID uint `json:"exercise_id" binding:"required"`
		Sets       int  `json:"sets"`
		Reps       int  `json:"reps"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "参数错误"})
		return
	}

	if err := h.goalService.AddGoalExercise(uint(id), userID.(uint), req.ExerciseID, req.Sets, req.Reps); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "添加成功"})
}

// CompleteExercise 完成动作
func (h *GoalHandler) CompleteExercise(c *gin.Context) {
	userID, _ := c.Get("userID")

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "参数错误"})
		return
	}

	var req service.CompleteExerciseInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "参数错误"})
		return
	}

	if err := h.goalService.CompleteExercise(uint(id), userID.(uint), req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "记录成功"})
}

// GetGoalProgress 获取目标进度
func (h *GoalHandler) GetGoalProgress(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "参数错误"})
		return
	}

	date := c.Query("date")

	progress, err := h.goalService.GetGoalProgress(uint(id), date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Code: 500, Message: "获取失败"})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "success", Data: progress})
}

// GetGoalHistory 获取目标历史
func (h *GoalHandler) GetGoalHistory(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "参数错误"})
		return
	}

	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	completions, err := h.goalService.GetGoalHistory(uint(id), startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Code: 500, Message: "获取失败"})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "success", Data: completions})
}

// GetUserFamiliesWithGoals 获取用户所有家庭及其目标
func (h *GoalHandler) GetUserFamiliesWithGoals(c *gin.Context) {
	userID, _ := c.Get("userID")

	familiesWithGoals, err := h.goalService.GetUserFamiliesWithGoals(userID.(uint))
	if err != nil {
		c.JSON(http.StatusOK, Response{Code: 0, Message: "获取成功", Data: []interface{}{}})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "success", Data: familiesWithGoals})
}
