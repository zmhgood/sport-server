package model

import (
	"time"

	"gorm.io/gorm"
)

// Admin 管理员模型
type Admin struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Username  string         `json:"username" gorm:"uniqueIndex;size:32;not null"`
	Password  string         `json:"-" gorm:"size:128;not null"` // 存储加密后的密码
	Nickname  string         `json:"nickname" gorm:"size:32"`
	Avatar    string         `json:"avatar" gorm:"size:255"`
	Role      string         `json:"role" gorm:"size:16;default:'admin'"` // admin, super_admin
	Status    int            `json:"status" gorm:"default:1"`             // 1:正常 0:禁用
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 指定表名
func (Admin) TableName() string {
	return "admins"
}
