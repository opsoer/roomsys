package services

import (
	"rental-server/models"

	"gorm.io/gorm"
)

type BuildingService struct {
	DB *gorm.DB
}

func NewBuildingService(db *gorm.DB) *BuildingService {
	return &BuildingService{DB: db}
}

type BuildingWithStats struct {
	models.Building
	Landlords     []models.BuildingLandlord `json:"landlords"`
	RoomCount     int64                     `json:"room_count"`
	VacantCount   int64                     `json:"vacant_count"`
	RentedCount   int64                     `json:"rented_count"`
	ExpiringCount int64                     `json:"expiring_count"`
}

func (s *BuildingService) GetByID(id uint) (*models.Building, error) {
	var building models.Building
	if err := s.DB.First(&building, id).Error; err != nil {
		return nil, err
	}
	return &building, nil
}

func (s *BuildingService) GetWithStats(id uint) (*BuildingWithStats, error) {
	var building models.Building
	if err := s.DB.First(&building, id).Error; err != nil {
		return nil, err
	}

	var landlords []models.BuildingLandlord
	s.DB.Where("building_id = ?", id).Find(&landlords)

	var roomCount, vacantCount, rentedCount, expiringCount int64
	s.DB.Model(&models.Room{}).Where("building_id = ?", id).Count(&roomCount)
	s.DB.Model(&models.Room{}).Where("building_id = ? AND status = ?", id, "vacant").Count(&vacantCount)
	s.DB.Model(&models.Room{}).Where("building_id = ? AND status = ?", id, "rented").Count(&rentedCount)
	s.DB.Model(&models.Room{}).Where("building_id = ? AND status = ?", id, "expiring").Count(&expiringCount)

	return &BuildingWithStats{
		Building:      building,
		Landlords:     landlords,
		RoomCount:     roomCount,
		VacantCount:   vacantCount,
		RentedCount:   rentedCount,
		ExpiringCount: expiringCount,
	}, nil
}

func (s *BuildingService) List(status, keyword string) ([]BuildingWithStats, error) {
	var buildings []models.Building
	query := s.DB

	if status != "" {
		switch status {
		case "normal":
			query = query.Where("expired_at = '' OR expired_at IS NULL OR expired_at > ?", "2024-01-01")
		case "expiring":
			query = query.Where("expired_at != '' AND expired_at IS NOT NULL AND expired_at <= ? AND expired_at > ?", "2024-12-31", "2024-01-01")
		case "expired":
			query = query.Where("expired_at != '' AND expired_at IS NOT NULL AND expired_at <= ?", "2024-01-01")
		}
	}

	if keyword != "" {
		query = query.Where("name LIKE ? OR id IN (SELECT building_id FROM building_landlords WHERE phone LIKE ?)",
			"%"+keyword+"%", "%"+keyword+"%")
	}

	if err := query.Find(&buildings).Error; err != nil {
		return nil, err
	}

	var result []BuildingWithStats
	for _, b := range buildings {
		ws, err := s.GetWithStats(b.ID)
		if err != nil {
			continue
		}
		result = append(result, *ws)
	}

	return result, nil
}

func (s *BuildingService) Create(building *models.Building) error {
	return s.DB.Create(building).Error
}

func (s *BuildingService) Update(id uint, updates map[string]interface{}) error {
	return s.DB.Model(&models.Building{}).Where("id = ?", id).Updates(updates).Error
}

func (s *BuildingService) Delete(id uint) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("building_id = ?", id).Delete(&models.RoomMedia{}).Error; err != nil {
			return err
		}
		if err := tx.Where("building_id = ?", id).Delete(&models.Room{}).Error; err != nil {
			return err
		}
		if err := tx.Where("building_id = ?", id).Delete(&models.BuildingLandlord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("building_id = ?", id).Delete(&models.Bill{}).Error; err != nil {
			return err
		}
		if err := tx.Where("building_id = ?", id).Delete(&models.Tenant{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Building{}, id).Error
	})
}

func (s *BuildingService) UpgradePackage(id uint, packageType string) error {
	return s.DB.Model(&models.Building{}).Where("id = ?", id).Update("package", packageType).Error
}

func (s *BuildingService) GetLandlords(buildingID uint) ([]models.BuildingLandlord, error) {
	var landlords []models.BuildingLandlord
	err := s.DB.Where("building_id = ?", buildingID).Find(&landlords).Error
	return landlords, err
}

func (s *BuildingService) CreateLandlord(landlord *models.BuildingLandlord) error {
	return s.DB.Create(landlord).Error
}

func (s *BuildingService) DeleteLandlords(buildingID uint) error {
	return s.DB.Where("building_id = ?", buildingID).Delete(&models.BuildingLandlord{}).Error
}
