package model

import (
	"time"

	"gorm.io/gorm"
)

// User 用户模型
type User struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	OpenID       string         `json:"openid" gorm:"column:openid;uniqueIndex;size:64"`
	UnionID      string         `json:"unionid" gorm:"column:unionid;size:64"`
	NickName     string         `json:"nick_name" gorm:"size:64"`
	AvatarURL    string         `json:"avatar_url" gorm:"size:255"`
	Gender       int            `json:"gender"` // 0:未知 1:男 2:女
	Age          int            `json:"age"`
	Phone        string         `json:"phone" gorm:"size:20;uniqueIndex"`
	HealthStatus string         `json:"health_status" gorm:"type:text"` // 健康状况备注
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

// MuscleGroup 肌肉部位分组
type MuscleGroup struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	Name        string         `json:"name" gorm:"size:32;not null"`
	Description string         `json:"description" gorm:"size:255"`
	ImageURL    string         `json:"image_url" gorm:"size:255"`
	Sort        int            `json:"sort" gorm:"default:0"`
	Exercises   []Exercise     `json:"exercises" gorm:"foreignKey:MuscleGroupID"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

// Muscle 肌肉
type Muscle struct {
	ID             uint           `json:"id" gorm:"primaryKey"`
	MuscleGroupID  uint           `json:"muscle_group_id" gorm:"index"`
	Name           string         `json:"name" gorm:"size:32;not null"`
	Description    string         `json:"description" gorm:"size:255"`
	ImageURL       string         `json:"image_url" gorm:"size:255"`
	MuscleGroup    MuscleGroup    `json:"muscle_group" gorm:"foreignKey:MuscleGroupID"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}

// Exercise 锻炼动作
type Exercise struct {
	ID            uint           `json:"id" gorm:"primaryKey"`
	MuscleGroupID uint           `json:"muscle_group_id" gorm:"index"`
	Name          string         `json:"name" gorm:"size:64;not null"`
	TargetMuscle  string         `json:"target_muscle" gorm:"size:255"`
	Description   string         `json:"description" gorm:"type:text"`
	Difficulty    string         `json:"difficulty" gorm:"size:16"` // 简单/中等/困难
	Duration      int            `json:"duration"`                  // 分钟
	Sets          int            `json:"sets"`                      // 组数
	Reps          string         `json:"reps"`                      // 次数
	Calories      int            `json:"calories"`                  // 消耗热量
	ImageURL      string         `json:"image_url" gorm:"size:255"`
	GifURL        string         `json:"gif_url" gorm:"size:255"`
	VideoURL      string         `json:"video_url" gorm:"size:255"` // 保留兼容性
	Sort          int            `json:"sort" gorm:"default:0"`
	MuscleGroup   MuscleGroup    `json:"muscle_group" gorm:"foreignKey:MuscleGroupID"`
	Steps         []ExerciseStep `json:"steps" gorm:"foreignKey:ExerciseID"`
	Benefits      []string       `json:"benefits" gorm:"-"`
	Precautions   []string       `json:"precautions" gorm:"-"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`
}

// ExerciseStep 动作步骤
type ExerciseStep struct {
	ID         uint           `json:"id" gorm:"primaryKey"`
	ExerciseID uint           `json:"exercise_id" gorm:"index"`
	Order      int            `json:"order"`
	Title      string         `json:"title" gorm:"size:64"`
	Desc       string         `json:"desc" gorm:"type:text"`
	ImageURL   string         `json:"image_url" gorm:"size:255"`
	Duration   int            `json:"duration"` // 秒
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `json:"-" gorm:"index"`
}

// ExerciseBenefit 动作益处
type ExerciseBenefit struct {
	ID         uint           `json:"id" gorm:"primaryKey"`
	ExerciseID uint           `json:"exercise_id" gorm:"index"`
	Content    string         `json:"content" gorm:"size:255"`
	Sort       int            `json:"sort" gorm:"default:0"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `json:"-" gorm:"index"`
}

// ExercisePrecaution 注意事项
type ExercisePrecaution struct {
	ID         uint           `json:"id" gorm:"primaryKey"`
	ExerciseID uint           `json:"exercise_id" gorm:"index"`
	Content    string         `json:"content" gorm:"size:255"`
	Sort       int            `json:"sort" gorm:"default:0"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `json:"-" gorm:"index"`
}

