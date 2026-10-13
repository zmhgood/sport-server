package repository

import (
	"errors"
	"gorm.io/gorm/clause"
	"time"

	"elderly-fitness/internal/model"

	"gorm.io/gorm"
)

type GoalRepository struct {
	db *gorm.DB
}

func NewGoalRepository(db *gorm.DB) *GoalRepository {
	return &GoalRepository{db: db}
}

// CreateGoal 创建目标
func (r *GoalRepository) CreateGoal(goal *model.DailyGoal) error {
	return r.db.Create(goal).Error
}

// UpdateGoal 更新目标
func (r *GoalRepository) UpdateGoal(goal *model.DailyGoal) error {
	return r.db.Save(goal).Error
}

// FindGoalByID 根据ID查找目标
func (r *GoalRepository) FindGoalByID(id uint) (*model.DailyGoal, error) {
	var goal model.DailyGoal
	err := r.db.Preload("Members.User", func(db *gorm.DB) *gorm.DB { return db.Select("id", "nick_name", "avatar_url") }).
		Preload("Exercises.Exercise").
		First(&goal, id).Error
	if err != nil {
		return nil, err
	}
	return &goal, nil
}

// GetGoalsByFamilyID 获取家庭的目标列表
func (r *GoalRepository) GetGoalsByFamilyID(familyID uint) ([]model.DailyGoal, error) {
	var goals []model.DailyGoal
	err := r.db.Where("family_id = ? AND is_active = ?", familyID, true).
		Preload("Members.User", func(db *gorm.DB) *gorm.DB { return db.Select("id", "nick_name", "avatar_url") }).
		Preload("Exercises.Exercise").
		Order("created_at DESC").
		Find(&goals).Error
	return goals, err
}

// GetGoalsByUserID 获取用户参与的目标列表
func (r *GoalRepository) GetGoalsByUserID(userID uint) ([]model.DailyGoal, error) {
	var goals []model.DailyGoal
	err := r.db.Joins("JOIN goal_members ON goal_members.goal_id = daily_goals.id").
		Joins("JOIN families ON families.id = daily_goals.family_id AND families.deleted_at IS NULL").
		Joins("JOIN family_members ON family_members.family_id = daily_goals.family_id AND family_members.user_id = goal_members.user_id AND family_members.deleted_at IS NULL").
		Where("goal_members.user_id = ? AND goal_members.deleted_at IS NULL AND daily_goals.is_active = ?", userID, true).
		Preload("Members.User", func(db *gorm.DB) *gorm.DB { return db.Select("id", "nick_name", "avatar_url") }).
		Preload("Exercises.Exercise").
		Order("daily_goals.created_at DESC").
		Find(&goals).Error
	return goals, err
}

// AddGoalMember 添加目标成员
func (r *GoalRepository) AddGoalMember(member *model.GoalMember) error {
	return r.db.Create(member).Error
}

// RemoveGoalMember 移除目标成员
func (r *GoalRepository) RemoveGoalMember(goalID, userID uint) error {
	return r.db.Where("goal_id = ? AND user_id = ?", goalID, userID).
		Delete(&model.GoalMember{}).Error
}

// AddGoalExercise 添加目标动作
func (r *GoalRepository) AddGoalExercise(exercise *model.GoalExercise) error {
	return r.db.Create(exercise).Error
}

// RemoveGoalExercise 移除目标动作
func (r *GoalRepository) RemoveGoalExercise(goalID, exerciseID uint) error {
	return r.db.Where("goal_id = ? AND id = ?", goalID, exerciseID).
		Delete(&model.GoalExercise{}).Error
}

// CreateCompletion 创建完成记录
func (r *GoalRepository) CreateCompletion(completion *model.GoalCompletion) error {
	return r.db.Create(completion).Error
}

// UpdateCompletion 更新完成记录
func (r *GoalRepository) UpdateCompletion(completion *model.GoalCompletion) error {
	return r.db.Save(completion).Error
}

// GetCompletion 获取指定日期的完成记录
func (r *GoalRepository) GetCompletion(goalMemberID, goalExerciseID uint, date string) (*model.GoalCompletion, error) {
	var completion model.GoalCompletion
	err := r.db.Where("goal_member_id = ? AND goal_exercise_id = ? AND date = ?",
		goalMemberID, goalExerciseID, date).First(&completion).Error
	if err != nil {
		return nil, err
	}
	return &completion, nil
}

// GetCompletionsByDate 获取指定日期的所有完成记录
func (r *GoalRepository) GetCompletionsByDate(goalID uint, date string) ([]model.GoalCompletion, error) {
	var completions []model.GoalCompletion
	err := r.db.Joins("JOIN (SELECT MAX(id) id FROM goal_completions WHERE deleted_at IS NULL GROUP BY goal_member_id,goal_exercise_id,date) latest ON latest.id=goal_completions.id").Joins("JOIN goal_members ON goal_members.id = goal_completions.goal_member_id").
		Joins("JOIN goal_exercises ON goal_exercises.id = goal_completions.goal_exercise_id AND goal_exercises.goal_id=goal_members.goal_id").
		Where("goal_members.goal_id = ? AND goal_exercises.goal_id = ? AND goal_completions.date = ?",
			goalID, goalID, date).
		Preload("GoalMember.User").
		Preload("GoalExercise.Exercise").
		Find(&completions).Error
	return completions, err
}

