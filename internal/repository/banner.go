package repository

import (
	"elderly-fitness/internal/model"
	"gorm.io/gorm/clause"
	"time"
)

// Save the complete list atomically, including simultaneous first saves.
func (r *SystemRepository) SetBanners(value string) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"value": value, "deleted_at": nil, "updated_at": time.Now()}),
	}).Create(&model.SystemConfig{Key: model.HomeBannersConfigKey, Value: value}).Error
}
