package model

import (
	"time"

	"gorm.io/gorm"
)

// Comment 评论模型
type Comment struct {
	ID         uint           `json:"id" gorm:"primaryKey"`
	ExerciseID uint           `json:"exercise_id" gorm:"index;not null"`
	UserID     uint           `json:"user_id" gorm:"index;not null"`
	ParentID   *uint          `json:"parent_id" gorm:"index"`           // NULL表示一级评论，否则为回复
	ReplyToUID *uint          `json:"reply_to_user_id"`                 // 回复的用户ID
	Content    string         `json:"content" gorm:"type:text;not null"`
	Images     string         `json:"images" gorm:"type:text"` // JSON数组存储图片URL
	LikeCount  int            `json:"like_count" gorm:"default:0"`
	Status     int            `json:"status" gorm:"default:1"` // 1:正常 0:隐藏 -1:删除
	User       User           `json:"user" gorm:"foreignKey:UserID"`
	Exercise   Exercise       `json:"exercise" gorm:"foreignKey:ExerciseID"`
	Replies    []Comment      `json:"replies" gorm:"foreignKey:ParentID"`
	IsLiked    bool           `json:"is_liked" gorm:"-"` // 当前用户是否点赞（不存数据库）
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `json:"-" gorm:"index"`
}

// CommentLike 评论点赞记录
type CommentLike struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	CommentID uint      `json:"comment_id" gorm:"index;not null"`
	UserID    uint      `json:"user_id" gorm:"index;not null"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName 指定表名
func (CommentLike) TableName() string {
	return "comment_likes"
}

// IsReply 是否为回复
func (c *Comment) IsReply() bool {
	return c.ParentID != nil && *c.ParentID > 0
}

// HasImages 是否有图片
func (c *Comment) HasImages() bool {
	return c.Images != "" && c.Images != "[]"
}
