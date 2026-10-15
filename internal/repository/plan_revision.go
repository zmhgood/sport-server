package repository

import (
	"elderly-fitness/internal/model"
	"elderly-fitness/pkg/clock"
	"errors"
	"gorm.io/gorm"
)

func RevisionForDate(db *gorm.DB, goalID uint, date string) (uint, error) {
	var revision model.PlanRevision
	err := db.Where("goal_id=? AND effective_date<=?", goalID, date).Order("effective_date DESC").First(&revision).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	return revision.ID, err
}

func (r *GoalRepository) LoadPlanDate(goal *model.DailyGoal, date string) error {
	revisionID, err := RevisionForDate(r.db, goal.ID, date)
	if err != nil {
		return err
	}
	goal.RevisionID = revisionID
	goal.CurrentDate = date
	goal.Exercises = make([]model.GoalExercise, 0)
	return r.db.Where("goal_id=? AND revision_id=?", goal.ID, revisionID).Order("id").Preload("Exercise").Find(&goal.Exercises).Error
}

func (r *GoalRepository) LoadPlansToday(goals []model.DailyGoal) error {
	date := clock.Now().Format("2006-01-02")
	for i := range goals {
		if err := r.LoadPlanDate(&goals[i], date); err != nil {
			return err
		}
	}
	return nil
}

func (r *GoalRepository) LoadPendingPlan(goal *model.DailyGoal) error {
	var revision model.PlanRevision
	err := r.db.Where("goal_id=? AND effective_date>?", goal.ID, goal.CurrentDate).Order("effective_date").First(&revision).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	goal.PendingEffectiveDate = revision.EffectiveDate.Format("2006-01-02")
	return r.db.Where("goal_id=? AND revision_id=?", goal.ID, revision.ID).Order("id").Preload("Exercise").Find(&goal.PendingExercises).Error
}
