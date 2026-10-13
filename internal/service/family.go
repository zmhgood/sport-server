package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

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
func (s *FamilyService) CreateFamily(name string, creatorID uint, requestKeys ...string) (*model.Family, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 64 {
		return nil, errors.New("家庭名称须为1至64个字")
	}
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

	if len(requestKeys) > 0 && requestKeys[0] != "" {
		if len(requestKeys[0]) > 64 {
			return nil, errors.New("创建请求标识无效")
		}
		family.RequestKey = &requestKeys[0]
	}

	if err := s.familyRepo.CreateWithAdmin(family, creatorID); err != nil {
		return nil, errors.New("创建家庭失败，请重试")
	}

	return s.familyRepo.FindByID(family.ID)
}

// JoinFamily 加入家庭
func (s *FamilyService) JoinFamily(inviteCode string, userID uint) (*model.Family, error) {
	inviteCode = strings.ToLower(strings.TrimSpace(inviteCode))
	if len(inviteCode) != 6 {
		return nil, errors.New("邀请码无效")
	}
	family, err := s.familyRepo.FindByInviteCode(inviteCode)
	if err != nil {
		return nil, errors.New("邀请码无效")
	}
	err = s.familyRepo.Transaction(func(tx *repository.FamilyRepository) error {
		current, err := tx.LockFamily(family.ID)
		if err != nil || current.InviteCode != inviteCode {
			return errors.New("邀请码无效")
		}
		if err := tx.AddMember(&model.FamilyMember{FamilyID: family.ID, UserID: userID, Role: "member", JoinedAt: time.Now()}); err != nil {
			if errors.Is(err, repository.ErrAlreadyMember) {
				return err
			}
			return errors.New("加入家庭失败，请重试")
		}
		return nil
	})
	if err != nil {
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
	return s.familyRepo.Transaction(func(tx *repository.FamilyRepository) error {
		if _, err := tx.LockFamily(familyID); err != nil {
			return errors.New("家庭不存在")
		}
		member, err := tx.GetMemberByFamilyAndUser(familyID, userID)
		if err != nil {
			return errors.New("您不是该家庭成员")
		}
		if member.Role == "admin" {
			members, err := tx.GetMembers(familyID)
			if err != nil {
				return err
			}
			if len(members) > 1 {
				return errors.New("请先转移管理员权限或移除其他成员")
			}
			if err := tx.RemoveMember(familyID, userID); err != nil {
				return err
			}
			return tx.Delete(familyID)
		}
		return tx.RemoveMember(familyID, userID)
	})
}

func (s *FamilyService) RemoveMember(familyID, adminID, memberID uint) error {
	if adminID == memberID {
		return errors.New("不能移除自己，请使用退出家庭")
	}
	return s.familyRepo.Transaction(func(tx *repository.FamilyRepository) error {
		if _, err := tx.LockFamily(familyID); err != nil {
			return errors.New("家庭不存在")
		}
		admin, err := tx.GetMemberByFamilyAndUser(familyID, adminID)
		if err != nil || admin.Role != "admin" {
			return errors.New("无权限操作")
		}
		return tx.RemoveMember(familyID, memberID)
	})
}

func (s *FamilyService) TransferAdmin(familyID, currentAdminID, newAdminID uint) error {
	return s.familyRepo.Transaction(func(tx *repository.FamilyRepository) error {
		if _, err := tx.LockFamily(familyID); err != nil {
			return errors.New("家庭不存在")
		}
		admin, err := tx.GetMemberByFamilyAndUser(familyID, currentAdminID)
		if err != nil || admin.Role != "admin" {
			return errors.New("无权限操作")
		}
		if _, err := tx.GetMemberByFamilyAndUser(familyID, newAdminID); err != nil {
			return errors.New("新管理员必须是有效家庭成员")
		}
		if currentAdminID == newAdminID {
			return nil
		}
		if err := tx.UpdateMemberRole(familyID, newAdminID, "admin"); err != nil {
			return err
		}
		return tx.UpdateMemberRole(familyID, currentAdminID, "member")
	})
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
func (s *FamilyService) ListAllFamilies(page, pageSize int, keywords ...string) ([]model.Family, int64, error) {
	return s.familyRepo.ListAll(page, pageSize, keywords...)
}

// generateInviteCode 生成邀请码
func (s *FamilyService) generateInviteCode() (string, error) {
	bytes := make([]byte, 3)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
