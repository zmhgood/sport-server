package service

import (
	"elderly-fitness/internal/model"
	"elderly-fitness/internal/repository"
)

type ExerciseService struct {
	exerciseRepo *repository.ExerciseRepository
	userRepo     *repository.UserRepository
}

func NewExerciseService(exerciseRepo *repository.ExerciseRepository, userRepo *repository.UserRepository) *ExerciseService {
	return &ExerciseService{
		exerciseRepo: exerciseRepo,
		userRepo:     userRepo,
	}
}

// GetMuscleGroups 获取肌肉分组列表
func (s *ExerciseService) GetMuscleGroups() ([]model.MuscleGroup, error) {
	return s.exerciseRepo.GetAllMuscleGroups()
}

// GetExercises 获取锻炼列表
func (s *ExerciseService) GetExercises(groupID uint) ([]model.Exercise, error) {
	if groupID > 0 {
		return s.exerciseRepo.GetExercisesByGroupID(groupID)
	}
	return s.exerciseRepo.GetAllExercises()
}

// GetExercisesList 获取锻炼列表（支持分页和筛选）
func (s *ExerciseService) GetExercisesList(page, pageSize int, muscleGroupID uint, difficulty, keyword string) ([]model.Exercise, int64, error) {
	return s.exerciseRepo.GetExercisesList(page, pageSize, muscleGroupID, difficulty, keyword)
}

// GetExerciseByID 获取锻炼详情
func (s *ExerciseService) GetExerciseByID(id uint) (*model.Exercise, error) {
	exercise, err := s.exerciseRepo.GetExerciseByID(id)
	if err != nil {
		return nil, err
	}

	// 获取步骤
	steps, err := s.exerciseRepo.GetExerciseSteps(id)
	if err == nil {
		exercise.Steps = steps
	}

	// 获取益处
	benefits, err := s.exerciseRepo.GetExerciseBenefits(id)
	if err == nil {
		for _, b := range benefits {
			exercise.Benefits = append(exercise.Benefits, b.Content)
		}
	}

	// 获取注意事项
	precautions, err := s.exerciseRepo.GetExercisePrecautions(id)
	if err == nil {
		for _, p := range precautions {
			exercise.Precautions = append(exercise.Precautions, p.Content)
		}
	}

	return exercise, nil
}

// GetRecommendExercises 获取推荐锻炼
func (s *ExerciseService) GetRecommendExercises(userID uint) ([]model.Exercise, error) {
	// TODO: 根据用户健康状况和锻炼历史推荐
	// 目前先返回简单的锻炼
	return s.exerciseRepo.GetExercisesByDifficulty("简单")
}

// RecordExercise 记录锻炼
func (s *ExerciseService) RecordExercise(record *model.UserExerciseRecord) error {
	return s.exerciseRepo.CreateExerciseRecord(record)
}

// CreateExercise 创建锻炼动作
func (s *ExerciseService) CreateExercise(exercise *model.Exercise) error {
	return s.exerciseRepo.CreateExercise(exercise)
}

// UpdateExercise 更新锻炼动作
func (s *ExerciseService) UpdateExercise(exercise *model.Exercise) error {
	return s.exerciseRepo.UpdateExercise(exercise)
}

// DeleteExercise 删除锻炼动作
func (s *ExerciseService) DeleteExercise(id uint) error {
	return s.exerciseRepo.DeleteExercise(id)
}
