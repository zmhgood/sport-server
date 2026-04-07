package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"elderly-fitness/internal/model"
	"elderly-fitness/internal/repository"
)

type FamilyService struct {
	familyRepo *repository.FamilyRepository
}

func NewFamilyService(familyRepo *repository.FamilyRepository) *FamilyService {
	return &FamilyService{familyRepo: familyRepo}
}

// CreateFamily 创建家庭
func (s *FamilyService) CreateFamily(name string, creatorID uint) (*model.Family, error) {
	// 生成邀请码
	inviteCode, err := s.generateInviteCode()
	if err != nil {
		return nil, err
	}

	family := &model.Family{
		Name:       name,
		CreatorID:  creatorID,
		InviteCode: inviteCode,
	}

	if err := s.familyRepo.Create(family); err != nil {
		return nil, err
	}

	// 创建者成为管理员
	member := &model.FamilyMember{
		FamilyID: family.ID,
		UserID:   creatorID,
		Role:     "admin",
		JoinedAt: time.Now(),
	}
	if err := s.familyRepo.AddMember(member); err != nil {
		return nil, err
	}

	return s.familyRepo.FindByID(family.ID)
}

// JoinFamily 加入家庭
func (s *FamilyService) JoinFamily(inviteCode string, userID uint) (*model.Family, error) {
	// 查找家庭
	family, err := s.familyRepo.FindByInviteCode(inviteCode)
	if err != nil {
		return nil, errors.New("邀请码无效")
	}

	// 检查是否已是成员
	isMember, _ := s.familyRepo.IsMember(family.ID, userID)
	if isMember {
		return nil, errors.New("您已是该家庭成员")
	}

	// 加入家庭
	member := &model.FamilyMember{
		FamilyID: family.ID,
		UserID:   userID,
		Role:     "member",
		JoinedAt: time.Now(),
	}
	if err := s.familyRepo.AddMember(member); err != nil {
		return nil, err
	}

	return s.familyRepo.FindByID(family.ID)
}

// GetFamily 获取家庭信息
func (s *FamilyService) GetFamily(familyID uint) (*model.Family, error) {
	return s.familyRepo.FindByID(familyID)
}

// GetFamilyByUserID 根据用户ID获取家庭（返回所有家庭）
func (s *FamilyService) GetFamilyByUserID(userID uint) ([]model.Family, error) {
	return s.familyRepo.FindByUserID(userID)
}

// GetFamiliesByUserID 获取用户所有家庭
func (s *FamilyService) GetFamiliesByUserID(userID uint) ([]model.Family, error) {
	return s.familyRepo.FindByUserID(userID)
}

// GetFamilyMembers 获取家庭成员列表
func (s *FamilyService) GetFamilyMembers(familyID uint) ([]model.FamilyMember, error) {
	return s.familyRepo.GetMembers(familyID)
}

// LeaveFamily 退出家庭
func (s *FamilyService) LeaveFamily(familyID, userID uint) error {
	// 检查是否是家庭成员
	member, err := s.familyRepo.GetMemberByFamilyAndUser(familyID, userID)
	if err != nil {
		return errors.New("您不是该家庭成员")
	}

	if member.Role == "admin" {
		// 检查是否还有其他成员
		members, _ := s.familyRepo.GetMembers(familyID)
		if len(members) > 1 {
			return errors.New("请先转移管理员权限或移除其他成员")
		}
		// 最后一个成员，删除家庭
		return s.familyRepo.Delete(familyID)
	}

	return s.familyRepo.RemoveMember(familyID, userID)
}

// RemoveMember 移除成员（管理员操作）
func (s *FamilyService) RemoveMember(familyID, adminID, memberID uint) error {
	// 检查操作者是否是该家庭的管理员
	adminMember, err := s.familyRepo.GetMemberByFamilyAndUser(familyID, adminID)
	if err != nil || adminMember.Role != "admin" {
		return errors.New("无权限操作")
	}

	return s.familyRepo.RemoveMember(familyID, memberID)
}

// TransferAdmin 转移管理员权限
func (s *FamilyService) TransferAdmin(familyID, currentAdminID, newAdminID uint) error {
	// 检查当前用户是否是该家庭的管理员
	adminMember, err := s.familyRepo.GetMemberByFamilyAndUser(familyID, currentAdminID)
	if err != nil || adminMember.Role != "admin" {
		return errors.New("无权限操作")
	}

	// 将新管理员设为admin
	if err := s.familyRepo.UpdateMemberRole(familyID, newAdminID, "admin"); err != nil {
		return err
	}

	// 将当前管理员设为member
	return s.familyRepo.UpdateMemberRole(familyID, currentAdminID, "member")
}

// GenerateNewInviteCode 生成新的邀请码
func (s *FamilyService) GenerateNewInviteCode(familyID, userID uint) (string, error) {
	// 检查是否是该家庭的管理员
	member, err := s.familyRepo.GetMemberByFamilyAndUser(familyID, userID)
	if err != nil || member.Role != "admin" {
		return "", errors.New("无权限操作")
	}

	family, err := s.familyRepo.FindByID(familyID)
	if err != nil {
		return "", err
	}

	newCode, err := s.generateInviteCode()
	if err != nil {
		return "", err
	}

	family.InviteCode = newCode
	if err := s.familyRepo.Update(family); err != nil {
		return "", err
	}

	return newCode, nil
}

// ListAllFamilies 列出所有家庭（管理后台用）
func (s *FamilyService) ListAllFamilies(page, pageSize int) ([]model.Family, int64, error) {
	return s.familyRepo.ListAll(page, pageSize)
}

// generateInviteCode 生成邀请码
func (s *FamilyService) generateInviteCode() (string, error) {
	bytes := make([]byte, 3)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
