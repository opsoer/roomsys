package services

import (
	"rental-server/config"
	"rental-server/models"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"

	"gorm.io/gorm"
)

type MediaService struct {
	DB  *gorm.DB
	Cfg *config.Config
}

func NewMediaService(db *gorm.DB, cfg *config.Config) *MediaService {
	return &MediaService{DB: db, Cfg: cfg}
}

func (s *MediaService) Upload(file *multipart.FileHeader, roomID uint, category string) (*models.RoomMedia, error) {
	ext := filepath.Ext(file.Filename)
	filename := time.Now().Format("20060102150405") + ext

	uploadDir := s.Cfg.UploadDir
	if uploadDir == "" {
		uploadDir = "./uploads"
	}

	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return nil, err
	}

	dst := filepath.Join(uploadDir, filename)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		return nil, err
	}

	mediaType := "image"
	if ext == ".mp4" || ext == ".mov" {
		mediaType = "video"
	}

	media := &models.RoomMedia{
		RoomID:   roomID,
		Type:     mediaType,
		Category: category,
		FilePath: filename,
		FileName: file.Filename,
		FileSize: file.Size,
	}

	if err := s.DB.Create(media).Error; err != nil {
		return nil, err
	}

	return media, nil
}

func (s *MediaService) Delete(id uint) error {
	var media models.RoomMedia
	if err := s.DB.First(&media, id).Error; err != nil {
		return err
	}

	uploadDir := s.Cfg.UploadDir
	if uploadDir == "" {
		uploadDir = "./uploads"
	}

	filePath := filepath.Join(uploadDir, media.FilePath)
	os.Remove(filePath)

	return s.DB.Delete(&media).Error
}

func (s *MediaService) GetByID(id uint) (*models.RoomMedia, error) {
	var media models.RoomMedia
	err := s.DB.First(&media, id).Error
	return &media, err
}
