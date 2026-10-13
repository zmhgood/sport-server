package service

import (
	"elderly-fitness/internal/model"
	"elderly-fitness/internal/repository"
	"elderly-fitness/pkg/clock"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Progress and its derived ledger are one transaction. Lock family eligibility,
// then the user across families, so retries and first-day aggregates serialize.
func (s *GoalService) completeProgress(goalID, userID uint, input CompleteExerciseInput) error {
	var goal model.DailyGoal
	if err := s.goalRepo.DB().First(&goal, goalID).Error; err != nil || !goal.IsActive {
		return errors.New("目标不存在或已停用")
	}
	return s.goalRepo.DB().Transaction(func(tx *gorm.DB) error {
		var family model.Family
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&family, goal.FamilyID).Error; err != nil {
			return errors.New("家庭不存在")
		}
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, userID).Error; err != nil {
			return errors.New("用户不存在")
		}
		var member model.FamilyMember
		if err := tx.Where("family_id=? AND user_id=?", family.ID, userID).First(&member).Error; err != nil {
			return errors.New("您已不属于该家庭")
		}
		if err := tx.First(&goal, goalID).Error; err != nil || !goal.IsActive {
			return errors.New("目标不存在或已停用")
		}
		var gm model.GoalMember
		if err := tx.Where("goal_id=? AND user_id=?", goalID, userID).First(&gm).Error; err != nil {
			return errors.New("您不是该目标成员，请先加入目标")
		}
		var ex model.GoalExercise
		if err := tx.Where("id=? AND goal_id=?", input.GoalExerciseID, goalID).First(&ex).Error; err != nil {
			return errors.New("动作不属于该目标")
		}
		if input.CompletedSets < 0 || input.CompletedSets > ex.Sets || ex.Sets <= 0 {
			return errors.New("完成组数超出目标范围")
		}
		now := clock.Now()
		date := now.Format("2006-01-02")
		var completion model.GoalCompletion
		err := tx.Where("goal_member_id=? AND goal_exercise_id=? AND date=?", gm.ID, ex.ID, date).Order("id DESC").Take(&completion).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("进度读取失败，请重试")
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			completion = model.GoalCompletion{GoalMemberID: gm.ID, GoalExerciseID: ex.ID, Date: now}
			if input.CompletedSets == 0 {
				return nil
			}
			if err := tx.Create(&completion).Error; err != nil {
				return errors.New("进度保存失败，请重试")
			}
		}
		status := "pending"
		if input.CompletedSets == ex.Sets {
			status = "done"
		}
		if err := tx.Model(&completion).Updates(map[string]interface{}{"completed_sets": input.CompletedSets, "status": status}).Error; err != nil {
			return errors.New("进度保存失败，请重试")
		}
		// Legacy ledgers have NULL completion_id and remain intact. New writes are
		// explicitly linked to one logical completion and replace its absolute value.
		record := model.UserExerciseRecord{CompletionID: &completion.ID, UserID: userID, ExerciseID: ex.ExerciseID, Sets: input.CompletedSets, Duration: input.CompletedSets * 30, CompletedAt: completion.Date}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "completion_id"}}, DoUpdates: clause.AssignmentColumns([]string{"sets", "duration"})}).Create(&record).Error; err != nil {
			return errors.New("锻炼记录保存失败，请重试")
		}
		return repository.RefreshDailyStatistics(tx, userID, date, model.UserDailyStats{UserID: userID, Date: now})
	})
}
