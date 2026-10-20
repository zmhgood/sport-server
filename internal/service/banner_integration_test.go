package service

import (
	"elderly-fitness/internal/model"
	"elderly-fitness/internal/repository"
	"errors"
	"strings"
	"testing"
)

func TestBannerConfigurationPersistenceAndIsolation(t *testing.T) {
	f := newFixture(t)
	tx := f.db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	if err := tx.Unscoped().Where("`key`=?", model.HomeBannersConfigKey).Delete(&model.SystemConfig{}).Error; err != nil {
		t.Fatal(err)
	}
	repo := repository.NewSystemRepository(tx)
	s := NewBannerService(repo, "https://assets.example.test")
	state, err := s.Get(true)
	if err != nil || state.Configured || len(state.Items) != 0 {
		t.Fatal("missing config must differ from disabled config")
	}
	if err := repo.SetConfig("banner_unrelated_qa", "keep"); err != nil {
		t.Fatal(err)
	}
	items := []model.Banner{{ID: 2, Title: "  家庭锻炼  ", ImageURL: "https://assets.example.test/uploads/images/2.jpg", Enabled: true}, {ID: 1, Title: "暂不展示", ImageURL: "https://assets.example.test/uploads/images/1.png", Enabled: false}}
	if err := s.Save(items); err != nil {
		t.Fatal(err)
	}
	admin, err := s.Get(false)
	if err != nil || len(admin.Items) != 2 || admin.Items[0].Title != "家庭锻炼" || admin.Items[0].ID != 2 {
		t.Fatal("admin config did not persist or order changed")
	}
	public, err := s.Get(true)
	if err != nil || !public.Configured || len(public.Items) != 1 || public.Items[0].ID != 2 {
		t.Fatal("disabled banner leaked")
	}
	items[0].Enabled = false
	if err := s.Save(items); err != nil {
		t.Fatal(err)
	}
	hidden, err := s.Get(true)
	if err != nil || !hidden.Configured || len(hidden.Items) != 0 {
		t.Fatal("all disabled must hide public carousel")
	}
	items[0].Enabled = true
	if err := s.Save(items); err != nil {
		t.Fatal(err)
	}
	generic := NewSystemService(repo)
	all, err := generic.GetAllConfigs()
	if err != nil || all["banner_unrelated_qa"] != "keep" {
		t.Fatal("unrelated config changed")
	}
	if _, ok := all[model.HomeBannersConfigKey]; ok {
		t.Fatal("generic public config leaked disabled banners")
	}
	if err := generic.SetConfig(model.HomeBannersConfigKey, "[]"); !errors.Is(err, ErrReservedConfigKey) {
		t.Fatal("generic config bypass allowed")
	}
	bad := append([]model.Banner{}, items...)
	bad[1].Title = strings.Repeat("家", 21)
	if !errors.Is(s.Save(bad), ErrInvalidBanners) {
		t.Fatal("invalid batch accepted")
	}
	after, err := s.Get(false)
	if err != nil || len(after.Items) != 2 || after.Items[1].Title != "暂不展示" {
		t.Fatal("failed save partially changed list")
	}
	items[0].Title = "' OR 1=1 --"
	items[1].Enabled = true
	if err := s.Save(items); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Get(false); err != nil || got.Items[0].Title != "' OR 1=1 --" {
		t.Fatal("SQL literal changed")
	}
	if err := s.Save([]model.Banner{}); err != nil {
		t.Fatal(err)
	}
	public, err = s.Get(true)
	if err != nil || !public.Configured || len(public.Items) != 0 {
		t.Fatal("explicit empty config must hide banners")
	}
	var n int64
	tx.Model(&model.SystemConfig{}).Where("`key`=?", model.HomeBannersConfigKey).Count(&n)
	if n != 1 {
		t.Fatal("duplicate config rows")
	}
	row, err := repo.GetConfig(model.HomeBannersConfigKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Delete(row).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.Save(items); err != nil {
		t.Fatal("soft-deleted config failed to restore", err)
	}
	if _, err := s.Get(false); err != nil {
		t.Fatal(err)
	}
	if err := tx.Model(&model.SystemConfig{}).Where("`key`=?", model.HomeBannersConfigKey).Update("value", "broken JSON").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(true); err == nil {
		t.Fatal("corrupt data silently presented as valid empty config")
	}
	if err := tx.Model(&model.SystemConfig{}).Where("`key`=?", model.HomeBannersConfigKey).Update("value", "null").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(true); err == nil {
		t.Fatal("null config silently treated as disabled")
	}
}
