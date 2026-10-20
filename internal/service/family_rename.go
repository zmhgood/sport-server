package service

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"elderly-fitness/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrFamilyNameInvalid     = errors.New("家庭名称须为1至64个字，且不含控制字符")
	ErrFamilyRenameForbidden = errors.New("只有家庭管理员可以修改家庭名称")
	ErrFamilyRenameNotFound  = errors.New("家庭不存在")
)

func (s *FamilyService) RenameFamily(familyID, userID uint, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 64 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return "", ErrFamilyNameInvalid
	}
	err := s.familyRepo.Transaction(func(tx *repository.FamilyRepository) error {
		family, err := tx.LockFamily(familyID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFamilyRenameNotFound
			}
			return err
		}
		member, err := tx.GetMemberByFamilyAndUser(familyID, userID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFamilyRenameForbidden
		}
		if err != nil {
			return err
		}
		if member.Role != "admin" {
			return ErrFamilyRenameForbidden
		}
		if family.Name == name {
			return nil
		}
		return tx.Rename(familyID, name)
	})
	return name, err
}
