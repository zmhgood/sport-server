package handler

import (
	"log"
	"net/http"
	"strconv"

	"elderly-fitness/internal/service"

	"github.com/gin-gonic/gin"
)

// UserHandler 用户相关处理器
type UserHandler struct {
	userService *service.UserService
}

// NewUserHandler 创建用户处理器
func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

// GetStatistics 获取用户统计数据
func (h *UserHandler) GetStatistics(c *gin.Context) {
	userID, _ := c.Get("userID")

	stats, err := h.userService.GetUserStatistics(userID.(uint))
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

// GetHistory 获取锻炼历史
func (h *UserHandler) GetHistory(c *gin.Context) {
	userID, _ := c.Get("userID")

	// 获取分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	
	log.Printf("[GetHistory] userID=%v, page=%d, pageSize=%d", userID, page, pageSize)

	history, total, err := h.userService.GetExerciseHistoryPaginated(userID.(uint), page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: "获取历史失败",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"list":  history,
			"total": total,
			"page":  page,
		},
	})
}

// GetTodayProgress 获取今日进度
func (h *UserHandler) GetTodayProgress(c *gin.Context) {
	userID, _ := c.Get("userID")

	progress, err := h.userService.GetTodayProgress(userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: "获取进度失败",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    progress,
	})
}

// GetWeekStats 获取一周锻炼统计
func (h *UserHandler) GetWeekStats(c *gin.Context) {
	userID, _ := c.Get("userID")

	// 获取周偏移参数，0表示本周，-1表示上周，1表示下周
	weekOffset, _ := strconv.Atoi(c.DefaultQuery("week_offset", "0"))

	log.Printf("[GetWeekStats] userID=%v, weekOffset=%d", userID, weekOffset)

	result, err := h.userService.GetWeekStats(userID.(uint), weekOffset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: "获取周统计失败",
		})
		return
	}

	const serverVersion = "week-stats-v4"

	stats := make([]gin.H, 0, len(result.Stats))
	for _, s := range result.Stats {
		stats = append(stats, gin.H{
			"date":          s.Date,
			"dateStr":       s.DateStr,
			"weekday":       s.Weekday,
			"count":         s.Count,
			"totalSets":     s.TotalSets,
			"minutes":       s.Minutes,
			"goalId":        s.GoalID,
			"goalName":      s.GoalName,
			"goalIds":       s.GoalIDs,
			"serverVersion": serverVersion,
		})
	}

	goalTotalSets := 0
	if len(result.Stats) > 0 {
		goalTotalSets = result.Stats[0].TotalSets
		log.Printf("[GetWeekStats] sample stat: %+v", result.Stats[0])
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success-week-stats-v4",
		Data: gin.H{
			"stats":         stats,
			"weekRange":     result.WeekRange,
			"startDate":     result.StartDate,
			"weekOffset":    result.WeekOffset,
			"canGoPrev":     result.CanGoPrev,
			"canGoNext":     result.CanGoNext,
			"goalTotalSets": goalTotalSets,
			"serverVersion": serverVersion,
		},
	})
}
