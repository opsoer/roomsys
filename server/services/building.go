package services

import (
	"rental-server/models"
	"rental-server/utils"

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

	type roomStatusCount struct {
		Status string
		Count  int64
	}
	var statusCounts []roomStatusCount
	s.DB.Model(&models.Room{}).
		Select("status, count(*) as count").
		Where("building_id = ?", id).
		Group("status").
		Scan(&statusCounts)

	statusMap := make(map[string]int64)
	var roomCount int64
	for _, sc := range statusCounts {
		statusMap[sc.Status] = sc.Count
		roomCount += sc.Count
	}

	return &BuildingWithStats{
		Building:      building,
		Landlords:     landlords,
		RoomCount:     roomCount,
		VacantCount:   statusMap["vacant"],
		RentedCount:   statusMap["rented"],
		ExpiringCount: statusMap["expiring"],
	}, nil
}

func (s *BuildingService) List(status, keyword string, page, size int) ([]BuildingWithStats, int64, error) {
	var buildings []models.Building
	query := s.DB

	today := utils.Now().Format("2006-01-02")
	thirtyDaysLater := utils.Now().AddDate(0, 0, 30).Format("2006-01-02")

	if status != "" {
		switch status {
		case "normal":
			query = query.Where("expired_at = '' OR expired_at IS NULL OR expired_at > ?", thirtyDaysLater)
		case "expiring":
			query = query.Where("expired_at != '' AND expired_at IS NOT NULL AND expired_at <= ? AND expired_at >= ?", thirtyDaysLater, today)
		case "expired":
			query = query.Where("expired_at != '' AND expired_at IS NOT NULL AND expired_at < ?", today)
		}
	}

	if keyword != "" {
		query = query.Where("name LIKE ? OR id IN (SELECT building_id FROM building_landlords WHERE phone LIKE ?)",
			"%"+keyword+"%", "%"+keyword+"%")
	}

	var total int64
	if err := query.Model(&models.Building{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Offset((page - 1) * size).Limit(size).Find(&buildings).Error; err != nil {
		return nil, 0, err
	}

	if len(buildings) == 0 {
		return []BuildingWithStats{}, 0, nil
	}

	buildingIDs := make([]uint, len(buildings))
	for i, b := range buildings {
		buildingIDs[i] = b.ID
	}

	type roomStatusCount struct {
		BuildingID uint
		Status     string
		Count      int64
	}
	var statusCounts []roomStatusCount
	s.DB.Model(&models.Room{}).
		Select("building_id, status, count(*) as count").
		Where("building_id IN ?", buildingIDs).
		Group("building_id, status").
		Scan(&statusCounts)

	statusMap := make(map[uint]map[string]int64)
	for _, sc := range statusCounts {
		if statusMap[sc.BuildingID] == nil {
			statusMap[sc.BuildingID] = make(map[string]int64)
		}
		statusMap[sc.BuildingID][sc.Status] = sc.Count
	}

	var allLandlords []models.BuildingLandlord
	s.DB.Where("building_id IN ?", buildingIDs).Find(&allLandlords)
	landlordMap := make(map[uint][]models.BuildingLandlord)
	for _, l := range allLandlords {
		landlordMap[l.BuildingID] = append(landlordMap[l.BuildingID], l)
	}

	result := make([]BuildingWithStats, len(buildings))
	for i, b := range buildings {
		sc := statusMap[b.ID]
		result[i] = BuildingWithStats{
			Building:      b,
			Landlords:     landlordMap[b.ID],
			RoomCount:     sc["vacant"] + sc["rented"] + sc["expiring"] + sc["expired"],
			VacantCount:   sc["vacant"],
			RentedCount:   sc["rented"],
			ExpiringCount: sc["expiring"],
		}
	}

	return result, total, nil
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
		if err := tx.Where("building_id = ?", id).Delete(&models.RentalContract{}).Error; err != nil {
			return err
		}
		if err := tx.Where("building_id = ?", id).Delete(&models.Shareholder{}).Error; err != nil {
			return err
		}
		if err := tx.Where("building_id = ?", id).Delete(&models.Dividend{}).Error; err != nil {
			return err
		}
		if err := tx.Where("building_id = ?", id).Delete(&models.Task{}).Error; err != nil {
			return err
		}
		if err := tx.Where("building_id = ?", id).Delete(&models.User{}).Error; err != nil {
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
