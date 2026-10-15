package service

import (
	"elderly-fitness/pkg/clock"
	"errors"
	"gorm.io/gorm"
	"strings"
	"time"
	"unicode/utf8"

	"elderly-fitness/internal/model"
	"elderly-fitness/internal/repository"
)

type GoalService struct {
	goalRepo     *repository.GoalRepository
	familyRepo   *repository.FamilyRepository
	exerciseRepo *repository.ExerciseRepository
}

func NewGoalService(goalRepo *repository.GoalRepository, familyRepo *repository.FamilyRepository, exerciseRepo *repository.ExerciseRepository) *GoalService {
	return &GoalService{
		goalRepo:     goalRepo,
		familyRepo:   familyRepo,
		exerciseRepo: exerciseRepo,
	}
}

// CreateGoalInput 创建目标输入
type CreateGoalInput struct {
	PlanType   string              `json:"plan_type"`
	ExecutorID uint                `json:"executor_id"`
	RequestID  string              `json:"request_id"`
	Name       string              `json:"name"`
	MemberIDs  []uint              `json:"member_ids"`
	Exercises  []GoalExerciseInput `json:"exercises"`
}

// GoalExerciseInput 目标动作输入
type GoalExerciseInput struct {
	ExerciseID uint `json:"exercise_id"`
	Sets       int  `json:"sets"`
	Reps       int  `json:"reps"`
}

