package repository

import (
	"log"

	"elderly-fitness/internal/model"

	"gorm.io/gorm"
)

type SystemRepository struct {
	db *gorm.DB
}

func NewSystemRepository(db *gorm.DB) *SystemRepository {
	return &SystemRepository{db: db}
}

// GetConfig 获取配置
func (r *SystemRepository) GetConfig(key string) (*model.SystemConfig, error) {
	var config model.SystemConfig
	err := r.db.Where("`key` = ?", key).First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// SetConfig 设置配置
func (r *SystemRepository) SetConfig(key, value string) error {
	var config model.SystemConfig
	// 使用 Unscoped 查找（包括软删除的记录），key 是保留字需要反引号
	result := r.db.Unscoped().Where("`key` = ?", key).First(&config)
	
	if result.Error == gorm.ErrRecordNotFound {
		// 创建新记录
		config = model.SystemConfig{
			Key:   key,
			Value: value,
		}
		if err := r.db.Create(&config).Error; err != nil {
			log.Printf("创建配置失败 key=%s, error=%v", key, err)
			return err
		}
		return nil
	}
	
	if result.Error != nil {
		log.Printf("查询配置失败 key=%s, error=%v", key, result.Error)
		return result.Error
	}
	
	// 更新记录，如果被软删除则恢复
	updates := map[string]interface{}{
		"value": value,
	}
	if config.DeletedAt.Valid {
		updates["deleted_at"] = nil
	}
	
	if err := r.db.Unscoped().Model(&config).Updates(updates).Error; err != nil {
		log.Printf("更新配置失败 key=%s, error=%v", key, err)
		return err
	}
	return nil
}

// GetAllConfigs 获取所有配置
func (r *SystemRepository) GetAllConfigs() ([]model.SystemConfig, error) {
	var configs []model.SystemConfig
	err := r.db.Find(&configs).Error
	return configs, err
}
