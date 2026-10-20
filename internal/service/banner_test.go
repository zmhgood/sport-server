package service

import (
	"elderly-fitness/internal/model"
	"errors"
	"strings"
	"testing"
)

func TestBannerValidation(t *testing.T) {
	s := NewBannerService(nil, "https://assets.example.test")
	valid := model.Banner{ID: 1, Title: "家庭锻炼", Description: "一起动起来", ImageURL: "https://assets.example.test/uploads/images/1.jpg", Enabled: true}
	for _, tc := range []struct {
		name   string
		change func(*model.Banner)
	}{
		{"zero id", func(b *model.Banner) { b.ID = 0 }}, {"unsafe JS id", func(b *model.Banner) { b.ID = 9007199254740992 }},
		{"long title", func(b *model.Banner) { b.Title = strings.Repeat("家", 21) }}, {"long emoji title", func(b *model.Banner) { b.Title = strings.Repeat("😀", 21) }},
		{"long description", func(b *model.Banner) { b.Description = strings.Repeat("动", 41) }}, {"control", func(b *model.Banner) { b.Title = "家\n庭" }},
		{"external host", func(b *model.Banner) { b.ImageURL = "https://evil.test/uploads/images/1.jpg" }},
		{"suffix host", func(b *model.Banner) { b.ImageURL = "https://assets.example.test.evil.test/uploads/images/1.jpg" }},
		{"HTTP", func(b *model.Banner) { b.ImageURL = "http://assets.example.test/uploads/images/1.jpg" }},
		{"script", func(b *model.Banner) { b.ImageURL = "javascript:alert(1)" }},
		{"credentials", func(b *model.Banner) { b.ImageURL = "https://user@assets.example.test/uploads/images/1.jpg" }},
		{"query", func(b *model.Banner) { b.ImageURL += "?secret=unused" }}, {"fragment", func(b *model.Banner) { b.ImageURL += "#x" }},
		{"wrong path", func(b *model.Banner) { b.ImageURL = "https://assets.example.test/private/1.jpg" }},
		{"traversal", func(b *model.Banner) { b.ImageURL = "https://assets.example.test/uploads/images/%2e%2e/1.jpg" }},
		{"SVG", func(b *model.Banner) { b.ImageURL = "https://assets.example.test/uploads/images/1.svg" }},
		{"long URL", func(b *model.Banner) {
			b.ImageURL = "https://assets.example.test/uploads/images/" + strings.Repeat("a", 1024) + ".jpg"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := valid
			tc.change(&item)
			if !errors.Is(s.validate([]model.Banner{item}), ErrInvalidBanners) {
				t.Fatal("invalid banner accepted")
			}
		})
	}
	if !errors.Is(s.validate([]model.Banner{valid, valid}), ErrInvalidBanners) {
		t.Fatal("duplicate id accepted")
	}
	many := make([]model.Banner, 11)
	if !errors.Is(s.validate(many), ErrInvalidBanners) {
		t.Fatal("over-limit list accepted")
	}
	boundary := valid
	boundary.Title = strings.Repeat("😀", 20)
	boundary.Description = strings.Repeat("动", 40)
	if err := s.validate([]model.Banner{boundary}); err != nil {
		t.Fatal(err)
	}
	if err := s.validate([]model.Banner{}); err != nil {
		t.Fatal(err)
	}
}
