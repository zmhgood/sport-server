package repository

import (
	"elderly-fitness/internal/model"
	"gorm.io/gorm"
)

func (r *FamilyRepository) Rename(id uint, name string) error {
	result := r.db.Model(&model.Family{}).Where("id = ?", id).Update("name", name)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
