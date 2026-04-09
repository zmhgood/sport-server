package service

import (
	"errors"
	"time"

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
	Name        string                  `json:"name"`
	MemberIDs   []uint                  `json:"member_ids"`
	Exercises   []GoalExerciseInput     `json:"exercises"`
}

// GoalExerciseInput 目标动作输入
type GoalExerciseInput struct {
	ExerciseID uint `json:"exercise_id"`
	Sets       int  `json:"sets"`
	Reps       int  `json:"reps"`
}

// CreateGoal 创建目标
func (s *GoalService) CreateGoal(familyID, creatorID uint, input CreateGoalInput) (*model.DailyGoal, error) {
	// 检查创建者是否是家庭成员
	isMember, _ := s.familyRepo.IsMember(familyID, creatorID)
	if !isMember {
		return nil, errors.New("无权限操作")
	}

	goal := &model.DailyGoal{
		FamilyID:  familyID,
		Name:      input.Name,
		CreatedBy: creatorID,
		IsActive:  true,
	}

	// 创建目标
	if err := s.goalRepo.CreateGoal(goal); err != nil {
		return nil, err
	}

	// 确保创建者在成员列表中
	memberIDs := input.MemberIDs
	creatorInList := false
	for _, id := range memberIDs {
		if id == creatorID {
			creatorInList = true
			break
		}
	}
	if !creatorInList {
		memberIDs = append(memberIDs, creatorID)
	}

	// 添加成员
	for _, memberID := range memberIDs {
		member := &model.GoalMember{
			GoalID: goal.ID,
			UserID: memberID,
		}
		if err := s.goalRepo.AddGoalMember(member); err != nil {
			return nil, err
		}
	}

	// 添加动作
	for _, ex := range input.Exercises {
		goalEx := &model.GoalExercise{
			GoalID:     goal.ID,
			ExerciseID: ex.ExerciseID,
			Sets:       ex.Sets,
			Reps:       ex.Reps,
		}
		if err := s.goalRepo.AddGoalExercise(goalEx); err != nil {
			return nil, err
		}
	}

	return s.goalRepo.FindGoalByID(goal.ID)
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

	date := time.Now().Format("2006-01-02")
	var result []GoalWithProgress

	for _, goal := range goals {
		// 直接使用已预加载的数据计算进度
		var todayCompleted int
		var totalExercises int
		
		// 找到当前用户在目标中的成员记录
		for _, member := range goal.Members {
			if member.UserID == userID {
				// 获取该成员今天的完成记录
				completions, _ := s.goalRepo.GetMemberCompletions(goal.ID, userID, date)
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
	FamilyID     uint              `json:"family_id"`
	FamilyName   string            `json:"family_name"`
	Goals        []GoalWithProgress `json:"goals"`
}

// GetUserFamiliesWithGoals 获取用户所有家庭及其目标
func (s *GoalService) GetUserFamiliesWithGoals(userID uint) ([]FamilyWithGoals, error) {
	// 获取用户所有家庭
	families, err := s.familyRepo.FindByUserID(userID)
	if err != nil {
		return nil, err
	}

	date := time.Now().Format("2006-01-02")
	var result []FamilyWithGoals

	for _, family := range families {
		// 获取该家庭的所有目标
		goals, err := s.goalRepo.GetGoalsByFamilyID(family.ID)
		if err != nil {
			continue
		}

		var familyGoals []GoalWithProgress
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
			if isMember {
				completions, _ := s.goalRepo.GetMemberCompletions(goal.ID, userID, date)
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
			FamilyID:     family.ID,
			FamilyName:   family.Name,
			Goals:        familyGoals,
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
	goal, err := s.goalRepo.FindGoalByID(goalID)
	if err != nil {
		return nil, err
	}

	// 检查权限
	isMember, _ := s.familyRepo.IsMember(goal.FamilyID, userID)
	if !isMember {
		return nil, errors.New("无权限操作")
	}

	if input.Name != "" {
		goal.Name = input.Name
	}
	if input.IsActive != nil {
		goal.IsActive = *input.IsActive
	}

	if err := s.goalRepo.UpdateGoal(goal); err != nil {
		return nil, err
	}

	return s.goalRepo.FindGoalByID(goalID)
}

// DeleteGoal 删除目标
func (s *GoalService) DeleteGoal(goalID, userID uint) error {
	goal, err := s.goalRepo.FindGoalByID(goalID)
	if err != nil {
		return err
	}

	// 检查权限
	isMember, _ := s.familyRepo.IsMember(goal.FamilyID, userID)
	if !isMember {
		return errors.New("无权限操作")
	}

	return s.goalRepo.DeleteGoal(goalID)
}

// AddGoalMember 添加目标成员
func (s *GoalService) AddGoalMember(goalID, userID, newMemberID uint) error {
	goal, err := s.goalRepo.FindGoalByID(goalID)
	if err != nil {
		return err
	}

	// 检查权限
	isMember, _ := s.familyRepo.IsMember(goal.FamilyID, userID)
	if !isMember {
		return errors.New("无权限操作")
	}

	// 检查新成员是否是家庭成员
	isFamilyMember, _ := s.familyRepo.IsMember(goal.FamilyID, newMemberID)
	if !isFamilyMember {
		return errors.New("该用户不是家庭成员")
	}

	member := &model.GoalMember{
		GoalID: goalID,
		UserID: newMemberID,
	}
	return s.goalRepo.AddGoalMember(member)
}

// AddGoalExercise 添加目标动作
func (s *GoalService) AddGoalExercise(goalID, userID uint, exerciseID uint, sets, reps int) error {
	goal, err := s.goalRepo.FindGoalByID(goalID)
	if err != nil {
		return err
	}

	// 检查权限
	isMember, _ := s.familyRepo.IsMember(goal.FamilyID, userID)
	if !isMember {
		return errors.New("无权限操作")
	}

	goalEx := &model.GoalExercise{
		GoalID:     goalID,
		ExerciseID: exerciseID,
		Sets:       sets,
		Reps:       reps,
	}
	return s.goalRepo.AddGoalExercise(goalEx)
}

// CompleteExerciseInput 完成动作输入
type CompleteExerciseInput struct {
	GoalExerciseID uint `json:"goal_exercise_id"`
	CompletedSets  int  `json:"completed_sets"`
}

// CompleteExercise 完成动作
func (s *GoalService) CompleteExercise(goalID, userID uint, input CompleteExerciseInput) error {
	// 获取目标成员
	goalMember, err := s.goalRepo.GetGoalMember(goalID, userID)
	if err != nil {
		return errors.New("您不是该目标成员，请先加入目标")
	}

	date := time.Now().Format("2006-01-02")

	// 获取或创建完成记录
	completion, err := s.goalRepo.GetOrCreateCompletion(goalMember.ID, input.GoalExerciseID, date)
	if err != nil {
		return errors.New("创建完成记录失败: " + err.Error())
	}

	// 更新完成状态
	goalEx, err := s.goalRepo.GetGoalExercise(input.GoalExerciseID)
	if err != nil {
		return errors.New("获取动作信息失败: " + err.Error())
	}

	completion.CompletedSets = input.CompletedSets
	if completion.CompletedSets >= goalEx.Sets {
		completion.Status = "done"
	} else {
		completion.Status = "pending"
	}

	if err := s.goalRepo.UpdateCompletion(completion); err != nil {
		return errors.New("更新完成记录失败: " + err.Error())
	}

	// 创建用户锻炼记录（用于统计）
	record := &model.UserExerciseRecord{
		UserID:      userID,
		ExerciseID:  goalEx.ExerciseID,
		Duration:    goalEx.Reps * input.CompletedSets * 30, // 估算时长：次数×组数×30秒
		Sets:        input.CompletedSets,
		CompletedAt: time.Now(),
	}
	if err := s.exerciseRepo.CreateExerciseRecord(record); err != nil {
		// 记录错误但不影响主流程
		// 可以添加日志记录
	}

	return nil
}

// GoalProgress 目标进度
type GoalProgress struct {
	GoalID      uint             `json:"goal_id"`
	GoalName    string           `json:"goal_name"`
	Date        string           `json:"date"`
	MemberStats []MemberProgress `json:"member_stats"`
}

// MemberProgress 成员进度
type MemberProgress struct {
	UserID         uint                `json:"user_id"`
	UserName       string              `json:"user_name"`
	UserAvatar     string              `json:"user_avatar"`
	ExerciseStats  []ExerciseProgress  `json:"exercise_stats"`
	CompletedCount int                 `json:"completed_count"` // 已完成的动作数
	TotalCount     int                 `json:"total_count"`     // 总动作数
	TotalSets      int                 `json:"total_sets"`      // 总组数
	CompletedSets  int                 `json:"completed_sets"`  // 已完成组数
	Progress       int                 `json:"progress"`        // 百分比（基于组数计算）
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
		date = time.Now().Format("2006-01-02")
	}

	progress := &GoalProgress{
		GoalID:   goal.ID,
		GoalName: goal.Name,
		Date:     date,
	}

	// 获取每个成员的进度
	for _, member := range goal.Members {
		memberProgress := MemberProgress{
			UserID:     member.UserID,
			UserName:   member.User.NickName,
			UserAvatar: member.User.AvatarURL,
		}

		// 获取该成员今天的完成记录
		completions, _ := s.goalRepo.GetMemberCompletions(goalID, member.UserID, date)
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

			if exProgress.CompletedSets >= exProgress.TargetSets {
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
		startDate = time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	}
	if endDate == "" {
		endDate = time.Now().Format("2006-01-02")
	}
	return s.goalRepo.GetHistoryCompletions(goalID, startDate, endDate)
}
