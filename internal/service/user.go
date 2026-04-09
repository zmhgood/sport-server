package service

import (
	"fmt"
	"log"
	"sort"
	"time"

	"elderly-fitness/internal/model"
	"elderly-fitness/internal/repository"
)

type UserService struct {
	userRepo     *repository.UserRepository
	exerciseRepo *repository.ExerciseRepository
	commentRepo  *repository.CommentRepository
}

func NewUserService(userRepo *repository.UserRepository, exerciseRepo *repository.ExerciseRepository) *UserService {
	return &UserService{
		userRepo:     userRepo,
		exerciseRepo: exerciseRepo,
	}
}

// SetCommentRepo 设置评论仓库
func (s *UserService) SetCommentRepo(repo *repository.CommentRepository) {
	s.commentRepo = repo
}

// UserStatistics 用户统计
type UserStatistics struct {
	TotalDays      int `json:"totalDays"`
	TotalExercises int `json:"totalExercises"`
	TotalMinutes   int `json:"totalMinutes"`
	TotalSets      int `json:"totalSets"`
}

// GetUserStatistics 获取用户统计数据
func (s *UserService) GetUserStatistics(userID uint) (*repository.UserStatistics, error) {
	return s.userRepo.GetUserTotalStats(userID)
}

// ExerciseHistory 锻炼历史
type ExerciseHistory struct {
	Date         time.Time      `json:"date"`
	GoalID       uint           `json:"goal_id"`
	GoalName     string         `json:"goal_name"`
	Exercises    []ExerciseItem `json:"exercises"`
	Duration     int            `json:"duration"`
	Progress     int            `json:"progress"`     // 完成进度百分比
	CompletedNum int            `json:"completed_num"` // 已完成动作数
	TotalNum     int            `json:"total_num"`     // 总动作数
}

// ExerciseItem 锻炼项
type ExerciseItem struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// GetExerciseHistory 获取锻炼历史（按目标+日期分组）
func (s *UserService) GetExerciseHistory(userID uint, limit int) ([]ExerciseHistory, error) {
	// 获取用户目标完成记录（带目标信息）
	goalCompletions, err := s.userRepo.GetUserGoalCompletionsWithGoal(userID, limit)
	if err != nil {
		goalCompletions = []repository.GoalCompletionWithGoal{}
	}
	log.Printf("[GetExerciseHistory] 用户 %d 的目标完成记录数: %d", userID, len(goalCompletions))

	// 按日期+目标分组
	historyMap := make(map[string]*ExerciseHistory)

	// 处理目标完成记录
	for _, completion := range goalCompletions {
		key := fmt.Sprintf("%s-%d", completion.Date.Format("2006-01-02"), completion.GoalID)
		if _, exists := historyMap[key]; !exists {
			historyMap[key] = &ExerciseHistory{
				Date:     completion.Date,
				GoalID:   completion.GoalID,
				GoalName: completion.GoalName,
				Duration: 0,
				TotalNum: completion.TotalExercises,
			}
		}
		// 避免重复添加同名锻炼
		exists := false
		for _, ex := range historyMap[key].Exercises {
			if ex.ID == completion.ExerciseID {
				exists = true
				break
			}
		}
		if !exists {
			historyMap[key].Exercises = append(historyMap[key].Exercises, ExerciseItem{
				ID:   completion.ExerciseID,
				Name: completion.ExerciseName,
			})
			historyMap[key].CompletedNum++
		}
		// 目标完成按每组30秒估算
		historyMap[key].Duration += completion.CompletedSets * 30
		// 计算进度百分比
		if historyMap[key].TotalNum > 0 {
			historyMap[key].Progress = historyMap[key].CompletedNum * 100 / historyMap[key].TotalNum
		}
	}

	// 转换为切片并排序
	result := make([]ExerciseHistory, 0, len(historyMap))
	for _, h := range historyMap {
		result = append(result, *h)
	}

	// 按日期降序排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Date.After(result[j].Date)
	})

	// 限制数量
	if len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}

