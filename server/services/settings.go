package services

import (
	"rental-server/models"

	"gorm.io/gorm"
)

type SettingsService struct {
	DB *gorm.DB
}

func NewSettingsService(db *gorm.DB) *SettingsService {
	return &SettingsService{DB: db}
}

func (s *SettingsService) Get(key string) (*models.Setting, error) {
	var setting models.Setting
	err := s.DB.First(&setting, key).Error
	return &setting, err
}

func (s *SettingsService) Update(key, value string) error {
	setting := models.Setting{Key: key, Value: value}
	return s.DB.Save(&setting).Error
}


