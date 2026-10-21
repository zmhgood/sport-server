package repository

import (
	"database/sql"
	"elderly-fitness/internal/model"
	"gorm.io/gorm"
)

type DashboardActivity struct {
	Date           string `json:"date"`
	UserCount      int64  `json:"userCount"`
	CompletedCount int64  `json:"-"`
}

type DashboardHotExercise struct {
	ExerciseID uint   `json:"exerciseId"`
	Name       string `json:"name"`
	Count      int64  `json:"count"`
}

type DashboardMetrics struct {
	UserCount     int64
	ExerciseCount int64
	CommentCount  int64
	Activity      []DashboardActivity
	HotExercises  []DashboardHotExercise
}

// Historical membership and archived plans/actions retain their contribution.
// Use the latest progress per person/date/plan/catalog action, not mirrored records
// or one count per HTTP submission. Validate progress after choosing the latest row.
func (r *UserRepository) dashboardProgress(start, end string) *gorm.DB {
	latest := r.db.Table("goal_completions c").
		Joins("JOIN goal_members m ON m.id=c.goal_member_id").
		Joins("JOIN daily_goals g ON g.id=m.goal_id").
		Joins("JOIN goal_exercises ge ON ge.id=c.goal_exercise_id AND ge.goal_id=g.id").
		Joins("JOIN exercises e ON e.id=ge.exercise_id").
		Where("c.deleted_at IS NULL AND c.date>=? AND c.date<=?", start, end).
		Select("MAX(c.id) id").Group("m.user_id,g.id,ge.exercise_id,c.date")
	return r.db.Table("goal_completions gc").
		Joins("JOIN (?) latest ON latest.id=gc.id", latest).
		Joins("JOIN goal_members gm ON gm.id=gc.goal_member_id").
		Joins("JOIN goal_exercises ge ON ge.id=gc.goal_exercise_id").
		Joins("JOIN exercises e ON e.id=ge.exercise_id").
		Where("ge.sets>0 AND gc.completed_sets>=0 AND gc.completed_sets<=ge.sets")
}

func (r *UserRepository) GetDashboardMetrics(start, end string) (*DashboardMetrics, error) {
	result := &DashboardMetrics{Activity: []DashboardActivity{}, HotExercises: []DashboardHotExercise{}}
	// A consistent read snapshot keeps the trend and ranking in the same window
	// even when another user submits progress between the two aggregate queries.
	err := r.db.Transaction(func(tx *gorm.DB) error {
		for _, count := range []struct {
			model interface{}
			dest  *int64
		}{
			{&model.User{}, &result.UserCount}, {&model.Exercise{}, &result.ExerciseCount}, {&model.Comment{}, &result.CommentCount},
		} {
			if err := tx.Model(count.model).Count(count.dest).Error; err != nil {
				return err
			}
		}
		repo := NewUserRepository(tx)
		if err := repo.dashboardProgress(start, end).Where("gc.completed_sets>0").
			Select("DATE_FORMAT(gc.date,'%Y-%m-%d') date,COUNT(DISTINCT gm.user_id) user_count,SUM(CASE WHEN gc.completed_sets=ge.sets THEN 1 ELSE 0 END) completed_count").
			Group("gc.date").Order("gc.date").Scan(&result.Activity).Error; err != nil {
			return err
		}
		return repo.dashboardProgress(start, end).Where("gc.completed_sets=ge.sets").
			Select("e.id exercise_id,e.name,COUNT(*) count").Group("e.id,e.name").
			Order("count DESC,e.id ASC").Limit(10).Scan(&result.HotExercises).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return result, err
}