// GetExerciseHistoryPaginated 获取锻炼历史（分页）
func (s *UserService) GetExerciseHistoryPaginated(userID uint, page, pageSize int) ([]ExerciseHistory, int, error) {
	log.Printf("[GetExerciseHistoryPaginated] userID=%d, page=%d, pageSize=%d", userID, page, pageSize)

	// 获取所有目标完成记录（设置一个较大的 limit）
	goalCompletions, err := s.userRepo.GetUserGoalCompletionsWithGoal(userID, 1000)
	if err != nil {
		goalCompletions = []repository.GoalCompletionWithGoal{}
	}
	log.Printf("[GetExerciseHistoryPaginated] GetUserGoalCompletionsWithGoal count: %d", len(goalCompletions))

	// 按日期+目标分组
	historyMap := make(map[string]*ExerciseHistory)

	// 处理目标完成记录
	for _, completion := range goalCompletions {
		key := fmt.Sprintf("%s-%d", completion.Date.Format("2006-01-02"), completion.GoalID)
		if _, exists := historyMap[key]; !exists {
			historyMap[key] = &ExerciseHistory{
				Date:     completion.Date,
				GoalID:   completion.GoalID,
				GoalName: completion.GoalName,
				Duration: 0,
				TotalNum: completion.TotalExercises,
			}
		}
		exists := false
		for _, ex := range historyMap[key].Exercises {
			if ex.ID == completion.ExerciseID {
				exists = true
				break
			}
		}
		if !exists {
			historyMap[key].Exercises = append(historyMap[key].Exercises, ExerciseItem{
				ID:   completion.ExerciseID,
				Name: completion.ExerciseName,
			})
			historyMap[key].CompletedNum++
		}
		historyMap[key].Duration += completion.CompletedSets * 30
		if historyMap[key].TotalNum > 0 {
			historyMap[key].Progress = historyMap[key].CompletedNum * 100 / historyMap[key].TotalNum
		}
	}


	// 转换为切片并排序
	result := make([]ExerciseHistory, 0, len(historyMap))
	for _, h := range historyMap {
		result = append(result, *h)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Date.After(result[j].Date)
	})

	log.Printf("[GetExerciseHistoryPaginated] After grouping, total records: %d", len(result))

	// 计算分页
	total := len(result)
	start := (page - 1) * pageSize
	if start >= total {
		log.Printf("[GetExerciseHistoryPaginated] start(%d) >= total(%d), return empty", start, total)
		return []ExerciseHistory{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	log.Printf("[GetExerciseHistoryPaginated] Returning records [%d:%d] of %d", start, end, total)
	return result[start:end], total, nil
}

// TodayProgress 今日进度
type TodayProgress struct {
	Completed int `json:"completed"`
	Total     int `json:"total"`
}

// GetTodayProgress 获取今日进度
func (s *UserService) GetTodayProgress(userID uint) (*TodayProgress, error) {
	today := time.Now().Format("2006-01-02")
	count, err := s.userRepo.GetUserTodayGoalCompletionCount(userID, today)
	if err != nil {
		return nil, err
	}

	return &TodayProgress{
		Completed: count,
		Total:     3, // 建议每日锻炼数
	}, nil
}

// GetUsers 获取用户列表（管理后台）
func (s *UserService) GetUsers(page, pageSize int, keyword string, gender *int) ([]model.User, int64, error) {
	return s.userRepo.ListUsers(page, pageSize, keyword, gender)
}

// GetComments 获取评论列表（管理后台）
func (s *UserService) GetComments(page, pageSize int, status *int, keyword string) ([]model.Comment, int64, error) {
	if s.commentRepo == nil {
		return nil, 0, nil
	}
	statusVal := -1
	if status != nil {
		statusVal = *status
	}
	return s.commentRepo.GetAllComments(statusVal, keyword, page, pageSize)
}

// UpdateCommentStatus 更新评论状态
func (s *UserService) UpdateCommentStatus(id uint, status int) error {
	if s.commentRepo == nil {
		return nil
	}
	comment := &model.Comment{Status: status}
	comment.ID = id
	return s.commentRepo.Update(comment)
}

// DeleteComment 删除评论
func (s *UserService) DeleteComment(id uint) error {
	if s.commentRepo == nil {
		return nil
	}
	return s.commentRepo.Delete(id)
}

// DashboardStats 仪表盘统计
type DashboardStats struct {
	UserCount       int64 `json:"userCount"`
	ExerciseCount   int64 `json:"exerciseCount"`
	CommentCount    int64 `json:"commentCount"`
	TodayRecordCount int64 `json:"todayRecordCount"`
}

// WeekStatsResult 周统计结果
type WeekStatsResult struct {
	Stats       []DailyStat `json:"stats"`
	WeekRange   string      `json:"weekRange"`
	StartDate   string      `json:"startDate"`
	WeekOffset  int         `json:"weekOffset"`
	CanGoPrev   bool        `json:"canGoPrev"`  // 是否可以向左滑动（查看更早的周）
	CanGoNext   bool        `json:"canGoNext"`  // 是否可以向右滑动（查看更新的周）
}

// GetWeekStats 获取一周锻炼统计
func (s *UserService) GetWeekStats(userID uint, weekOffset int) (*WeekStatsResult, error) {
	// 获取指定周的数据，从周一开始
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	// 计算本周一的日期，然后根据偏移量计算目标周的周一
	monday := now.AddDate(0, 0, -weekday+1+weekOffset*7)
	startDate := monday.Format("2006-01-02")

	// 获取目标完成记录
	completions, err := s.userRepo.GetWeekCompletions(userID, startDate)
	if err != nil {
		return nil, err
	}

	// 获取用户家庭目标总组数（柱状图分母）
	goalTotalSets, err := s.userRepo.GetUserGoalTotalSets(userID)
	if err != nil {
		goalTotalSets = 0
	}
	log.Printf("[GetWeekStats] userID=%d goalTotalSets=%d err=%v", userID, goalTotalSets, err)

	// 按日期初始化
	dailyMap := make(map[string]*DailyStat)
	weekdays := []string{"一", "二", "三", "四", "五", "六", "日"}
	for i := 0; i < 7; i++ {
		date := monday.AddDate(0, 0, i).Format("2006-01-02")
		dateObj := monday.AddDate(0, 0, i)
		dailyMap[date] = &DailyStat{
			Date:      date,
			DateStr:   dateObj.Format("01/02"),
			Weekday:   "周" + weekdays[i],
			Count:     0,
			TotalSets: goalTotalSets,
			Minutes:   0,
			GoalID:    0,
			GoalName:  "",
			GoalIDs:   []uint{},
		}
	}

	// 统计每天完成的组数（Count）
	goalIDsByDate := make(map[string]map[uint]bool)
	for _, c := range completions {
		dateStr := c.Date.Format("2006-01-02")
		if stat, ok := dailyMap[dateStr]; ok {
			// 累计锻炼分钟数（按组数估算）
			stat.Minutes += c.CompletedSets * 30 / 60 // 每组30秒，转为分钟

			// 统计当天完成组数
			stat.Count += c.CompletedSets

			// 记录该日期参与的目标（去重）
			if _, ok := goalIDsByDate[dateStr]; !ok {
				goalIDsByDate[dateStr] = map[uint]bool{}
			}
			if !goalIDsByDate[dateStr][c.GoalID] {
				goalIDsByDate[dateStr][c.GoalID] = true
				stat.GoalIDs = append(stat.GoalIDs, c.GoalID)
				if stat.GoalID == 0 {
					stat.GoalID = c.GoalID
					stat.GoalName = c.GoalName
				}
			}
		}
	}

	// 转换为数组，按日期排序（周一到周日）
	result := make([]DailyStat, 0, 7)
	for i := 0; i < 7; i++ {
		date := monday.AddDate(0, 0, i).Format("2006-01-02")
		result = append(result, *dailyMap[date])
	}

	// 计算周的日期范围描述
	monthStart := monday.Format("1月2日")
	monthEnd := monday.AddDate(0, 0, 6).Format("1月2日")
	weekRange := monthStart + " - " + monthEnd

	// 判断是否可以继续滑动
	// canGoNext: 只有当前周的offset < 0时，才能向右滑动（回到未来/本周）
	canGoNext := weekOffset < 0
	
	// canGoPrev: 允许向过去滑动，但最多回看2年（方便测试）
	twoYearsAgo := time.Now().AddDate(-2, 0, 0).Format("2006-01-02")
	canGoPrev := startDate > twoYearsAgo

	log.Printf("[GetWeekStats] weekOffset=%d, startDate=%s, twoYearsAgo=%s, canGoPrev=%v, canGoNext=%v", 
		weekOffset, startDate, twoYearsAgo, canGoPrev, canGoNext)

	return &WeekStatsResult{
		Stats:      result,
		WeekRange:  weekRange,
		StartDate:  startDate,
		WeekOffset: weekOffset,
		CanGoPrev:  canGoPrev,
		CanGoNext:  canGoNext,
	}, nil
}

// DailyStat 每日统计
type DailyStat struct {
	Date      string `json:"date"`
	DateStr   string `json:"dateStr"`
	Weekday   string `json:"weekday"`
	Count     int    `json:"count"`
	TotalSets int    `json:"totalSets"`
	Minutes   int    `json:"minutes"`
	GoalID    uint   `json:"goalId"`
	GoalName  string `json:"goalName"`
	GoalIDs   []uint `json:"goalIds"` // 当天参与的所有目标ID
}

// GetDashboardStats 获取仪表盘统计
func (s *UserService) GetDashboardStats() (*DashboardStats, error) {
	userCount, _ := s.userRepo.Count()
	exerciseCount, _ := s.exerciseRepo.Count()
	commentCount, _ := s.commentRepo.Count()
	todayRecordCount, _ := s.exerciseRepo.GetTodayAllUsersExerciseCount()
	
	return &DashboardStats{
		UserCount:        userCount,
		ExerciseCount:    exerciseCount,
		CommentCount:     commentCount,
		TodayRecordCount: todayRecordCount,
	}, nil
}
