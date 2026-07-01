package services

import (
	"rental-server/models"

	"gorm.io/gorm"
)

type TaskService struct {
	DB *gorm.DB
}

func NewTaskService(db *gorm.DB) *TaskService {
	return &TaskService{DB: db}
}

func (s *TaskService) List(buildingID uint, status string) ([]models.Task, error) {
	var tasks []models.Task
	query := s.DB.Where("building_id = ?", buildingID)

	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Preload("Room").Order("created_at DESC").Find(&tasks).Error
	return tasks, err
}

func (s *TaskService) GetByID(id uint) (*models.Task, error) {
	var task models.Task
	err := s.DB.Preload("Room").First(&task, id).Error
	return &task, err
}

func (s *TaskService) Create(task *models.Task) error {
	return s.DB.Create(task).Error
}

func (s *TaskService) Update(id uint, updates map[string]interface{}) error {
	return s.DB.Model(&models.Task{}).Where("id = ?", id).Updates(updates).Error
}

func (s *TaskService) Delete(id uint) error {
	return s.DB.Delete(&models.Task{}, id).Error
}

func (s *TaskService) Process(id uint, status string) error {
	return s.DB.Model(&models.Task{}).Where("id = ?", id).Update("status", status).Error
}

func (s *TaskService) Complete(id uint) error {
	return s.DB.Model(&models.Task{}).Where("id = ?", id).Update("status", "completed").Error
}
