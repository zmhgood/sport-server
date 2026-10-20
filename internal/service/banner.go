package service

import (
	"elderly-fitness/internal/model"
	"elderly-fitness/internal/repository"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

var ErrInvalidBanners = errors.New("轮播最多10张，标题最多20字，说明最多40字；图片须使用本站COS上传地址")

type BannerService struct {
	repo   *repository.SystemRepository
	bucket *url.URL
}

func NewBannerService(repo *repository.SystemRepository, bucketURL string) *BannerService {
	bucket, _ := url.Parse(strings.TrimRight(bucketURL, "/"))
	return &BannerService{repo: repo, bucket: bucket}
}

func bannerTextValid(value string, limit int) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) <= limit && strings.IndexFunc(value, unicode.IsControl) < 0
}

func (s *BannerService) validate(items []model.Banner) error {
	if len(items) > 10 {
		return ErrInvalidBanners
	}
	seen := map[uint64]bool{}
	for i := range items {
		item := &items[i]
		item.Title = strings.TrimSpace(item.Title)
		item.Description = strings.TrimSpace(item.Description)
		item.ImageURL = strings.TrimSpace(item.ImageURL)
		if item.ID == 0 || item.ID > 9007199254740991 || seen[item.ID] || !bannerTextValid(item.Title, 20) || !bannerTextValid(item.Description, 40) || len(item.ImageURL) > 1024 {
			return ErrInvalidBanners
		}
		seen[item.ID] = true
		u, err := url.Parse(item.ImageURL)
		if err != nil || s.bucket == nil || u.Scheme != "https" || u.Host == "" || !strings.EqualFold(u.Host, s.bucket.Host) || u.User != nil || u.RawQuery != "" || u.Fragment != "" || !strings.HasPrefix(u.Path, "/uploads/images/") || strings.Contains(u.Path, "..") {
			return ErrInvalidBanners
		}
		path := strings.ToLower(u.Path)
		valid := false
		for _, ext := range []string{".jpg", ".jpeg", ".png", ".gif", ".webp"} {
			if strings.HasSuffix(path, ext) {
				valid = true
			}
		}
		if !valid {
			return ErrInvalidBanners
		}
	}
	return nil
}

func (s *BannerService) Get(public bool) (*model.BannerConfig, error) {
	row, err := s.repo.GetConfig(model.HomeBannersConfigKey)
	result := &model.BannerConfig{Items: []model.Banner{}}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	var items []model.Banner
	if err = json.Unmarshal([]byte(row.Value), &items); err != nil {
		return nil, errors.New("轮播配置读取失败")
	}
	if items == nil {
		return nil, errors.New("轮播配置读取失败")
	}
	if err = s.validate(items); err != nil {
		return nil, err
	}
	result.Configured = true
	for _, item := range items {
		if !public || item.Enabled {
			result.Items = append(result.Items, item)
		}
	}
	return result, nil
}

func (s *BannerService) Save(items []model.Banner) error {
	if items == nil {
		return ErrInvalidBanners
	}
	if err := s.validate(items); err != nil {
		return err
	}
	value, err := json.Marshal(items)
	if err != nil {
		return err
	}
	return s.repo.SetBanners(string(value))
}
