package services

import (
	"rental-server/config"
	"rental-server/models"

	"gorm.io/gorm"
)

type MediaService struct {
	DB  *gorm.DB
	Cfg *config.Config
}

func NewMediaService(db *gorm.DB, cfg *config.Config) *MediaService {
	return &MediaService{DB: db, Cfg: cfg}
}

func (s *MediaService) GetRoomByID(roomID uint, buildingID uint) (*models.Room, error) {
	var room models.Room
	err := s.DB.Where("id = ? AND building_id = ?", roomID, buildingID).First(&room).Error
	return &room, err
}

func (s *MediaService) GetBuildingByID(buildingID uint) (*models.Building, error) {
	var building models.Building
	err := s.DB.First(&building, buildingID).Error
	return &building, err
}

func (s *MediaService) CreateMedia(media *models.RoomMedia) error {
	return s.DB.Create(media).Error
}

func (s *MediaService) GetMediaByID(mediaID uint) (*models.RoomMedia, error) {
	var media models.RoomMedia
	err := s.DB.First(&media, mediaID).Error
	return &media, err
}

func (s *MediaService) GetMediaByRoomAndCategory(roomID uint, category string) ([]models.RoomMedia, error) {
	var medias []models.RoomMedia
	err := s.DB.Where("room_id = ? AND category = ?", roomID, category).Find(&medias).Error
	return medias, err
}

func (s *MediaService) DeleteMedia(media *models.RoomMedia) error {
	return s.DB.Delete(media).Error
}

func (s *MediaService) UpdateBuildingCover(buildingID uint, coverImage string) error {
	return s.DB.Model(&models.Building{}).Where("id = ?", buildingID).Update("cover_image", coverImage).Error
}
