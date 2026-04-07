package repository

import (
	"elderly-fitness/internal/model"

	"gorm.io/gorm"
)

type SMSCodeRepository struct {
	db *gorm.DB
}

func NewSMSCodeRepository(db *gorm.DB) *SMSCodeRepository {
	return &SMSCodeRepository{db: db}
}

// Create 创建验证码记录
func (r *SMSCodeRepository) Create(smsCode *model.SMSCode) error {
	return r.db.Create(smsCode).Error
}

// FindLatestByPhone 查找手机号最新的验证码
func (r *SMSCodeRepository) FindLatestByPhone(phone, purpose string) (*model.SMSCode, error) {
	var smsCode model.SMSCode
	err := r.db.Where("phone = ? AND purpose = ?", phone, purpose).
		Order("created_at DESC").
		First(&smsCode).Error
	if err != nil {
		return nil, err
	}
	return &smsCode, nil
}

// Update 更新验证码记录
func (r *SMSCodeRepository) Update(smsCode *model.SMSCode) error {
	return r.db.Save(smsCode).Error
}
