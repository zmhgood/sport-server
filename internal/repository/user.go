package repository

import (
	"elderly-fitness/pkg/clock"
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
	if user.Phone == "" {
		return r.db.Omit("Phone").Create(user).Error
	}
	return r.db.Create(user).Error
}

// Update 更新用户
func (r *UserRepository) Update(user *model.User) error {
	if user.Phone == "" {
		return r.db.Omit("Phone").Save(user).Error
	}
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
	return r.totalStatistics(userID)
}

type UserStatistics struct {
	TotalDays      int `json:"totalDays"`
	TotalExercises int `json:"totalExercises"`
	TotalMinutes   int `json:"totalMinutes"`
	TotalSets      int `json:"totalSets"`
}

// GetUserTodayGoalCompletionCount 获取用户当天目标完成记录数
func (r *UserRepository) GetUserTodayGoalCompletionCount(userID uint, date string) (int, error) {
	return r.completedToday(date, &userID)
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
		JOIN families f ON f.id=g.family_id AND f.deleted_at IS NULL
 JOIN family_members fm ON fm.family_id=g.family_id AND fm.user_id=gm.user_id AND fm.deleted_at IS NULL
 WHERE gm.user_id = ? AND gm.deleted_at IS NULL AND g.is_active=true
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
	return r.progressRows(userID, limit)
}

func (r *UserRepository) GetWeekCompletions(userID uint, startDate string) ([]GoalCompletionWithGoal, error) {
	start, err := time.ParseInLocation("2006-01-02", startDate, clock.Zone)
	if err != nil {
		return nil, err
	}
	rows, err := r.progressRows(userID, 0)
	if err != nil {
		return nil, err
	}
	end := start.AddDate(0, 0, 7)
	result := []GoalCompletionWithGoal{}
	for _, row := range rows {
		if !row.Date.Before(start) && row.Date.Before(end) {
			result = append(result, row)
		}
	}
	return result, nil
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
