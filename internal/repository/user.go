package repository

import (
	"log"
	"time"

	"elderly-fitness/internal/model"

	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create 创建用户
func (r *UserRepository) Create(user *model.User) error {
	return r.db.Create(user).Error
}

// Update 更新用户
func (r *UserRepository) Update(user *model.User) error {
	return r.db.Save(user).Error
}

// FindByID 根据ID查找用户
func (r *UserRepository) FindByID(id uint) (*model.User, error) {
	var user model.User
	err := r.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByOpenID 根据OpenID查找用户
func (r *UserRepository) FindByOpenID(openID string) (*model.User, error) {
	var user model.User
	err := r.db.Where("openid = ?", openID).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByPhone 根据手机号查找用户
func (r *UserRepository) FindByPhone(phone string) (*model.User, error) {
	var user model.User
	err := r.db.Where("phone = ?", phone).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// UserStats 用户统计
type UserStats struct {
	TotalDays      int64
	TotalExercises int64
	TotalMinutes   int64
}

// GetUserTotalStats 获取用户总统计（仅家庭目标完成）
func (r *UserRepository) GetUserTotalStats(userID uint) (*UserStatistics, error) {
	var stats UserStatistics

	// 总锻炼天数（目标完成去重）
	var totalDaysResult int64
	r.db.Raw(`
		SELECT COUNT(DISTINCT DATE(gc.date))
		FROM goal_completions gc
		JOIN goal_members gm ON gc.goal_member_id = gm.id
		WHERE gm.user_id = ?
	`, userID).Scan(&totalDaysResult)
	stats.TotalDays = int(totalDaysResult)

	// 目标完成统计
	var goalAgg struct {
		TotalSeconds int
		TotalSets    int
		TotalCount   int64
	}
	r.db.Raw(`
		SELECT
			COALESCE(SUM(completed_sets * 30), 0) AS total_seconds,
			COALESCE(SUM(completed_sets), 0) AS total_sets,
			COALESCE(COUNT(*), 0) AS total_count
		FROM goal_completions gc
		JOIN goal_members gm ON gc.goal_member_id = gm.id
		WHERE gm.user_id = ?
	`, userID).Scan(&goalAgg)

	stats.TotalExercises = int(goalAgg.TotalCount)
	stats.TotalMinutes = goalAgg.TotalSeconds / 60 // 转换为分钟
	stats.TotalSets = goalAgg.TotalSets

	return &stats, nil
}

type UserStatistics struct {
	TotalDays      int `json:"totalDays"`
	TotalExercises int `json:"totalExercises"`
	TotalMinutes   int `json:"totalMinutes"`
	TotalSets      int `json:"totalSets"`
}

// GetUserTodayGoalCompletionCount 获取用户当天目标完成记录数
func (r *UserRepository) GetUserTodayGoalCompletionCount(userID uint, date string) (int, error) {
	var count int64
	err := r.db.Raw(`
		SELECT COUNT(*)
		FROM goal_completions gc
		JOIN goal_members gm ON gc.goal_member_id = gm.id
		WHERE gm.user_id = ? AND DATE(gc.date) = ?
	`, userID, date).Scan(&count).Error
	return int(count), err
}

// GetUserGoalTotalSets 获取用户家庭目标总组数
func (r *UserRepository) GetUserGoalTotalSets(userID uint) (int, error) {
	var total int64
	err := r.db.Raw(`
		SELECT COALESCE(SUM(COALESCE(NULLIF(ge.sets, 0), e.sets, 0)), 0)
		FROM goal_members gm
		JOIN daily_goals g ON gm.goal_id = g.id AND g.deleted_at IS NULL
		JOIN goal_exercises ge ON ge.goal_id = g.id AND ge.deleted_at IS NULL
		JOIN exercises e ON ge.exercise_id = e.id
		WHERE gm.user_id = ?
	`, userID).Scan(&total).Error
	log.Printf("[GetUserGoalTotalSets] userID=%d total=%d err=%v", userID, total, err)
	return int(total), err
}

// ListUsers 获取用户列表（管理后台用）
func (r *UserRepository) ListUsers(page, pageSize int, keyword string, gender *int) ([]model.User, int64, error) {
	var users []model.User
	var total int64

	query := r.db.Model(&model.User{})
	if keyword != "" {
		query = query.Where("nick_name LIKE ? OR phone LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if gender != nil {
		query = query.Where("gender = ?", *gender)
	}

	query.Count(&total)
	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&users).Error
	return users, total, err
}

// Count 获取用户总数
func (r *UserRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&model.User{}).Count(&count).Error
	return count, err
}

// GetUserGoalCompletions 获取用户的目标完成记录
func (r *UserRepository) GetUserGoalCompletions(userID uint, limit int) ([]model.GoalCompletion, error) {
	var completions []model.GoalCompletion

	// 先获取用户的 GoalMember IDs
	var goalMemberIDs []uint
	r.db.Model(&model.GoalMember{}).
		Where("user_id = ?", userID).
		Pluck("id", &goalMemberIDs)

	if len(goalMemberIDs) == 0 {
		return completions, nil
	}

	// 查询完成记录
	err := r.db.Model(&model.GoalCompletion{}).
		Where("goal_member_id IN ?", goalMemberIDs).
		Where("status = ?", "done").
		Preload("GoalExercise.Exercise").
		Order("date DESC").
		Limit(limit).
		Find(&completions).Error

	return completions, err
}

// GoalCompletionWithGoal 目标完成记录（带目标信息）
type GoalCompletionWithGoal struct {
	Date           time.Time
	GoalID         uint
	GoalName       string
	ExerciseID     uint
	ExerciseName   string
	CompletedSets  int
	TargetSets     int
	GoalTotalSets  int
	TotalExercises int // 该目标的总动作数
}

// GetUserGoalCompletionsWithGoal 获取用户的目标完成记录（带目标信息）
func (r *UserRepository) GetUserGoalCompletionsWithGoal(userID uint, limit int) ([]GoalCompletionWithGoal, error) {
	var results []GoalCompletionWithGoal

	// 联表查询：goal_completions -> goal_members -> goals, goal_exercises -> exercises
	// 同时统计每个目标的动作总数与总组数
	err := r.db.Table("goal_completions gc").
		Select("gc.date, g.id as goal_id, g.name as goal_name, e.id as exercise_id, e.name as exercise_name, gc.completed_sets, COALESCE(NULLIF(ge.sets, 0), e.sets, 0) as target_sets, ge_sum.total_exercises, ge_sum.total_sets as goal_total_sets").
		Joins("JOIN goal_members gm ON gc.goal_member_id = gm.id").
		Joins("JOIN daily_goals g ON gm.goal_id = g.id").
		Joins("JOIN goal_exercises ge ON gc.goal_exercise_id = ge.id").
		Joins("JOIN exercises e ON ge.exercise_id = e.id").
		Joins("JOIN (SELECT ge.goal_id, COUNT(*) as total_exercises, SUM(COALESCE(NULLIF(ge.sets, 0), e2.sets, 0)) as total_sets FROM goal_exercises ge JOIN exercises e2 ON ge.exercise_id = e2.id WHERE ge.deleted_at IS NULL GROUP BY ge.goal_id) ge_sum ON g.id = ge_sum.goal_id").
		Where("gm.user_id = ?", userID).
		Order("gc.date DESC").
		Limit(limit).
		Scan(&results).Error

	return results, err
}

// GetWeekCompletions 获取用户一周内的目标完成记录
func (r *UserRepository) GetWeekCompletions(userID uint, startDate string) ([]GoalCompletionWithGoal, error) {
	var results []GoalCompletionWithGoal

	// 计算一周的结束日期（周一 + 6天 = 周日）
	start, _ := time.Parse("2006-01-02", startDate)
	endDate := start.AddDate(0, 0, 6).Format("2006-01-02")

	err := r.db.Table("goal_completions gc").
		Select("gc.date, g.id as goal_id, g.name as goal_name, e.id as exercise_id, e.name as exercise_name, gc.completed_sets, COALESCE(NULLIF(ge.sets, 0), e.sets, 0) as target_sets, ge_sum.total_exercises, ge_sum.total_sets as goal_total_sets").
		Joins("JOIN goal_members gm ON gc.goal_member_id = gm.id").
		Joins("JOIN daily_goals g ON gm.goal_id = g.id").
		Joins("JOIN goal_exercises ge ON gc.goal_exercise_id = ge.id").
		Joins("JOIN exercises e ON ge.exercise_id = e.id").
		Joins("JOIN (SELECT ge.goal_id, COUNT(*) as total_exercises, SUM(COALESCE(NULLIF(ge.sets, 0), e2.sets, 0)) as total_sets FROM goal_exercises ge JOIN exercises e2 ON ge.exercise_id = e2.id WHERE ge.deleted_at IS NULL GROUP BY ge.goal_id) ge_sum ON g.id = ge_sum.goal_id").
		Where("gm.user_id = ?", userID).
		Where("gc.date >= ?", startDate).
		Where("gc.date <= ?", endDate).
		Order("gc.date DESC").
		Scan(&results).Error

	return results, err
}

// GetEarliestExerciseDate 获取用户最早的锻炼记录日期
func (r *UserRepository) GetEarliestExerciseDate(userID uint) (string, error) {
	var earliestDate string
	
	// 从 goal_completions 表获取最早记录
	err := r.db.Table("goal_completions gc").
		Select("MIN(DATE(gc.date))").
		Joins("JOIN goal_members gm ON gc.goal_member_id = gm.id").
		Where("gm.user_id = ?", userID).
		Scan(&earliestDate).Error

	if err != nil {
		return "", err
	}
	
	return earliestDate, nil
}
