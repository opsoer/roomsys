package services

import (
	"fmt"
	"regexp"
	"strconv"

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

	var rooms []models.Room
	s.DB.Where("building_id = ?", id).Find(&rooms)

	roomIDs := make([]uint, len(rooms))
	for i, r := range rooms {
		roomIDs[i] = r.ID
	}

	var contracts []models.RentalContract
	if len(roomIDs) > 0 {
		s.DB.Where("room_id IN ? AND status = ?", roomIDs, "active").Find(&contracts)
	}
	contractMap := make(map[uint]string)
	for _, ct := range contracts {
		contractMap[ct.RoomID] = ct.EndDate
	}

	stats := make(map[string]int64)
	for _, r := range rooms {
		dynStatus := utils.DynamicRoomStatus(r.Status, contractMap[r.ID])
		stats[dynStatus]++
	}

	return &BuildingWithStats{
		Building:      building,
		Landlords:     landlords,
		RoomCount:     int64(len(rooms)),
		VacantCount:   stats["vacant"],
		RentedCount:   stats["rented"],
		ExpiringCount: stats["expiring"],
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

	var allRooms []models.Room
	s.DB.Where("building_id IN ?", buildingIDs).Find(&allRooms)

	roomIDs := make([]uint, len(allRooms))
	for i, r := range allRooms {
		roomIDs[i] = r.ID
	}

	var allContracts []models.RentalContract
	if len(roomIDs) > 0 {
		s.DB.Where("room_id IN ? AND status = ?", roomIDs, "active").Find(&allContracts)
	}
	contractMap := make(map[uint]string)
	for _, ct := range allContracts {
		contractMap[ct.RoomID] = ct.EndDate
	}

	dynStatusMap := make(map[uint]map[string]int64)
	for _, r := range allRooms {
		if dynStatusMap[r.BuildingID] == nil {
			dynStatusMap[r.BuildingID] = make(map[string]int64)
		}
		dynStatus := utils.DynamicRoomStatus(r.Status, contractMap[r.ID])
		dynStatusMap[r.BuildingID][dynStatus]++
	}

	var allLandlords []models.BuildingLandlord
	s.DB.Where("building_id IN ?", buildingIDs).Find(&allLandlords)
	landlordMap := make(map[uint][]models.BuildingLandlord)
	for _, l := range allLandlords {
		landlordMap[l.BuildingID] = append(landlordMap[l.BuildingID], l)
	}

	result := make([]BuildingWithStats, len(buildings))
	for i, b := range buildings {
		ds := dynStatusMap[b.ID]
		result[i] = BuildingWithStats{
			Building:      b,
			Landlords:     landlordMap[b.ID],
			RoomCount:     ds["vacant"] + ds["rented"] + ds["expiring"] + ds["expired"],
			VacantCount:   ds["vacant"],
			RentedCount:   ds["rented"],
			ExpiringCount: ds["expiring"],
		}
	}

	return result, total, nil
}

func (s *BuildingService) Create(building *models.Building) error {
	return s.DB.Create(building).Error
}

func (s *BuildingService) ExistsByName(name string) (bool, error) {
	var count int64
	err := s.DB.Model(&models.Building{}).Where("name = ?", name).Count(&count).Error
	return count > 0, err
}

func (s *BuildingService) GenerateSuggestedName(baseName string) (string, error) {
	var names []string
	if err := s.DB.Model(&models.Building{}).
		Where("name = ? OR name LIKE ?", baseName, baseName+"%").
		Pluck("name", &names).Error; err != nil {
		return "", err
	}
	maxSuffix := 0
	re := regexp.MustCompile(`^` + regexp.QuoteMeta(baseName) + `(\d+)$`)
	for _, n := range names {
		matches := re.FindStringSubmatch(n)
		if len(matches) == 2 {
			if num, err := strconv.Atoi(matches[1]); err == nil && num > maxSuffix {
				maxSuffix = num
			}
		}
	}
	return fmt.Sprintf("%s%d", baseName, maxSuffix+1), nil
}

func (s *BuildingService) Update(id uint, updates map[string]interface{}) error {
	return s.DB.Model(&models.Building{}).Where("id = ?", id).Updates(updates).Error
}

func (s *BuildingService) HasActiveContracts(id uint) (bool, error) {
	var count int64
	err := s.DB.Model(&models.RentalContract{}).
		Where("building_id = ? AND status = ?", id, "active").
		Count(&count).Error
	return count > 0, err
}

func (s *BuildingService) Delete(id uint) error {
	has, err := s.HasActiveContracts(id)
	if err != nil {
		return err
	}
	if has {
		return fmt.Errorf("active_contracts_exist")
	}
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var roomIDs []uint
		tx.Model(&models.Room{}).Where("building_id = ?", id).Pluck("id", &roomIDs)
		if len(roomIDs) > 0 {
			if err := tx.Where("room_id IN ?", roomIDs).Delete(&models.RoomMedia{}).Error; err != nil {
				return err
			}
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
