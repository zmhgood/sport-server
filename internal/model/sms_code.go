package model

import (
	"time"

	"gorm.io/gorm"
)

// SMSCode 短信验证码
type SMSCode struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Phone     string         `json:"phone" gorm:"size:20;index;not null"`
	Code      string         `json:"code" gorm:"size:6;not null"`
	Purpose   string         `json:"purpose" gorm:"size:20;not null"` // login, register, reset_password
	Used      bool           `json:"used" gorm:"default:false"`
	ExpiresAt time.Time      `json:"expires_at"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// IsExpired 检查验证码是否过期
func (s *SMSCode) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// IsValid 检查验证码是否有效
func (s *SMSCode) IsValid() bool {
	return !s.Used && !s.IsExpired()
}
