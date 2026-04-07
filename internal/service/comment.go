package service

import (
	"encoding/json"
	"errors"

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
	if len(req.Content) == 0 {
		return nil, errors.New("评论内容不能为空")
	}
	if len(req.Content) > 500 {
		return nil, errors.New("评论内容不能超过500字")
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
	// 检查评论是否存在
	_, err := s.commentRepo.GetByID(commentID)
	if err != nil {
		return errors.New("评论不存在")
	}

	// 检查是否已点赞
	hasLiked, err := s.commentRepo.HasLiked(commentID, userID)
	if err != nil {
		return err
	}
	if hasLiked {
		return errors.New("已点赞过")
	}

	// 创建点赞记录
	like := &model.CommentLike{
		CommentID: commentID,
		UserID:    userID,
	}
	if err := s.commentRepo.CreateLike(like); err != nil {
		return err
	}

	// 增加点赞数
	return s.commentRepo.IncrementLikeCount(commentID)
}

// UnlikeComment 取消点赞
func (s *CommentService) UnlikeComment(commentID, userID uint) error {
	// 删除点赞记录
	if err := s.commentRepo.DeleteLike(commentID, userID); err != nil {
		return err
	}

	// 减少点赞数
	return s.commentRepo.DecrementLikeCount(commentID)
}

// ToggleLike 切换点赞状态
func (s *CommentService) ToggleLike(commentID, userID uint) (bool, error) {
	hasLiked, err := s.commentRepo.HasLiked(commentID, userID)
	if err != nil {
		return false, err
	}

	if hasLiked {
		return false, s.UnlikeComment(commentID, userID)
	}
	return true, s.LikeComment(commentID, userID)
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
