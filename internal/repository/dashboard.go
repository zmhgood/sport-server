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
	Engagement    DashboardEngagementMetrics
}

type DashboardEngagementMetrics struct {
	ActivationEligibleUsers  int64
	ActivationCompletedUsers int64
	ActiveUserDays           []DashboardUserDay
	FamilyActiveCount        int64
	FamilyDaily              []DashboardFamilyDaily
}

type DashboardUserDay struct {
	UserID uint   `gorm:"column:user_id"`
	Date   string `gorm:"column:date"`
}

type DashboardFamilyDaily struct {
	Date        string `gorm:"column:date"`
	FamilyCount int64  `gorm:"column:family_count"`
}

type DashboardEngagementRange struct {
	ActivityStartDate     string
	ActivityEndDate       string
	RegistrationStartDate string
	RegistrationAfterDate string
	ActivationProgressEnd string
	FamilyStartDate       string
	FamilyEndDate         string
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

func (r *UserRepository) GetDashboardMetrics(start, end string, engagementRange DashboardEngagementRange) (*DashboardMetrics, error) {
	result := &DashboardMetrics{Activity: []DashboardActivity{}, HotExercises: []DashboardHotExercise{}}
	// Keep all dashboard and engagement metrics on the same read-only snapshot.
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
		if err := repo.dashboardProgress(engagementRange.ActivityStartDate, engagementRange.ActivityEndDate).
			Where("gc.completed_sets>0").
			Select("gm.user_id user_id,DATE_FORMAT(gc.date,'%Y-%m-%d') date").
			Group("gm.user_id,gc.date").Order("gc.date,gm.user_id").Scan(&result.Engagement.ActiveUserDays).Error; err != nil {
			return err
		}
		activationProgress := repo.dashboardProgress(engagementRange.RegistrationStartDate, engagementRange.ActivationProgressEnd).
			Where("gc.completed_sets=ge.sets").
			Select("gm.user_id user_id, gc.date date").Group("gm.user_id,gc.date")
		var activation struct {
			EligibleUsers  int64 `gorm:"column:eligible_users"`
			CompletedUsers int64 `gorm:"column:completed_users"`
		}
		if err := tx.Table("users u").
			Joins("LEFT JOIN (?) p ON p.user_id=u.id AND p.date>=DATE(u.created_at) AND p.date<=DATE_ADD(DATE(u.created_at),INTERVAL 6 DAY)", activationProgress).
			Where("u.created_at>=? AND u.created_at<?", engagementRange.RegistrationStartDate, engagementRange.RegistrationAfterDate).
			Select("COUNT(DISTINCT u.id) eligible_users,COUNT(DISTINCT CASE WHEN p.user_id IS NOT NULL THEN u.id END) completed_users").
			Scan(&activation).Error; err != nil {
			return err
		}
		result.Engagement.ActivationEligibleUsers = activation.EligibleUsers
		result.Engagement.ActivationCompletedUsers = activation.CompletedUsers
		var familyCounts []DashboardFamilyDaily
		if err := repo.dashboardProgress(engagementRange.FamilyStartDate, engagementRange.FamilyEndDate).
			Joins("JOIN daily_goals g ON g.id=gm.goal_id AND g.id=ge.goal_id").
			Joins("JOIN families f ON f.id=g.family_id").
			Select("DATE_FORMAT(gc.date,'%Y-%m-%d') date,COUNT(DISTINCT g.family_id) family_count").
			Where("gc.completed_sets>0").Group("gc.date").Order("gc.date").Scan(&familyCounts).Error; err != nil {
			return err
		}
		result.Engagement.FamilyDaily = familyCounts
		var familyCount struct {
			Count int64 `gorm:"column:family_count"`
		}
		if err := repo.dashboardProgress(engagementRange.FamilyStartDate, engagementRange.FamilyEndDate).
			Joins("JOIN daily_goals g ON g.id=gm.goal_id AND g.id=ge.goal_id").
			Joins("JOIN families f ON f.id=g.family_id").
			Where("gc.completed_sets>0").
			Select("COUNT(DISTINCT g.family_id) family_count").Scan(&familyCount).Error; err != nil {
			return err
		}
		result.Engagement.FamilyActiveCount = familyCount.Count
		return repo.dashboardProgress(start, end).Where("gc.completed_sets=ge.sets").
			Select("e.id exercise_id,e.name,COUNT(*) count").Group("e.id,e.name").
			Order("count DESC,e.id ASC").Limit(10).Scan(&result.HotExercises).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return result, err
}
