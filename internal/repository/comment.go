package repository

import (
	"elderly-fitness/internal/model"

	"gorm.io/gorm"
)

type CommentRepository struct {
	db *gorm.DB
}

func NewCommentRepository(db *gorm.DB) *CommentRepository {
	return &CommentRepository{db: db}
}

// Create 创建评论
func (r *CommentRepository) Create(comment *model.Comment) error {
	return r.db.Create(comment).Error
}

// Update 更新评论
func (r *CommentRepository) Update(comment *model.Comment) error {
	return r.db.Save(comment).Error
}

// Delete 软删除评论
func (r *CommentRepository) Delete(id uint) error {
	return r.db.Delete(&model.Comment{}, id).Error
}

// GetByID 根据ID获取评论
func (r *CommentRepository) GetByID(id uint) (*model.Comment, error) {
	var comment model.Comment
	err := r.db.Preload("User").First(&comment, id).Error
	if err != nil {
		return nil, err
	}
	return &comment, nil
}

// GetByExerciseID 获取锻炼动作的评论列表
func (r *CommentRepository) GetByExerciseID(exerciseID uint, page, pageSize int) ([]model.Comment, int64, error) {
	var comments []model.Comment
	var total int64

	// 统计一级评论数量
	r.db.Model(&model.Comment{}).
		Where("exercise_id = ? AND parent_id IS NULL AND status = 1", exerciseID).
		Count(&total)

	// 获取一级评论
	offset := (page - 1) * pageSize
	err := r.db.Preload("User").
		Where("exercise_id = ? AND parent_id IS NULL AND status = 1", exerciseID).
		Order("created_at DESC").
		Offset(offset).Limit(pageSize).
		Find(&comments).Error

	if err != nil {
		return nil, 0, err
	}

	// 获取每个一级评论的回复
	for i := range comments {
		var replies []model.Comment
		r.db.Preload("User").
			Where("parent_id = ? AND status = 1", comments[i].ID).
			Order("created_at ASC").
			Find(&replies)
		comments[i].Replies = replies
	}

	return comments, total, nil
}

// GetCommentsByUserID 获取用户的评论列表
func (r *CommentRepository) GetCommentsByUserID(userID uint, page, pageSize int) ([]model.Comment, int64, error) {
	var comments []model.Comment
	var total int64

	r.db.Model(&model.Comment{}).Where("user_id = ? AND status = 1", userID).Count(&total)

	offset := (page - 1) * pageSize
	err := r.db.Preload("Exercise").
		Where("user_id = ? AND status = 1", userID).
		Order("created_at DESC").
		Offset(offset).Limit(pageSize).
		Find(&comments).Error

	return comments, total, err
}

// CreateLike 创建点赞
func (r *CommentRepository) CreateLike(like *model.CommentLike) error {
	return r.db.Create(like).Error
}

// DeleteLike 删除点赞
func (r *CommentRepository) DeleteLike(commentID, userID uint) error {
	return r.db.Where("comment_id = ? AND user_id = ?", commentID, userID).
		Delete(&model.CommentLike{}).Error
}

// HasLiked 检查是否已点赞
func (r *CommentRepository) HasLiked(commentID, userID uint) (bool, error) {
	var count int64
	err := r.db.Model(&model.CommentLike{}).
		Where("comment_id = ? AND user_id = ?", commentID, userID).
		Count(&count).Error
	return count > 0, err
}

// IncrementLikeCount 增加点赞数
func (r *CommentRepository) IncrementLikeCount(commentID uint) error {
	return r.db.Model(&model.Comment{}).
		Where("id = ?", commentID).
		UpdateColumn("like_count", gorm.Expr("like_count + 1")).Error
}

// DecrementLikeCount 减少点赞数
func (r *CommentRepository) DecrementLikeCount(commentID uint) error {
	return r.db.Model(&model.Comment{}).
		Where("id = ? AND like_count > 0", commentID).
		UpdateColumn("like_count", gorm.Expr("like_count - 1")).Error
}

// GetCommentCount 获取动作的评论数
func (r *CommentRepository) GetCommentCount(exerciseID uint) (int64, error) {
	var count int64
	err := r.db.Model(&model.Comment{}).
		Where("exercise_id = ? AND status = 1", exerciseID).
		Count(&count).Error
	return count, err
}

// GetAllComments 获取所有评论（管理后台用）
func (r *CommentRepository) GetAllComments(status int, keyword string, page, pageSize int) ([]model.Comment, int64, error) {
	var comments []model.Comment
	var total int64

	query := r.db.Model(&model.Comment{}).Preload("User").Preload("Exercise")

	if status >= 0 {
		query = query.Where("status = ?", status)
	}
	if keyword != "" {
		query = query.Where("content LIKE ?", "%"+keyword+"%")
	}

	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&comments).Error
	return comments, total, err
}

// UpdateStatus 更新评论状态
func (r *CommentRepository) UpdateStatus(id uint, status int) error {
	return r.db.Model(&model.Comment{}).Where("id = ?", id).Update("status", status).Error
}

// Count 获取评论总数
func (r *CommentRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&model.Comment{}).Count(&count).Error
	return count, err
}
