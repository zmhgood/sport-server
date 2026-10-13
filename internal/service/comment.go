package service

import (
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"elderly-fitness/internal/model"
	"elderly-fitness/internal/repository"
)

type CommentService struct {
	commentRepo *repository.CommentRepository
}

func NewCommentService(commentRepo *repository.CommentRepository) *CommentService {
	return &CommentService{
		commentRepo: commentRepo,
	}
}

// CreateCommentRequest 创建评论请求
type CreateCommentRequest struct {
	ExerciseID uint     `json:"exercise_id"`
	ParentID   *uint    `json:"parent_id"`
	ReplyToUID *uint    `json:"reply_to_user_id"`
	Content    string   `json:"content"`
	Images     []string `json:"images"`
}

// CreateComment 创建评论
func (s *CommentService) CreateComment(userID uint, req *CreateCommentRequest) (*model.Comment, error) {
	// 验证内容
	req.Content = strings.TrimSpace(req.Content)
	if len(req.Content) == 0 {
		return nil, errors.New("评论内容不能为空")
	}
	if utf8.RuneCountInString(req.Content) > 500 {
		return nil, errors.New("评论内容不能超过500字")
	}

	if err := s.commentRepo.ValidateReferences(req.ExerciseID, req.ParentID); err != nil {
		return nil, err
	}
	// 处理图片
	var imagesJSON string
	if len(req.Images) > 0 {
		imagesBytes, _ := json.Marshal(req.Images)
		imagesJSON = string(imagesBytes)
	}

	comment := &model.Comment{
		ExerciseID: req.ExerciseID,
		UserID:     userID,
		ParentID:   req.ParentID,
		ReplyToUID: req.ReplyToUID,
		Content:    req.Content,
		Images:     imagesJSON,
		Status:     1,
	}

	if err := s.commentRepo.Create(comment); err != nil {
		return nil, errors.New("创建评论失败")
	}

	// 加载用户信息
	comment, _ = s.commentRepo.GetByID(comment.ID)

	return comment, nil
}

// DeleteComment 删除评论
func (s *CommentService) DeleteComment(commentID, userID uint) error {
	comment, err := s.commentRepo.GetByID(commentID)
	if err != nil {
		return errors.New("评论不存在")
	}

	// 只能删除自己的评论
	if comment.UserID != userID {
		return errors.New("无权删除此评论")
	}

	return s.commentRepo.Delete(commentID)
}

// GetCommentList 获取评论列表
func (s *CommentService) GetCommentList(exerciseID, userID uint, page, pageSize int) (*CommentListResult, error) {
	comments, total, err := s.commentRepo.GetByExerciseID(exerciseID, page, pageSize)
	if err != nil {
		return nil, err
	}

	// 检查用户是否点赞过
	for i := range comments {
		liked, _ := s.commentRepo.HasLiked(comments[i].ID, userID)
		comments[i].IsLiked = liked
	}

	return &CommentListResult{
		List:     comments,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// CommentListResult 评论列表结果
type CommentListResult struct {
	List     []model.Comment `json:"list"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

// LikeComment 点赞评论
func (s *CommentService) LikeComment(commentID, userID uint) error {
	_, err := s.commentRepo.ChangeLike(commentID, userID, true, false)
	return err
}
func (s *CommentService) UnlikeComment(commentID, userID uint) error {
	_, err := s.commentRepo.ChangeLike(commentID, userID, false, false)
	return err
}
func (s *CommentService) ToggleLike(commentID, userID uint) (bool, error) {
	return s.commentRepo.ChangeLike(commentID, userID, false, true)
}

// GetUserComments 获取用户的评论
func (s *CommentService) GetUserComments(userID uint, page, pageSize int) (*CommentListResult, error) {
	comments, total, err := s.commentRepo.GetCommentsByUserID(userID, page, pageSize)
	if err != nil {
		return nil, err
	}

	return &CommentListResult{
		List:     comments,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetAllComments 获取所有评论（管理后台用）
func (s *CommentService) GetAllComments(status int, keyword string, page, pageSize int) (*CommentListResult, error) {
	comments, total, err := s.commentRepo.GetAllComments(status, keyword, page, pageSize)
	if err != nil {
		return nil, err
	}

	return &CommentListResult{
		List:     comments,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// UpdateCommentStatus 更新评论状态
func (s *CommentService) UpdateCommentStatus(commentID uint, status int) error {
	return s.commentRepo.UpdateStatus(commentID, status)
}

// AdminDeleteComment 管理员删除评论
func (s *CommentService) AdminDeleteComment(commentID uint) error {
	return s.commentRepo.Delete(commentID)
}