// GetMemberCompletions 获取成员在指定日期的完成记录
func (r *GoalRepository) GetMemberCompletions(goalID, userID uint, date string) ([]model.GoalCompletion, error) {
	var completions []model.GoalCompletion
	err := r.db.Joins("JOIN (SELECT MAX(id) id FROM goal_completions WHERE deleted_at IS NULL GROUP BY goal_member_id,goal_exercise_id,date) latest ON latest.id=goal_completions.id").Joins("JOIN goal_members ON goal_members.id = goal_completions.goal_member_id").
		Joins("JOIN goal_exercises ON goal_exercises.id = goal_completions.goal_exercise_id AND goal_exercises.goal_id=goal_members.goal_id").
		Where("goal_members.goal_id = ? AND goal_members.user_id = ? AND goal_completions.date = ?",
			goalID, userID, date).
		Preload("GoalExercise.Exercise").
		Find(&completions).Error
	return completions, err
}

// GetOrCreateCompletion 获取或创建完成记录
func (r *GoalRepository) GetOrCreateCompletion(goalMemberID, goalExerciseID uint, date string) (*model.GoalCompletion, error) {
	completion, err := r.GetCompletion(goalMemberID, goalExerciseID, date)
	if err == nil {
		return completion, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// 不存在则创建
	completion = &model.GoalCompletion{
		GoalMemberID:   goalMemberID,
		GoalExerciseID: goalExerciseID,
		Date:           parseDate(date),
		Status:         "pending",
	}
	if err := r.CreateCompletion(completion); err != nil {
		return nil, err
	}
	return completion, nil
}

// DeleteGoal 删除目标
func (r *GoalRepository) DeleteGoal(id uint) error {
	return r.db.Delete(&model.DailyGoal{}, id).Error
}

// GetGoalMember 获取目标成员
func (r *GoalRepository) GetGoalMember(goalID, userID uint) (*model.GoalMember, error) {
	var member model.GoalMember
	err := r.db.Where("goal_id = ? AND user_id = ?", goalID, userID).First(&member).Error
	if err != nil {
		return nil, err
	}
	return &member, nil
}

// GetGoalExercise 获取目标动作
func (r *GoalRepository) GetGoalExercise(id uint) (*model.GoalExercise, error) {
	var exercise model.GoalExercise
	err := r.db.First(&exercise, id).Error
	if err != nil {
		return nil, err
	}
	return &exercise, nil
}

// GetHistoryCompletions 获取历史完成记录
func (r *GoalRepository) GetHistoryCompletions(goalID uint, startDate, endDate string) ([]model.GoalCompletion, error) {
	var completions []model.GoalCompletion
	err := r.db.Joins("JOIN (SELECT MAX(id) id FROM goal_completions WHERE deleted_at IS NULL GROUP BY goal_member_id,goal_exercise_id,date) latest ON latest.id=goal_completions.id").Joins("JOIN goal_members ON goal_members.id = goal_completions.goal_member_id").
		Joins("JOIN goal_exercises ON goal_exercises.id = goal_completions.goal_exercise_id AND goal_exercises.goal_id=goal_members.goal_id").
		Where("goal_members.goal_id = ? AND goal_exercises.goal_id = ? AND goal_completions.date >= ? AND goal_completions.date <= ?",
			goalID, goalID, startDate, endDate).
		Preload("GoalMember.User").
		Preload("GoalExercise.Exercise").
		Order("goal_completions.date DESC").
		Find(&completions).Error
	return completions, err
}

func parseDate(dateStr string) time.Time {
	t, _ := time.Parse("2006-01-02", dateStr)
	return t
}

func (r *GoalRepository) DB() *gorm.DB { return r.db }
func (r *GoalRepository) CreateWithRelations(goal *model.DailyGoal, members []uint, exercises []model.GoalExercise) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var family model.Family
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&family, goal.FamilyID).Error; err != nil {
			return err
		}
		var current model.FamilyMember
		if err := tx.Where("family_id=? AND user_id=?", goal.FamilyID, goal.CreatedBy).First(&current).Error; err != nil {
			return errors.New("目标创建者已不属于家庭")
		}
		var creator model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&creator, goal.CreatedBy).Error; err != nil {
			return err
		}
		if goal.RequestKey != nil {
			var old model.DailyGoal
			err := tx.Unscoped().Where("created_by=? AND request_key=?", goal.CreatedBy, *goal.RequestKey).First(&old).Error
			if err == nil {
				if old.DeletedAt.Valid || old.FamilyID != goal.FamilyID {
					return errors.New("原创建结果已失效")
				}
				*goal = old
				return nil
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		for _, id := range members {
			var n int64
			if err := tx.Model(&model.FamilyMember{}).Where("family_id=? AND user_id=?", goal.FamilyID, id).Count(&n).Error; err != nil {
				return err
			}
			if n == 0 {
				return errors.New("目标成员必须属于当前家庭")
			}
		}
		if err := tx.Create(goal).Error; err != nil {
			return err
		}
		for _, id := range members {
			if err := tx.Create(&model.GoalMember{GoalID: goal.ID, UserID: id}).Error; err != nil {
				return err
			}
		}
		for _, ex := range exercises {
			ex.GoalID = goal.ID
			if err := tx.Create(&ex).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
