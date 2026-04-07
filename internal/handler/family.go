package handler

import (
	"net/http"
	"strconv"

	"elderly-fitness/internal/service"

	"github.com/gin-gonic/gin"
)

type FamilyHandler struct {
	familyService *service.FamilyService
}

func NewFamilyHandler(familyService *service.FamilyService) *FamilyHandler {
	return &FamilyHandler{familyService: familyService}
}

// CreateFamilyRequest 创建家庭请求
type CreateFamilyRequest struct {
	Name string `json:"name" binding:"required"`
}

// CreateFamily 创建家庭
func (h *FamilyHandler) CreateFamily(c *gin.Context) {
	userID, _ := c.Get("userID")

	var req CreateFamilyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "参数错误"})
		return
	}

	family, err := h.familyService.CreateFamily(req.Name, userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Code: 500, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "创建成功", Data: family})
}

// JoinFamilyRequest 加入家庭请求
type JoinFamilyRequest struct {
	InviteCode string `json:"invite_code" binding:"required"`
}

// JoinFamily 加入家庭
func (h *FamilyHandler) JoinFamily(c *gin.Context) {
	userID, _ := c.Get("userID")

	var req JoinFamilyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "参数错误"})
		return
	}

	family, err := h.familyService.JoinFamily(req.InviteCode, userID.(uint))
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "加入成功", Data: family})
}

// GetMyFamilies 获取我的所有家庭
func (h *FamilyHandler) GetMyFamilies(c *gin.Context) {
	userID, _ := c.Get("userID")

	families, err := h.familyService.GetFamiliesByUserID(userID.(uint))
	if err != nil {
		c.JSON(http.StatusOK, Response{Code: 0, Message: "暂未加入家庭", Data: []interface{}{}})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "success", Data: families})
}

// GetFamily 获取指定家庭信息
func (h *FamilyHandler) GetFamily(c *gin.Context) {
	familyIDStr := c.Param("id")
	familyID, err := strconv.ParseUint(familyIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "参数错误"})
		return
	}

	family, err := h.familyService.GetFamily(uint(familyID))
	if err != nil {
		c.JSON(http.StatusNotFound, Response{Code: 404, Message: "家庭不存在"})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "success", Data: family})
}

// GetFamilyMembers 获取家庭成员列表
func (h *FamilyHandler) GetFamilyMembers(c *gin.Context) {
	familyIDStr := c.Query("family_id")
	if familyIDStr == "" {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "缺少family_id参数"})
		return
	}

	familyID, err := strconv.ParseUint(familyIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "参数错误"})
		return
	}

	members, err := h.familyService.GetFamilyMembers(uint(familyID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Code: 500, Message: "获取失败"})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "success", Data: members})
}

// LeaveFamilyRequest 退出家庭请求
type LeaveFamilyRequest struct {
	FamilyID uint `json:"family_id" binding:"required"`
}

// LeaveFamily 退出家庭
func (h *FamilyHandler) LeaveFamily(c *gin.Context) {
	userID, _ := c.Get("userID")

	var req LeaveFamilyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "参数错误"})
		return
	}

	if err := h.familyService.LeaveFamily(req.FamilyID, userID.(uint)); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "退出成功"})
}

// RemoveMemberRequest 移除成员请求
type RemoveMemberRequest struct {
	FamilyID uint `json:"family_id" binding:"required"`
	MemberID uint `json:"member_id" binding:"required"`
}

// RemoveMember 移除成员
func (h *FamilyHandler) RemoveMember(c *gin.Context) {
	userID, _ := c.Get("userID")

	var req RemoveMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "参数错误"})
		return
	}

	if err := h.familyService.RemoveMember(req.FamilyID, userID.(uint), req.MemberID); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "移除成功"})
}

// TransferAdminRequest 转移管理员请求
type TransferAdminRequest struct {
	FamilyID   uint `json:"family_id" binding:"required"`
	NewAdminID uint `json:"new_admin_id" binding:"required"`
}

// TransferAdmin 转移管理员
func (h *FamilyHandler) TransferAdmin(c *gin.Context) {
	userID, _ := c.Get("userID")

	var req TransferAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "参数错误"})
		return
	}

	if err := h.familyService.TransferAdmin(req.FamilyID, userID.(uint), req.NewAdminID); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "转移成功"})
}

// RefreshInviteCodeRequest 刷新邀请码请求
type RefreshInviteCodeRequest struct {
	FamilyID uint `json:"family_id" binding:"required"`
}

// RefreshInviteCode 刷新邀请码
func (h *FamilyHandler) RefreshInviteCode(c *gin.Context) {
	userID, _ := c.Get("userID")

	var req RefreshInviteCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "参数错误"})
		return
	}

	newCode, err := h.familyService.GenerateNewInviteCode(req.FamilyID, userID.(uint))
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "刷新成功",
		Data: gin.H{
			"invite_code": newCode,
		},
	})
}

// ============ 管理后台接口 ============

// ListFamiliesForAdmin 管理后台获取家庭列表
func (h *FamilyHandler) ListFamiliesForAdmin(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	families, total, err := h.familyService.ListAllFamilies(page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Code: 500, Message: "获取失败"})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"list":      families,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}
