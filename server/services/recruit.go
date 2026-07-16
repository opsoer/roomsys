// 招募服务，处理房东/租客招募提交的后端逻辑
package services

import (
	"rental-server/models"

	"gorm.io/gorm"
)

// RecruitService 招募服务
type RecruitService struct {
	DB *gorm.DB
}

// NewRecruitService 创建招募服务实例
func NewRecruitService(db *gorm.DB) *RecruitService {
	return &RecruitService{DB: db}
}

// Submit 提交招募申请
func (s *RecruitService) Submit(submission *models.RecruitSubmission) error {
	return s.DB.Create(submission).Error
}

// List 获取所有招募申请（按时间倒序）
func (s *RecruitService) List() ([]models.RecruitSubmission, error) {
	var submissions []models.RecruitSubmission
	err := s.DB.Order("created_at DESC").Find(&submissions).Error
	return submissions, err
}

// Process 将招募标记为已处理
func (s *RecruitService) Process(id uint) error {
	return s.DB.Model(&models.RecruitSubmission{}).Where("id = ?", id).Update("status", "processed").Error
}

// UnprocessedCount 获取未处理的招募申请数量
func (s *RecruitService) UnprocessedCount() (int64, error) {
	var count int64
	err := s.DB.Model(&models.RecruitSubmission{}).Where("status = ?", "pending").Count(&count).Error
	return count, err
}
