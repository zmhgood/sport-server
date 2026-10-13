package repository

import (
	"elderly-fitness/internal/model"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *GoalRepository) Mutate(goalID, userID uint, fn func(*gorm.DB, *model.DailyGoal) error) error {
	var goal model.DailyGoal
	if err := r.db.First(&goal, goalID).Error; err != nil {
		return errors.New("目标不存在")
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		var family model.Family
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&family, goal.FamilyID).Error; err != nil {
			return errors.New("家庭不存在")
		}
		var member model.FamilyMember
		if err := tx.Where("family_id=? AND user_id=?", family.ID, userID).First(&member).Error; err != nil {
			return errors.New("无权限操作")
		}
		if err := tx.First(&goal, goalID).Error; err != nil {
			return errors.New("目标不存在")
		}
		return fn(tx, &goal)
	})
}
