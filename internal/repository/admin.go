package repository

import (
	"elderly-fitness/internal/model"

	"gorm.io/gorm"
)

type AdminRepository struct {
	db *gorm.DB
}

func NewAdminRepository(db *gorm.DB) *AdminRepository {
	return &AdminRepository{db: db}
}

// FindByUsername 根据用户名查找管理员
func (r *AdminRepository) FindByUsername(username string) (*model.Admin, error) {
	var admin model.Admin
	err := r.db.Where("username = ?", username).First(&admin).Error
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

// FindByID 根据ID查找管理员
func (r *AdminRepository) FindByID(id uint) (*model.Admin, error) {
	var admin model.Admin
	err := r.db.First(&admin, id).Error
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

// Create 创建管理员
func (r *AdminRepository) Create(admin *model.Admin) error {
	return r.db.Create(admin).Error
}

// ListUsers 获取用户列表
func (r *AdminRepository) ListUsers(page, pageSize int, keyword string, gender *int) ([]model.User, int64, error) {
	var users []model.User
	var total int64

	query := r.db.Model(&model.User{})
	if keyword != "" {
		query = query.Where("nick_name LIKE ? OR phone LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if gender != nil {
		query = query.Where("gender = ?", *gender)
	}

	query.Count(&total)
	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&users).Error
	return users, total, err
}

// ListAllComments 获取所有评论列表（管理后台用）
func (r *AdminRepository) ListAllComments(page, pageSize int, status *int, keyword string) ([]model.Comment, int64, error) {
	var comments []model.Comment
	var total int64

	query := r.db.Model(&model.Comment{})
	if status != nil {
		query = query.Where("status = ?", *status)
	}
	if keyword != "" {
		query = query.Where("content LIKE ?", "%"+keyword+"%")
	}

	query.Count(&total)
	offset := (page - 1) * pageSize
	err := r.db.Preload("User").Preload("Exercise").
		Where(query.Statement.Clauses["WHERE"].Expression).
		Order("created_at DESC").
		Offset(offset).Limit(pageSize).
		Find(&comments).Error
	
	if err != nil {
		return nil, 0, err
	}

	// 重新构建查询
	query = r.db.Model(&model.Comment{})
	if status != nil {
		query = query.Where("status = ?", *status)
	}
	if keyword != "" {
		query = query.Where("content LIKE ?", "%"+keyword+"%")
	}
	query.Count(&total)

	err = r.db.Preload("User").Preload("Exercise").
		Scopes(func(d *gorm.DB) *gorm.DB {
			if status != nil {
				d = d.Where("status = ?", *status)
			}
			if keyword != "" {
				d = d.Where("content LIKE ?", "%"+keyword+"%")
			}
			return d
		}).
		Order("created_at DESC").
		Offset(offset).Limit(pageSize).
		Find(&comments).Error

	return comments, total, err
}

// UpdateCommentStatus 更新评论状态
func (r *AdminRepository) UpdateCommentStatus(id uint, status int) error {
	return r.db.Model(&model.Comment{}).Where("id = ?", id).Update("status", status).Error
}
