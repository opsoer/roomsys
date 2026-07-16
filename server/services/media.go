// 媒体服务，提供房间图片/视频等媒体资源管理
package services

import (
	"rental-server/config"
	"rental-server/models"

	"gorm.io/gorm"
)

// MediaService 媒体服务
type MediaService struct {
	DB  *gorm.DB
	Cfg *config.Config
}

// NewMediaService 创建媒体服务实例
func NewMediaService(db *gorm.DB, cfg *config.Config) *MediaService {
	return &MediaService{DB: db, Cfg: cfg}
}

// GetRoomByID 根据ID和楼栋ID获取房间
func (s *MediaService) GetRoomByID(roomID uint, buildingID uint) (*models.Room, error) {
	var room models.Room
	err := s.DB.Where("id = ? AND building_id = ?", roomID, buildingID).First(&room).Error
	return &room, err
}

// GetBuildingByID 根据ID获取楼栋
func (s *MediaService) GetBuildingByID(buildingID uint) (*models.Building, error) {
	var building models.Building
	err := s.DB.First(&building, buildingID).Error
	return &building, err
}

// CreateMedia 创建媒体资源记录
func (s *MediaService) CreateMedia(media *models.RoomMedia) error {
	return s.DB.Create(media).Error
}

// GetMediaByID 根据ID获取媒体资源
func (s *MediaService) GetMediaByID(mediaID uint) (*models.RoomMedia, error) {
	var media models.RoomMedia
	err := s.DB.First(&media, mediaID).Error
	return &media, err
}

// GetMediaByRoomAndCategory 根据房间和分类获取媒体列表
func (s *MediaService) GetMediaByRoomAndCategory(roomID uint, category string) ([]models.RoomMedia, error) {
	var medias []models.RoomMedia
	err := s.DB.Where("room_id = ? AND category = ?", roomID, category).Find(&medias).Error
	return medias, err
}

// DeleteMedia 删除媒体资源
func (s *MediaService) DeleteMedia(media *models.RoomMedia) error {
	return s.DB.Delete(media).Error
}

// CountMediaByRoomAndType 统计指定房间和类型的媒体数量
func (s *MediaService) CountMediaByRoomAndType(roomID uint, mediaType string) (int64, error) {
	var count int64
	err := s.DB.Model(&models.RoomMedia{}).Where("room_id = ? AND type = ?", roomID, mediaType).Count(&count).Error
	return count, err
}

// UpdateBuildingCover 更新楼栋封面图
func (s *MediaService) UpdateBuildingCover(buildingID uint, coverImage string) error {
	return s.DB.Model(&models.Building{}).Where("id = ?", buildingID).Update("cover_image", coverImage).Error
}
