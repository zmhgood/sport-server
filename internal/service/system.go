package service

import (
	"elderly-fitness/internal/repository"
)

type SystemService struct {
	systemRepo *repository.SystemRepository
}

func NewSystemService(systemRepo *repository.SystemRepository) *SystemService {
	return &SystemService{systemRepo: systemRepo}
}

// GetConfig 获取配置
func (s *SystemService) GetConfig(key string) (string, error) {
	config, err := s.systemRepo.GetConfig(key)
	if err != nil {
		return "", err
	}
	return config.Value, nil
}

// SetConfig 设置配置
func (s *SystemService) SetConfig(key, value string) error {
	return s.systemRepo.SetConfig(key, value)
}

// GetAllConfigs 获取所有配置
func (s *SystemService) GetAllConfigs() (map[string]string, error) {
	configs, err := s.systemRepo.GetAllConfigs()
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, config := range configs {
		result[config.Key] = config.Value
	}
	return result, nil
}

// 预定义配置键
const (
	ConfigLogoURL    = "logo_url"
	ConfigSiteName   = "site_name"
	ConfigSiteSlogan = "site_slogan"
)
