// 设置服务，提供系统配置的键值对存储与读写
package services

import (
	"rental-server/models"

	"gorm.io/gorm"
)

// SettingsService 设置服务
type SettingsService struct {
	DB *gorm.DB
}

// NewSettingsService 创建设置服务实例
func NewSettingsService(db *gorm.DB) *SettingsService {
	return &SettingsService{DB: db}
}

// Get 根据键获取设置值
func (s *SettingsService) Get(key string) (*models.Setting, error) {
	var setting models.Setting
	err := s.DB.First(&setting, key).Error
	return &setting, err
}

// Update 更新或创建设置（键不存在时插入）
func (s *SettingsService) Update(key, value string) error {
	setting := models.Setting{Key: key, Value: value}
	return s.DB.Save(&setting).Error
}


