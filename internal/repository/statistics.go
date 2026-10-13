package repository

import (
	"elderly-fitness/internal/model"
	"gorm.io/gorm"
)

// Read logical daily progress, including archived family/goal/member history.
// Legacy duplicates are read once (latest row), never deleted or silently rewritten.
func (r *UserRepository) completionQuery() *gorm.DB {
	return r.db.Table("goal_completions gc").
		Joins("JOIN (SELECT MAX(id) id FROM goal_completions WHERE deleted_at IS NULL GROUP BY goal_member_id,goal_exercise_id,date) latest ON latest.id=gc.id").
		Joins("JOIN goal_members gm ON gm.id=gc.goal_member_id").
		Joins("JOIN daily_goals g ON g.id=gm.goal_id").
		Joins("JOIN goal_exercises ge ON ge.id=gc.goal_exercise_id AND ge.goal_id=g.id").
		Joins("JOIN exercises e ON e.id=ge.exercise_id").
		Where("ge.sets>0 AND gc.completed_sets>=0 AND gc.completed_sets<=ge.sets")
}

func (r *UserRepository) totalStatistics(userID uint) (*UserStatistics, error) {
	var sums struct {
		Days      int
		Sets      int
		Seconds   int
		Exercises int
	}
	err := r.completionQuery().Where("gm.user_id=?", userID).Select(`
 COUNT(DISTINCT CASE WHEN gc.completed_sets>0 THEN gc.date END) days,
 COALESCE(SUM(gc.completed_sets),0) sets,
 COALESCE(SUM(gc.completed_sets*30),0) seconds,
 COALESCE(SUM(CASE WHEN gc.completed_sets=ge.sets THEN 1 ELSE 0 END),0) exercises`).Scan(&sums).Error
	if err != nil {
		return nil, err
	}
	return &UserStatistics{TotalDays: sums.Days, TotalSets: sums.Sets, TotalMinutes: sums.Seconds / 60, TotalExercises: sums.Exercises}, nil
}

type FamilyStatistics struct {
	FamilyID           uint   `json:"family_id"`
	FamilyName         string `json:"family_name"`
	TotalDays          int    `json:"total_days"`
	TotalSets          int    `json:"total_sets"`
	CompletedExercises int    `json:"completed_exercises"`
	IsCurrentMember    bool   `json:"is_current_member"`
}

func (r *UserRepository) GetFamilyStatistics(userID uint) ([]FamilyStatistics, error) {
	var result []FamilyStatistics
	err := r.completionQuery().Joins("JOIN families f ON f.id=g.family_id").Where("gm.user_id=? AND gc.completed_sets>0", userID).
		Select(`f.id family_id,f.name family_name,COUNT(DISTINCT gc.date) total_days,SUM(gc.completed_sets) total_sets,
  SUM(CASE WHEN gc.completed_sets=ge.sets THEN 1 ELSE 0 END) completed_exercises,
  (f.deleted_at IS NULL AND EXISTS (SELECT 1 FROM family_members fm WHERE fm.family_id=f.id AND fm.user_id=? AND fm.deleted_at IS NULL)) is_current_member`, userID).
		Group("f.id,f.name,f.deleted_at").Order("f.id").Scan(&result).Error
	return result, err
}
func (r *UserRepository) progressRows(userID uint, limit int) ([]GoalCompletionWithGoal, error) {
	var rows []GoalCompletionWithGoal
	q := r.completionQuery().Where("gm.user_id=? AND gc.completed_sets>0", userID).
		Joins("JOIN (SELECT goal_id,COUNT(*) total_exercises,SUM(sets) total_sets FROM goal_exercises GROUP BY goal_id) ge_sum ON ge_sum.goal_id=g.id").
		Select("gc.date,g.id goal_id,g.name goal_name,e.id exercise_id,e.name exercise_name,gc.completed_sets,ge.sets target_sets,ge_sum.total_exercises,ge_sum.total_sets goal_total_sets").Order("gc.date DESC,g.id DESC,ge.id")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Scan(&rows).Error
	return rows, err
}
func (r *UserRepository) completedToday(date string, userID *uint) (int, error) {
	var n int64
	q := r.completionQuery().Where("gc.date=? AND gc.completed_sets=ge.sets", date)
	if userID != nil {
		q = q.Where("gm.user_id=?", *userID)
	}
	err := q.Count(&n).Error
	return int(n), err
}

// Replace derived daily statistics from canonical progress, not HTTP attempts.
func RefreshDailyStatistics(tx *gorm.DB, userID uint, date string, day model.UserDailyStats) error {
	var sums struct {
		Count   int
		Seconds int
	}
	err := NewUserRepository(tx).completionQuery().Where("gm.user_id=? AND gc.date=?", userID, date).
		Select("COALESCE(SUM(CASE WHEN gc.completed_sets=ge.sets THEN 1 ELSE 0 END),0) count,COALESCE(SUM(gc.completed_sets*30),0) seconds").Scan(&sums).Error
	if err != nil {
		return err
	}
	day.ExerciseCount = sums.Count
	day.TotalDuration = sums.Seconds
	var existing model.UserDailyStats
	err = tx.Where("user_id=? AND date=?", userID, date).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return tx.Create(&day).Error
	}
	if err != nil {
		return err
	}
	return tx.Model(&existing).Updates(map[string]interface{}{"exercise_count": sums.Count, "total_duration": sums.Seconds}).Error
}
