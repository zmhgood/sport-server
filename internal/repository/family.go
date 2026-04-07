package repository

import (
	"elderly-fitness/internal/model"

	"gorm.io/gorm"
)

type FamilyRepository struct {
	db *gorm.DB
}

func NewFamilyRepository(db *gorm.DB) *FamilyRepository {
	return &FamilyRepository{db: db}
}

// Create 创建家庭
func (r *FamilyRepository) Create(family *model.Family) error {
	return r.db.Create(family).Error
}

// Update 更新家庭
func (r *FamilyRepository) Update(family *model.Family) error {
	return r.db.Save(family).Error
}

// FindByID 根据ID查找家庭
func (r *FamilyRepository) FindByID(id uint) (*model.Family, error) {
	var family model.Family
	err := r.db.Preload("Members.User").First(&family, id).Error
	if err != nil {
		return nil, err
	}
	return &family, nil
}

// FindByInviteCode 根据邀请码查找家庭
func (r *FamilyRepository) FindByInviteCode(code string) (*model.Family, error) {
	var family model.Family
	err := r.db.Where("invite_code = ?", code).First(&family).Error
	if err != nil {
		return nil, err
	}
	return &family, nil
}

// FindByUserID 根据用户ID查找所属家庭（支持多家庭）
func (r *FamilyRepository) FindByUserID(userID uint) ([]model.Family, error) {
	var members []model.FamilyMember
	err := r.db.Where("user_id = ?", userID).Find(&members).Error
	if err != nil {
		return nil, err
	}

	var families []model.Family
	for _, member := range members {
		family, err := r.FindByID(member.FamilyID)
		if err == nil {
			families = append(families, *family)
		}
	}
	return families, nil
}

// FindOneByUserID 根据用户ID查找第一个家庭（兼容旧逻辑）
func (r *FamilyRepository) FindOneByUserID(userID uint) (*model.Family, error) {
	var member model.FamilyMember
	err := r.db.Where("user_id = ?", userID).First(&member).Error
	if err != nil {
		return nil, err
	}
	return r.FindByID(member.FamilyID)
}

// AddMember 添加家庭成员
func (r *FamilyRepository) AddMember(member *model.FamilyMember) error {
	return r.db.Create(member).Error
}

// RemoveMember 移除家庭成员
func (r *FamilyRepository) RemoveMember(familyID, userID uint) error {
	return r.db.Where("family_id = ? AND user_id = ?", familyID, userID).
		Delete(&model.FamilyMember{}).Error
}

// GetMemberByUserID 根据用户ID获取家庭成员信息（返回第一个）
func (r *FamilyRepository) GetMemberByUserID(userID uint) (*model.FamilyMember, error) {
	var member model.FamilyMember
	err := r.db.Where("user_id = ?", userID).First(&member).Error
	if err != nil {
		return nil, err
	}
	return &member, nil
}

// GetMemberByFamilyAndUser 根据家庭ID和用户ID获取成员信息
func (r *FamilyRepository) GetMemberByFamilyAndUser(familyID, userID uint) (*model.FamilyMember, error) {
	var member model.FamilyMember
	err := r.db.Where("family_id = ? AND user_id = ?", familyID, userID).First(&member).Error
	if err != nil {
		return nil, err
	}
	return &member, nil
}

// ListAll 列出所有家庭（管理后台用）
func (r *FamilyRepository) ListAll(page, pageSize int) ([]model.Family, int64, error) {
	var families []model.Family
	var total int64

	r.db.Model(&model.Family{}).Count(&total)

	offset := (page - 1) * pageSize
	err := r.db.Preload("Members").
		Order("created_at DESC").
		Offset(offset).Limit(pageSize).
		Find(&families).Error

	return families, total, err
}

// GetMembers 获取家庭成员列表
func (r *FamilyRepository) GetMembers(familyID uint) ([]model.FamilyMember, error) {
	var members []model.FamilyMember
	err := r.db.Where("family_id = ?", familyID).
		Preload("User").
		Order("joined_at ASC").
		Find(&members).Error
	return members, err
}

// IsMember 检查用户是否是家庭成员
func (r *FamilyRepository) IsMember(familyID, userID uint) (bool, error) {
	var count int64
	err := r.db.Model(&model.FamilyMember{}).
		Where("family_id = ? AND user_id = ?", familyID, userID).
		Count(&count).Error
	return count > 0, err
}

// UpdateMemberRole 更新成员角色
func (r *FamilyRepository) UpdateMemberRole(familyID, userID uint, role string) error {
	return r.db.Model(&model.FamilyMember{}).
		Where("family_id = ? AND user_id = ?", familyID, userID).
		Update("role", role).Error
}

// Delete 删除家庭
func (r *FamilyRepository) Delete(id uint) error {
	return r.db.Delete(&model.Family{}, id).Error
}
