package services

import (
	"rental-server/models"

	"gorm.io/gorm"
)

type RecruitService struct {
	DB *gorm.DB
}

func NewRecruitService(db *gorm.DB) *RecruitService {
	return &RecruitService{DB: db}
}

func (s *RecruitService) Submit(submission *models.RecruitSubmission) error {
	return s.DB.Create(submission).Error
}

func (s *RecruitService) List() ([]models.RecruitSubmission, error) {
	var submissions []models.RecruitSubmission
	err := s.DB.Order("created_at DESC").Find(&submissions).Error
	return submissions, err
}

func (s *RecruitService) Process(id uint) error {
	return s.DB.Model(&models.RecruitSubmission{}).Where("id = ?", id).Update("status", "processed").Error
}

func (s *RecruitService) UnprocessedCount() (int64, error) {
	var count int64
	err := s.DB.Model(&models.RecruitSubmission{}).Where("status = ?", "pending").Count(&count).Error
	return count, err
}