// UserExerciseRecord 用户锻炼记录
type UserExerciseRecord struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	UserID      uint           `json:"user_id" gorm:"index"`
	ExerciseID  uint           `json:"exercise_id" gorm:"index"`
	Duration    int            `json:"duration"` // 秒
	Sets        int            `json:"sets"`     // 完成组数
	CompletedAt time.Time      `json:"completed_at"`
	User        User           `json:"user" gorm:"foreignKey:UserID"`
	Exercise    Exercise       `json:"exercise" gorm:"foreignKey:ExerciseID"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

// UserDailyStats 用户每日统计
type UserDailyStats struct {
	ID             uint           `json:"id" gorm:"primaryKey"`
	UserID         uint           `json:"user_id" gorm:"uniqueIndex:idx_user_date"`
	Date           time.Time      `json:"date" gorm:"type:date;uniqueIndex:idx_user_date"`
	ExerciseCount  int            `json:"exercise_count"`
	TotalDuration  int            `json:"total_duration"` // 秒
	TotalCalories  int            `json:"total_calories"`
	User           User           `json:"user" gorm:"foreignKey:UserID"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}

// ================== 家庭功能相关模型 ==================

// Family 家庭
type Family struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	Name        string         `json:"name" gorm:"size:64;not null"`
	CreatorID   uint           `json:"creator_id" gorm:"index"`
	InviteCode  string         `json:"invite_code" gorm:"size:16;uniqueIndex"`
	Members     []FamilyMember `json:"members" gorm:"foreignKey:FamilyID"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

// FamilyMember 家庭成员
type FamilyMember struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	FamilyID  uint           `json:"family_id" gorm:"index;uniqueIndex:idx_family_user"`
	UserID    uint           `json:"user_id" gorm:"index;uniqueIndex:idx_family_user"`
	Nickname  string         `json:"nickname" gorm:"size:32"` // 家庭内昵称
	Role      string         `json:"role" gorm:"size:16;default:'member'"` // admin/member
	JoinedAt  time.Time      `json:"joined_at"`
	User      User           `json:"user" gorm:"foreignKey:UserID"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// DailyGoal 每日目标
type DailyGoal struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	FamilyID  uint           `json:"family_id" gorm:"index"`
	Name      string         `json:"name" gorm:"size:64;not null"`
	CreatedBy uint           `json:"created_by" gorm:"index"`
	IsActive  bool           `json:"is_active" gorm:"default:true"`
	Family    Family         `json:"family" gorm:"foreignKey:FamilyID"`
	Members   []GoalMember   `json:"members" gorm:"foreignKey:GoalID"`
	Exercises []GoalExercise `json:"exercises" gorm:"foreignKey:GoalID"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// GoalMember 目标成员(参与该目标的家庭成员)
type GoalMember struct {
	ID          uint              `json:"id" gorm:"primaryKey"`
	GoalID      uint              `json:"goal_id" gorm:"index;uniqueIndex:idx_goal_member"`
	UserID      uint              `json:"user_id" gorm:"index;uniqueIndex:idx_goal_member"`
	User        User              `json:"user" gorm:"foreignKey:UserID"`
	Completions []GoalCompletion  `json:"completions" gorm:"foreignKey:GoalMemberID"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	DeletedAt   gorm.DeletedAt    `json:"-" gorm:"index"`
}

// GoalExercise 目标动作
type GoalExercise struct {
	ID          uint              `json:"id" gorm:"primaryKey"`
	GoalID      uint              `json:"goal_id" gorm:"index"`
	ExerciseID  uint              `json:"exercise_id" gorm:"index"`
	Sets        int               `json:"sets"`       // 要求组数
	Reps        int               `json:"reps"`       // 每组次数
	Exercise    Exercise          `json:"exercise" gorm:"foreignKey:ExerciseID"`
	Completions []GoalCompletion  `json:"completions" gorm:"foreignKey:GoalExerciseID"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	DeletedAt   gorm.DeletedAt    `json:"-" gorm:"index"`
}

// GoalCompletion 目标完成记录
type GoalCompletion struct {
	ID              uint           `json:"id" gorm:"primaryKey"`
	GoalMemberID    uint           `json:"goal_member_id" gorm:"index"`
	GoalExerciseID  uint           `json:"goal_exercise_id" gorm:"index"`
	Date            time.Time      `json:"date" gorm:"type:date;index"`
	CompletedSets   int            `json:"completed_sets"`   // 已完成组数
	Status          string         `json:"status" gorm:"size:16;default:'pending'"` // pending/done
	GoalMember      GoalMember     `json:"goal_member" gorm:"foreignKey:GoalMemberID"`
	GoalExercise    GoalExercise   `json:"goal_exercise" gorm:"foreignKey:GoalExerciseID"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index"`
}

// SystemConfig 系统配置
type SystemConfig struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Key       string         `json:"key" gorm:"size:64;uniqueIndex;not null"`
	Value     string         `json:"value" gorm:"type:text"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}
