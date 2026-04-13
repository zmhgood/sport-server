package repository

import (
	"time"

	"elderly-fitness/internal/model"

	"gorm.io/gorm"
)

type ExerciseRepository struct {
	db *gorm.DB
}

func NewExerciseRepository(db *gorm.DB) *ExerciseRepository {
	return &ExerciseRepository{db: db}
}

// GetAllMuscleGroups 获取所有肌肉分组
func (r *ExerciseRepository) GetAllMuscleGroups() ([]model.MuscleGroup, error) {
	var groups []model.MuscleGroup
	err := r.db.Order("sort ASC").Find(&groups).Error
	return groups, err
}

// GetAllExercises 获取所有锻炼
func (r *ExerciseRepository) GetAllExercises() ([]model.Exercise, error) {
	var exercises []model.Exercise
	err := r.db.Preload("MuscleGroup").Order("sort ASC").Find(&exercises).Error
	return exercises, err
}

// GetExercisesList 获取锻炼列表（支持分页和筛选）
func (r *ExerciseRepository) GetExercisesList(page, pageSize int, muscleGroupID uint, difficulty, keyword string) ([]model.Exercise, int64, error) {
	var exercises []model.Exercise
	var total int64

	db := r.db.Model(&model.Exercise{})

	if muscleGroupID > 0 {
		db = db.Where("muscle_group_id = ?", muscleGroupID)
	}
	if difficulty != "" {
		db = db.Where("difficulty = ?", difficulty)
	}
	if keyword != "" {
		db = db.Where("name LIKE ?", "%"+keyword+"%")
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := db.Preload("MuscleGroup").Order("sort ASC").Offset(offset).Limit(pageSize).Find(&exercises).Error
	return exercises, total, err
}

// GetExercisesByGroupID 根据分组ID获取锻炼
func (r *ExerciseRepository) GetExercisesByGroupID(groupID uint) ([]model.Exercise, error) {
	var exercises []model.Exercise
	err := r.db.Where("muscle_group_id = ?", groupID).
		Order("sort ASC").
		Find(&exercises).Error
	return exercises, err
}

// GetExercisesByDifficulty 根据难度获取锻炼
func (r *ExerciseRepository) GetExercisesByDifficulty(difficulty string) ([]model.Exercise, error) {
	var exercises []model.Exercise
	err := r.db.Where("difficulty = ?", difficulty).
		Order("sort ASC").
		Limit(10).
		Find(&exercises).Error
	return exercises, err
}

// GetExerciseByID 根据ID获取锻炼
func (r *ExerciseRepository) GetExerciseByID(id uint) (*model.Exercise, error) {
	var exercise model.Exercise
	err := r.db.First(&exercise, id).Error
	if err != nil {
		return nil, err
	}
	return &exercise, nil
}

// GetExerciseSteps 获取锻炼步骤
func (r *ExerciseRepository) GetExerciseSteps(exerciseID uint) ([]model.ExerciseStep, error) {
	var steps []model.ExerciseStep
	err := r.db.Where("exercise_id = ?", exerciseID).
		Order("`order` ASC").
		Find(&steps).Error
	return steps, err
}

// GetExerciseBenefits 获取锻炼益处
func (r *ExerciseRepository) GetExerciseBenefits(exerciseID uint) ([]model.ExerciseBenefit, error) {
	var benefits []model.ExerciseBenefit
	err := r.db.Where("exercise_id = ?", exerciseID).
		Order("sort ASC").
		Find(&benefits).Error
	return benefits, err
}

// GetExercisePrecautions 获取锻炼注意事项
func (r *ExerciseRepository) GetExercisePrecautions(exerciseID uint) ([]model.ExercisePrecaution, error) {
	var precautions []model.ExercisePrecaution
	err := r.db.Where("exercise_id = ?", exerciseID).
		Order("sort ASC").
		Find(&precautions).Error
	return precautions, err
}

// CreateExerciseRecord 创建锻炼记录
func (r *ExerciseRepository) CreateExerciseRecord(record *model.UserExerciseRecord) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 创建记录
		if err := tx.Create(record).Error; err != nil {
			return err
		}

		// 更新每日统计
		today := time.Now().Format("2006-01-02")
		var stats model.UserDailyStats
		err := tx.Where("user_id = ? AND date = ?", record.UserID, today).
			First(&stats).Error
		
		if err == gorm.ErrRecordNotFound {
			// 创建新的统计
			stats = model.UserDailyStats{
				UserID:        record.UserID,
				Date:          time.Now(),
				ExerciseCount: 1,
				TotalDuration: record.Duration,
				TotalCalories: 0, // TODO: 根据锻炼计算
			}
			return tx.Create(&stats).Error
		} else if err != nil {
			return err
		}

		// 更新现有统计
		return tx.Model(&stats).Updates(map[string]interface{}{
			"exercise_count": gorm.Expr("exercise_count + 1"),
			"total_duration": gorm.Expr("total_duration + ?", record.Duration),
		}).Error
	})
}

// GetUserExerciseRecords 获取用户锻炼记录
func (r *ExerciseRepository) GetUserExerciseRecords(userID uint, limit int) ([]model.UserExerciseRecord, error) {
	var records []model.UserExerciseRecord
	err := r.db.Where("user_id = ?", userID).
		Preload("Exercise").
		Order("created_at DESC").
		Limit(limit).
		Find(&records).Error
	return records, err
}

// GetUserTodayExerciseCount 获取用户今日锻炼次数
func (r *ExerciseRepository) GetUserTodayExerciseCount(userID uint, date string) (int, error) {
	var count int64
	err := r.db.Model(&model.UserExerciseRecord{}).
		Where("user_id = ? AND DATE(created_at) = ?", userID, date).
		Count(&count).Error
	return int(count), err
}

// Count 获取锻炼动作总数
func (r *ExerciseRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&model.Exercise{}).Count(&count).Error
	return count, err
}

// GetTodayAllUsersExerciseCount 获取今日所有用户的锻炼记录数
func (r *ExerciseRepository) GetTodayAllUsersExerciseCount() (int64, error) {
	var count int64
	today := time.Now().Format("2006-01-02")
	err := r.db.Model(&model.UserExerciseRecord{}).
		Where("DATE(created_at) = ?", today).
		Count(&count).Error
	return count, err
}

// CreateExercise 创建锻炼动作
func (r *ExerciseRepository) CreateExercise(exercise *model.Exercise) error {
	return r.db.Create(exercise).Error
}

// UpdateExercise 更新锻炼动作
func (r *ExerciseRepository) UpdateExercise(exercise *model.Exercise) error {
	return r.db.Model(&model.Exercise{}).Where("id = ?", exercise.ID).Updates(map[string]interface{}{
		"muscle_group_id": exercise.MuscleGroupID,
		"name":            exercise.Name,
		"target_muscle":   exercise.TargetMuscle,
		"description":     exercise.Description,
		"difficulty":      exercise.Difficulty,
		"duration":        exercise.Duration,
		"sets":            exercise.Sets,
		"reps":            exercise.Reps,
		"calories":        exercise.Calories,
		"image_url":       exercise.ImageURL,
		"gif_url":         exercise.GifURL,
		"video_url":       exercise.VideoURL,
		"sort":            exercise.Sort,
	}).Error
}

// DeleteExercise 删除锻炼动作
func (r *ExerciseRepository) DeleteExercise(id uint) error {
	return r.db.Delete(&model.Exercise{}, id).Error
}
