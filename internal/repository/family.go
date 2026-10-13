package repository

import (
	"elderly-fitness/internal/model"

	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
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
	err := r.db.Preload("Creator", func(db *gorm.DB) *gorm.DB { return db.Select("id", "nick_name") }).Preload("Members.User", func(db *gorm.DB) *gorm.DB { return db.Select("id", "nick_name", "avatar_url") }).First(&family, id).Error
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
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
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
var ErrAlreadyMember = errors.New("您已是该家庭成员")

func (r *FamilyRepository) AddMember(member *model.FamilyMember) error {
	var previous model.FamilyMember
	err := r.db.Unscoped().Where("family_id = ? AND user_id = ?", member.FamilyID, member.UserID).First(&previous).Error
	if err == nil {
		if !previous.DeletedAt.Valid {
			return ErrAlreadyMember
		}
		member.ID = previous.ID
		return r.db.Unscoped().Model(&previous).Updates(map[string]interface{}{
			"deleted_at": nil, "role": member.Role, "joined_at": member.JoinedAt,
		}).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return r.db.Create(member).Error
}

func (r *FamilyRepository) Transaction(fn func(*FamilyRepository) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error { return fn(NewFamilyRepository(tx)) })
}

func (r *FamilyRepository) LockFamily(id uint) (*model.Family, error) {
	var family model.Family
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&family, id).Error
	return &family, err
}

func (r *FamilyRepository) CreateWithAdmin(family *model.Family, userID uint) error {
	return r.Transaction(func(tx *FamilyRepository) error {
		var user model.User
		if err := tx.db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, userID).Error; err != nil {
			return err
		}
		if family.RequestKey != nil {
			var old model.Family
			err := tx.db.Unscoped().Where("creator_id=? AND request_key=?", userID, *family.RequestKey).First(&old).Error
			if err == nil {
				member, e := tx.IsMember(old.ID, userID)
				if old.DeletedAt.Valid || e != nil || !member {
					return errors.New("原创建结果已失效")
				}
				*family = old
				return nil
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		if err := tx.Create(family); err != nil {
			return err
		}
		return tx.AddMember(&model.FamilyMember{FamilyID: family.ID, UserID: userID, Role: "admin", JoinedAt: time.Now()})
	})
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
func (r *FamilyRepository) ListAll(page, pageSize int, keywords ...string) ([]model.Family, int64, error) {
	var families []model.Family
	var total int64

	query := r.db.Model(&model.Family{})
	if len(keywords) > 0 && keywords[0] != "" {
		query = query.Where("name LIKE ?", "%"+keywords[0]+"%")
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Preload("Creator", func(db *gorm.DB) *gorm.DB { return db.Select("id", "nick_name") }).Preload("Members.User", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "nick_name", "avatar_url")
	}).
		Order("created_at DESC").
		Offset(offset).Limit(pageSize).
		Find(&families).Error

	return families, total, err
}

// GetMembers 获取家庭成员列表
func (r *FamilyRepository) GetMembers(familyID uint) ([]model.FamilyMember, error) {
	var members []model.FamilyMember
	err := r.db.Where("family_id = ?", familyID).
		Preload("User", func(db *gorm.DB) *gorm.DB { return db.Select("id", "nick_name", "avatar_url") }).
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
