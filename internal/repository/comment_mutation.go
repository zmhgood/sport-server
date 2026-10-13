package repository

import (
	"elderly-fitness/internal/model"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *CommentRepository) ChangeLike(id, userID uint, want, toggle bool) (bool, error) {
	var liked bool
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var comment model.Comment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND status=1", id).First(&comment).Error; err != nil {
			return errors.New("评论不存在")
		}
		var n int64
		if err := tx.Model(&model.CommentLike{}).Where("comment_id=? AND user_id=?", id, userID).Count(&n).Error; err != nil {
			return err
		}
		liked = want
		if toggle {
			liked = n == 0
		}
		if liked && n == 0 {
			if err := tx.Create(&model.CommentLike{CommentID: id, UserID: userID}).Error; err != nil {
				return err
			}
		}
		if !liked {
			if err := tx.Where("comment_id=? AND user_id=?", id, userID).Delete(&model.CommentLike{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&model.CommentLike{}).Where("comment_id=?", id).Distinct("user_id").Count(&n).Error; err != nil {
			return err
		}
		return tx.Model(&comment).UpdateColumn("like_count", n).Error
	})
	return liked, err
}
func (r *CommentRepository) ValidateReferences(exerciseID uint, parentID *uint) error {
	var ex model.Exercise
	if err := r.db.First(&ex, exerciseID).Error; err != nil {
		return errors.New("动作不存在")
	}
	if parentID != nil {
		var parent model.Comment
		if err := r.db.Where("id=? AND exercise_id=? AND status=1", *parentID, exerciseID).First(&parent).Error; err != nil {
			return errors.New("回复评论不属于该动作")
		}
	}
	return nil
}
