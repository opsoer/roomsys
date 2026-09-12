// 任务服务，提供后台任务的增删改查与管理
package services

import (
	"rental-server/models"

	"gorm.io/gorm"
)

// TaskService 任务服务
type TaskService struct {
	DB *gorm.DB
}

// NewTaskService 创建任务服务实例
func NewTaskService(db *gorm.DB) *TaskService {
	return &TaskService{DB: db}
}

// List 分页查询任务列表（支持按状态筛选）
// 双模式分页：lastID > 0 时走游标分页（按 created_at + id 定位下一批），不再重查总数（total 返回 -1）
func (s *TaskService) List(buildingID uint, status string, page, lastID, size int, lastKey string) ([]models.Task, int64, error) {
	var tasks []models.Task
	query := s.DB.Where("building_id = ? AND scope = ?", buildingID, "building")

	if status != "" {
		query = query.Where("status = ?", status)
	}

	if lastID > 0 {
		query = query.Where("(created_at < ? OR (created_at = ? AND id < ?))", lastKey, lastKey, lastID)
	}

	var total int64
	if lastID == 0 {
		if err := query.Model(&models.Task{}).Count(&total).Error; err != nil {
			return nil, 0, err
		}
	} else {
		total = -1
	}

	q := query.Preload("Room").Order("created_at DESC, id DESC")
	if lastID > 0 {
		err := q.Limit(size).Find(&tasks).Error
		return tasks, total, err
	}
	err := q.Offset((page - 1) * size).Limit(size).Find(&tasks).Error
	return tasks, total, err
}

// GetByID 根据ID获取任务详情
func (s *TaskService) GetByID(id uint) (*models.Task, error) {
	var task models.Task
	err := s.DB.Preload("Room").First(&task, id).Error
	return &task, err
}

// Create 创建任务
func (s *TaskService) Create(task *models.Task) error {
	return s.DB.Create(task).Error
}

// Delete 删除任务
func (s *TaskService) Delete(id uint) error {
	return s.DB.Delete(&models.Task{}, id).Error
}

// Complete 将任务标记为已完成
func (s *TaskService) Complete(id uint) error {
	return s.DB.Model(&models.Task{}).Where("id = ?", id).Update("status", "completed").Error
}
