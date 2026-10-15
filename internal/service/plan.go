package service

import (
	"elderly-fitness/internal/model"
	"elderly-fitness/pkg/clock"
	"errors"
	"gorm.io/gorm"
	"strings"
	"unicode/utf8"
)

func canManagePlan(db *gorm.DB, goal *model.DailyGoal, userID uint) bool {
	var member model.FamilyMember
	if db.Where("family_id=? AND user_id=?", goal.FamilyID, userID).First(&member).Error != nil {
		return false
	}
	if member.Role == "admin" {
		return true
	}
	if goal.PlanType == "personal" {
		return goal.ExecutorID == userID
	}
	return goal.CreatedBy == userID
}

func (s *GoalService) GetGoalForUser(goalID, userID uint) (*model.DailyGoal, error) {
	goal, err := s.goalRepo.FindGoalByID(goalID)
	if err != nil {
		return nil, err
	}
	eligible, err := s.familyRepo.IsMember(goal.FamilyID, userID)
	if err != nil || !eligible {
		return nil, errors.New("无权访问该家庭")
	}
	goal.CanManage = canManagePlan(s.goalRepo.DB(), goal, userID)
	for _, member := range goal.Members {
		if member.UserID == userID {
			goal.IsParticipating = true
		}
	}
	if err := s.goalRepo.LoadPendingPlan(goal); err != nil {
		return nil, err
	}
	return goal, nil
}

func (s *GoalService) validatePlanExercises(input []GoalExerciseInput) ([]model.GoalExercise, error) {
	if len(input) == 0 || len(input) > 100 {
		return nil, errors.New("请选择1至100个动作")
	}
	result := make([]model.GoalExercise, 0, len(input))
	seen := map[uint]bool{}
	for _, ex := range input {
		if ex.Sets < 1 || ex.Sets > 100 || ex.Reps < 1 || ex.Reps > 1000 || seen[ex.ExerciseID] {
			return nil, errors.New("动作组数、次数或重复动作无效")
		}
		if _, err := s.exerciseRepo.GetExerciseByID(ex.ExerciseID); err != nil {
			return nil, errors.New("动作不存在")
		}
		seen[ex.ExerciseID] = true
		result = append(result, model.GoalExercise{ExerciseID: ex.ExerciseID, Sets: ex.Sets, Reps: ex.Reps})
	}
	return result, nil
}

func (s *GoalService) updatePlan(goalID, userID uint, input UpdateGoalInput) (*model.DailyGoal, error) {
	if input.MemberIDs != nil {
		return nil, errors.New("请使用参与入口管理成员")
	}
	name := strings.TrimSpace(input.Name)
	if input.Name != "" && (name == "" || utf8.RuneCountInString(name) > 64) {
		return nil, errors.New("计划名称无效")
	}
	var exercises []model.GoalExercise
	var err error
	if input.Exercises != nil {
		exercises, err = s.validatePlanExercises(input.Exercises)
		if err != nil {
			return nil, err
		}
	}
	err = s.goalRepo.Mutate(goalID, userID, func(tx *gorm.DB, goal *model.DailyGoal) error {
		if !canManagePlan(tx, goal, userID) {
			return errors.New("无权限修改计划")
		}
		changes := map[string]interface{}{}
		if name != "" {
			changes["name"] = name
		}
		if input.IsActive != nil {
			changes["is_active"] = *input.IsActive
		}
		if len(changes) > 0 {
			if err := tx.Model(goal).Updates(changes).Error; err != nil {
				return err
			}
		}
		if exercises == nil {
			return nil
		}
		return savePlanRevision(tx, goal.ID, exercises)
	})
	if err != nil {
		return nil, err
	}
	return s.GetGoalForUser(goalID, userID)
}

// Caller holds the family lock, shared by progress submissions and membership changes.
func savePlanRevision(tx *gorm.DB, goalID uint, exercises []model.GoalExercise) error {
	now := clock.Now()
	today := now.Format("2006-01-02")
	var started int64
	if err := tx.Model(&model.GoalCompletion{}).Joins("JOIN goal_members gm ON gm.id=goal_completions.goal_member_id").Where("gm.goal_id=? AND date=?", goalID, today).Count(&started).Error; err != nil {
		return err
	}
	effective := today
	if started > 0 {
		effective = now.AddDate(0, 0, 1).Format("2006-01-02")
	}
	var revision model.PlanRevision
	err := tx.Where("goal_id=? AND effective_date=?", goalID, effective).First(&revision).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		effectiveTime := now
		if started > 0 {
			effectiveTime = effectiveTime.AddDate(0, 0, 1)
		}
		revision = model.PlanRevision{GoalID: goalID, EffectiveDate: effectiveTime}
		if err := tx.Create(&revision).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		// This effective revision has no submitted records yet; retain superseded rows as archives.
		if err := tx.Where("goal_id=? AND revision_id=?", goalID, revision.ID).Delete(&model.GoalExercise{}).Error; err != nil {
			return err
		}
	}
	for i := range exercises {
		exercises[i].GoalID, exercises[i].RevisionID = goalID, revision.ID
		if err := tx.Create(&exercises[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *GoalService) LeaveGoal(goalID, userID uint) error {
	return s.goalRepo.Mutate(goalID, userID, func(tx *gorm.DB, goal *model.DailyGoal) error {
		if goal.PlanType == "personal" {
			return errors.New("个人计划请使用暂停功能")
		}
		return tx.Where("goal_id=? AND user_id=?", goalID, userID).Delete(&model.GoalMember{}).Error
	})
}
