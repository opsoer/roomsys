package services

import (
	"rental-server/models"

	"gorm.io/gorm"
)

type RoomService struct {
	DB *gorm.DB
}

func NewRoomService(db *gorm.DB) *RoomService {
	return &RoomService{DB: db}
}

func (s *RoomService) GetByID(id uint) (*models.Room, error) {
	var room models.Room
	if err := s.DB.Preload("Media").First(&room, id).Error; err != nil {
		return nil, err
	}
	return &room, nil
}

func (s *RoomService) GetWithContract(id uint) (*models.Room, *models.RentalContract, error) {
	var room models.Room
	if err := s.DB.Preload("Media").First(&room, id).Error; err != nil {
		return nil, nil, err
	}

	var contract models.RentalContract
	err := s.DB.Where("room_id = ? AND status = ?", id, "active").
		Preload("Tenant").
		First(&contract).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return &room, nil, err
	}

	if contract.ID == 0 {
		return &room, nil, nil
	}
	return &room, &contract, nil
}

func (s *RoomService) List(buildingID uint, page, size int) ([]models.Room, int64, error) {
	var rooms []models.Room
	query := s.DB.Where("building_id = ?", buildingID)

	var total int64
	if err := query.Model(&models.Room{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Preload("Media").Order("room_number").Offset((page - 1) * size).Limit(size).Find(&rooms).Error
	return rooms, total, err
}

func (s *RoomService) Create(room *models.Room) error {
	return s.DB.Create(room).Error
}

func (s *RoomService) Update(id uint, updates map[string]interface{}) error {
	return s.DB.Model(&models.Room{}).Where("id = ?", id).Updates(updates).Error
}

func (s *RoomService) Delete(id uint) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("room_id = ?", id).Delete(&models.RoomMedia{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Room{}, id).Error
	})
}

func (s *RoomService) UpdateStatus(id uint, status string) error {
	return s.DB.Model(&models.Room{}).Where("id = ?", id).Update("status", status).Error
}

func (s *RoomService) GetMedia(id uint) (*models.RoomMedia, error) {
	var media models.RoomMedia
	err := s.DB.First(&media, id).Error
	return &media, err
}

func (s *RoomService) DeleteMedia(id uint) error {
	return s.DB.Delete(&models.RoomMedia{}, id).Error
}

func (s *RoomService) CreateMedia(media *models.RoomMedia) error {
	return s.DB.Create(media).Error
}

func (s *RoomService) GetActiveContract(roomID uint) (*models.RentalContract, error) {
	var contract models.RentalContract
	err := s.DB.Where("room_id = ? AND status = ?", roomID, "active").
		Preload("Tenant").
		First(&contract).Error
	if err != nil {
		return nil, err
	}
	return &contract, nil
}

func (s *RoomService) GetActiveContractPublic(roomID uint) (*models.RentalContract, error) {
	var contract models.RentalContract
	err := s.DB.Where("room_id = ? AND status = ?", roomID, "active").
		First(&contract).Error
	if err != nil {
		return nil, err
	}
	return &contract, nil
}

func (s *RoomService) CreateContract(contract *models.RentalContract) error {
	return s.DB.Create(contract).Error
}

func (s *RoomService) UpdateContract(id uint, updates map[string]interface{}) error {
	return s.DB.Model(&models.RentalContract{}).Where("id = ?", id).Updates(updates).Error
}

func (s *RoomService) EndContract(id uint) error {
	return s.DB.Model(&models.RentalContract{}).Where("id = ?", id).Update("status", "ended").Error
}

func (s *RoomService) CreateTenant(tenant *models.Tenant) error {
	return s.DB.Create(tenant).Error
}

func (s *RoomService) GetRoomsWithExpiringContract(buildingID uint, days int) ([]models.Room, error) {
	var rooms []models.Room
	err := s.DB.Where("building_id = ? AND status = ?", buildingID, "rented").
		Preload("Media").
		Find(&rooms).Error
	return rooms, err
}