// CreateGoal 创建目标
func (s *GoalService) CreateGoal(familyID, creatorID uint, input CreateGoalInput) (*model.DailyGoal, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || utf8.RuneCountInString(input.Name) > 64 || len(input.Exercises) == 0 {
		return nil, errors.New("请填写目标名称和动作")
	}
	member, err := s.familyRepo.IsMember(familyID, creatorID)
	if err != nil || !member {
		return nil, errors.New("无权限操作")
	}
	planType := input.PlanType
	if planType == "" {
		planType = "shared"
	}
	if planType != "shared" && planType != "personal" {
		return nil, errors.New("计划类型无效")
	}
	ids := make([]uint, 0)
	if planType == "personal" {
		if input.ExecutorID == 0 {
			return nil, errors.New("请选择执行者")
		}
		ids = append(ids, input.ExecutorID)
	} else {
		if input.ExecutorID != 0 {
			return nil, errors.New("共同计划不指定执行者")
		}
		if input.MemberIDs == nil {
			ids = append(ids, creatorID)
		}
	}
	seen := map[uint]bool{}
	for _, id := range input.MemberIDs {
		if planType == "personal" {
			return nil, errors.New("个人计划只指定一位执行者")
		}
		if id != creatorID {
			return nil, errors.New("共同计划需由家人自愿加入")
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	exercises, err := s.validatePlanExercises(input.Exercises)
	if err != nil {
		return nil, err
	}
	goal := &model.DailyGoal{FamilyID: familyID, Name: input.Name, CreatedBy: creatorID, IsActive: true, PlanType: planType, ExecutorID: input.ExecutorID}
	if input.RequestID != "" {
		if len(input.RequestID) > 64 {
			return nil, errors.New("创建请求标识无效")
		}
		goal.RequestKey = &input.RequestID
	}

	if err := s.goalRepo.CreateWithRelations(goal, ids, exercises); err != nil {
		return nil, errors.New("创建目标失败，请检查成员或重试")
	}
	return s.GetGoalForUser(goal.ID, creatorID)
}

// GetGoal 获取目标详情
func (s *GoalService) GetGoal(goalID uint) (*model.DailyGoal, error) {
	return s.goalRepo.FindGoalByID(goalID)
}

// GetFamilyGoals 获取家庭目标列表
func (s *GoalService) GetFamilyGoals(familyID uint) ([]model.DailyGoal, error) {
	return s.goalRepo.GetGoalsByFamilyID(familyID)
}

// GetUserGoals 获取用户参与的目标列表
func (s *GoalService) GetUserGoals(userID uint) ([]model.DailyGoal, error) {
	return s.goalRepo.GetGoalsByUserID(userID)
}

// GetUserGoalsWithProgress 获取用户参与的目标列表（包含今日进度）
func (s *GoalService) GetUserGoalsWithProgress(userID uint) ([]GoalWithProgress, error) {
	goals, err := s.goalRepo.GetGoalsByUserID(userID)
	if err != nil {
		return nil, err
	}

	date := clock.Now().Format("2006-01-02")
	result := make([]GoalWithProgress, 0)

	for _, goal := range goals {
		// 直接使用已预加载的数据计算进度
		var todayCompleted int
		var totalExercises int

		// 找到当前用户在目标中的成员记录
		for _, member := range goal.Members {
			if member.UserID == userID {
				// 获取该成员今天的完成记录
				completions, err := s.goalRepo.GetMemberCompletions(goal.ID, userID, date)
				if err != nil {
					return nil, err
				}
				completionMap := make(map[uint]model.GoalCompletion)
				for _, c := range completions {
					completionMap[c.GoalExerciseID] = c
				}

				// 计算已完成的动作数
				totalExercises = len(goal.Exercises)
				for _, ex := range goal.Exercises {
					if c, ok := completionMap[ex.ID]; ok {
						if c.CompletedSets >= ex.Sets {
							todayCompleted++
						}
					}
				}
				break
			}
		}

		result = append(result, GoalWithProgress{
			DailyGoal:      goal,
			TodayCompleted: todayCompleted,
			TotalExercises: totalExercises,
			MemberCount:    len(goal.Members),
			ExerciseCount:  len(goal.Exercises),
		})
	}

	return result, nil
}

// GoalWithProgress 目标带进度信息
type GoalWithProgress struct {
	model.DailyGoal
	TodayCompleted int `json:"today_completed"`
	TotalExercises int `json:"total_exercises"`
	MemberCount    int `json:"member_count"`
	ExerciseCount  int `json:"exercise_count"`
}

// FamilyWithGoals 家庭及其目标信息
type FamilyWithGoals struct {
	FamilyID   uint               `json:"family_id"`
	FamilyName string             `json:"family_name"`
	Goals      []GoalWithProgress `json:"goals"`
}

// GetUserFamiliesWithGoals 获取用户所有家庭及其目标
func (s *GoalService) GetUserFamiliesWithGoals(userID uint) ([]FamilyWithGoals, error) {
	// 获取用户所有家庭
	families, err := s.familyRepo.FindByUserID(userID)
	if err != nil {
		return nil, err
	}

	date := clock.Now().Format("2006-01-02")
	result := make([]FamilyWithGoals, 0)

	for _, family := range families {
		// 获取该家庭的所有目标
		goals, err := s.goalRepo.GetGoalsByFamilyID(family.ID)
		if err != nil {
			return nil, err
		}

		familyGoals := make([]GoalWithProgress, 0)
		for _, goal := range goals {
			// 只返回活跃的目标
			if !goal.IsActive {
				continue
			}

			// 计算今日进度
			var todayCompleted int
			totalExercises := len(goal.Exercises)

			// 检查当前用户是否是目标成员
			isMember := false
			for _, member := range goal.Members {
				if member.UserID == userID {
					isMember = true
					break
				}
			}

			// 如果是目标成员，计算今日完成进度
			if !isMember {
				continue
			}
			if isMember {
				completions, err := s.goalRepo.GetMemberCompletions(goal.ID, userID, date)
				if err != nil {
					return nil, err
				}
				completionMap := make(map[uint]model.GoalCompletion)
				for _, c := range completions {
					completionMap[c.GoalExerciseID] = c
				}

				for _, ex := range goal.Exercises {
					if c, ok := completionMap[ex.ID]; ok {
						if c.CompletedSets >= ex.Sets {
							todayCompleted++
						}
					}
				}
			}

			familyGoals = append(familyGoals, GoalWithProgress{
				DailyGoal:      goal,
				TodayCompleted: todayCompleted,
				TotalExercises: totalExercises,
				MemberCount:    len(goal.Members),
				ExerciseCount:  len(goal.Exercises),
			})
		}

		result = append(result, FamilyWithGoals{
			FamilyID:   family.ID,
			FamilyName: family.Name,
			Goals:      familyGoals,
		})
	}

	return result, nil
}

// UpdateGoalInput 更新目标输入
type UpdateGoalInput struct {
	Name      string              `json:"name"`
	IsActive  *bool               `json:"is_active"`
	MemberIDs []uint              `json:"member_ids"`
	Exercises []GoalExerciseInput `json:"exercises"`
}

// UpdateGoal 更新目标
func (s *GoalService) UpdateGoal(goalID, userID uint, input UpdateGoalInput) (*model.DailyGoal, error) {
	return s.updatePlan(goalID, userID, input)
}
func (s *GoalService) DeleteGoal(goalID, userID uint) error {
	return s.goalRepo.Mutate(goalID, userID, func(tx *gorm.DB, goal *model.DailyGoal) error {
		if !canManagePlan(tx, goal, userID) {
			return errors.New("无权限修改计划")
		}
		return tx.Delete(goal).Error
	})
}
func (s *GoalService) AddGoalMember(goalID, userID, newMemberID uint) error {
	return s.goalRepo.Mutate(goalID, userID, func(tx *gorm.DB, goal *model.DailyGoal) error {
		if goal.PlanType == "personal" || userID != newMemberID {
			return errors.New("仅可自行加入共同计划")
		}
		if !goal.IsActive {
			return errors.New("目标已停用")
		}
		var fm model.FamilyMember
		if err := tx.Where("family_id=? AND user_id=?", goal.FamilyID, newMemberID).First(&fm).Error; err != nil {
			return errors.New("该用户不是家庭成员")
		}
		var old model.GoalMember
		err := tx.Unscoped().Where("goal_id=? AND user_id=?", goalID, newMemberID).First(&old).Error
		if err == nil {
			if !old.DeletedAt.Valid {
				return nil
			}
			return tx.Unscoped().Model(&old).Update("deleted_at", nil).Error
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return tx.Create(&model.GoalMember{GoalID: goalID, UserID: newMemberID}).Error
	})
}
func (s *GoalService) AddGoalExercise(goalID, userID, exerciseID uint, sets, reps int) error {
	if sets < 1 || sets > 100 || reps < 1 || reps > 1000 {
		return errors.New("动作组数或次数无效")
	}
	return s.goalRepo.Mutate(goalID, userID, func(tx *gorm.DB, goal *model.DailyGoal) error {
		if !canManagePlan(tx, goal, userID) {
			return errors.New("无权限修改计划")
		}
		r := repository.NewGoalRepository(tx)
		if err := r.LoadPlanDate(goal, clock.Now().Format("2006-01-02")); err != nil {
			return err
		}
		if err := r.LoadPendingPlan(goal); err != nil {
			return err
		}
		existing := goal.Exercises
		if goal.PendingEffectiveDate != "" {
			existing = goal.PendingExercises
		}
		inputs := []GoalExerciseInput{{ExerciseID: exerciseID, Sets: sets, Reps: reps}}
		for _, ex := range existing {
			inputs = append(inputs, GoalExerciseInput{ExerciseID: ex.ExerciseID, Sets: ex.Sets, Reps: ex.Reps})
		}
		exercises, err := s.validatePlanExercises(inputs)
		if err != nil {
			return err
		}
		return savePlanRevision(tx, goalID, exercises)
	})
}

// CompleteExerciseInput 完成动作输入
type CompleteExerciseInput struct {
	Date           string `json:"date"`
	GoalExerciseID uint   `json:"goal_exercise_id"`
	CompletedSets  int    `json:"completed_sets"`
}

// CompleteExercise 完成动作
func (s *GoalService) CompleteExercise(goalID, userID uint, input CompleteExerciseInput) error {
	return s.completeProgress(goalID, userID, input)
}

// GoalProgress 目标进度
type GoalProgress struct {
	GoalID      uint                 `json:"goal_id"`
	GoalName    string               `json:"goal_name"`
	Date        string               `json:"date"`
	Exercises   []model.GoalExercise `json:"exercises"`
	MemberStats []MemberProgress     `json:"member_stats"`
}

// MemberProgress 成员进度
type MemberProgress struct {
	IsCurrentMember bool               `json:"is_current_member"`
	UserID          uint               `json:"user_id"`
	UserName        string             `json:"user_name"`
	UserAvatar      string             `json:"user_avatar"`
	ExerciseStats   []ExerciseProgress `json:"exercise_stats"`
	CompletedCount  int                `json:"completed_count"` // 已完成的动作数
	TotalCount      int                `json:"total_count"`     // 总动作数
	TotalSets       int                `json:"total_sets"`      // 总组数
	CompletedSets   int                `json:"completed_sets"`  // 已完成组数
	Progress        int                `json:"progress"`        // 百分比（基于组数计算）
}

// ExerciseProgress 动作进度
type ExerciseProgress struct {
	GoalExerciseID uint   `json:"goal_exercise_id"`
	ExerciseID     uint   `json:"exercise_id"`
	ExerciseName   string `json:"exercise_name"`
	TargetSets     int    `json:"target_sets"`
	CompletedSets  int    `json:"completed_sets"`
	Reps           int    `json:"reps"`
	Status         string `json:"status"`
}

// GetGoalProgress 获取目标今日进度
func (s *GoalService) GetGoalProgress(goalID uint, date string) (*GoalProgress, error) {
	goal, err := s.goalRepo.FindGoalByID(goalID)
	if err != nil {
		return nil, err
	}

	if date == "" {
		date = clock.Now().Format("2006-01-02")
	}

	if _, err := time.Parse("2006-01-02", date); err != nil {
		return nil, errors.New("日期格式无效")
	}
	if err := s.goalRepo.LoadPlanDate(goal, date); err != nil {
		return nil, err
	}
	if err := s.goalRepo.DB().Unscoped().Where("goal_id=?", goalID).Preload("User").Find(&goal.Members).Error; err != nil {
		return nil, err
	}
	progress := &GoalProgress{
		GoalID:      goal.ID,
		GoalName:    goal.Name,
		Date:        date,
		Exercises:   goal.Exercises,
		MemberStats: make([]MemberProgress, 0),
	}

	// 获取每个成员的进度
	for _, member := range goal.Members {
		current, err := s.familyRepo.IsMember(goal.FamilyID, member.UserID)
		if err != nil {
			return nil, err
		}
		memberProgress := MemberProgress{
			IsCurrentMember: current,
			UserID:          member.UserID,
			UserName:        member.User.NickName,
			UserAvatar:      member.User.AvatarURL,
		}

		// 获取该成员今天的完成记录
		completions, err := s.goalRepo.GetMemberCompletions(goalID, member.UserID, date)
		if err != nil {
			return nil, err
		}
		if member.DeletedAt.Valid && len(completions) == 0 {
			continue
		}
		memberProgress.IsCurrentMember = current && !member.DeletedAt.Valid
		completionMap := make(map[uint]model.GoalCompletion)
		for _, c := range completions {
			completionMap[c.GoalExerciseID] = c
		}

		// 计算每个动作的进度
		for _, ex := range goal.Exercises {
			exProgress := ExerciseProgress{
				GoalExerciseID: ex.ID,
				ExerciseID:     ex.ExerciseID,
				ExerciseName:   ex.Exercise.Name,
				TargetSets:     ex.Sets,
				Reps:           ex.Reps,
			}

			if c, ok := completionMap[ex.ID]; ok {
				exProgress.CompletedSets = c.CompletedSets
				exProgress.Status = c.Status
			} else {
				exProgress.Status = "pending"
			}

			if exProgress.TargetSets > 0 && exProgress.CompletedSets >= exProgress.TargetSets {
				memberProgress.CompletedCount++
			}
			memberProgress.TotalCount++

			// 累加组数
			memberProgress.TotalSets += ex.Sets
			memberProgress.CompletedSets += exProgress.CompletedSets

			memberProgress.ExerciseStats = append(memberProgress.ExerciseStats, exProgress)
		}

		// 计算总进度（基于组数）
		if memberProgress.TotalSets > 0 {
			memberProgress.Progress = memberProgress.CompletedSets * 100 / memberProgress.TotalSets
		}

		progress.MemberStats = append(progress.MemberStats, memberProgress)
	}

	return progress, nil
}

// GetGoalHistory 获取目标历史记录
func (s *GoalService) GetGoalHistory(goalID uint, startDate, endDate string) ([]model.GoalCompletion, error) {
	if startDate == "" {
		startDate = clock.Now().AddDate(0, 0, -7).Format("2006-01-02")
	}
	if endDate == "" {
		endDate = clock.Now().Format("2006-01-02")
	}
	return s.goalRepo.GetHistoryCompletions(goalID, startDate, endDate)
}
