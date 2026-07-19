// 房间服务，提供房间及合同的增删改查、租客管理
package services

import (
	"rental-server/models"

	"gorm.io/gorm"
)

// RoomService 房间服务
type RoomService struct {
	DB *gorm.DB
}

// NewRoomService 创建房间服务实例
func NewRoomService(db *gorm.DB) *RoomService {
	return &RoomService{DB: db}
}

// GetByID 根据ID获取房间（预加载媒体信息）
func (s *RoomService) GetByID(id uint) (*models.Room, error) {
	var room models.Room
	if err := s.DB.Preload("Media").First(&room, id).Error; err != nil {
		return nil, err
	}
	return &room, nil
}

// GetWithContract 获取房间及当前活跃合同信息
func (s *RoomService) GetWithContract(id uint) (*models.Room, *models.RentalContract, error) {
	var room models.Room
	if err := s.DB.Preload("Media").First(&room, id).Error; err != nil {
		return nil, nil, err
	}

	var contract models.RentalContract
	err := s.DB.Where("room_id = ? AND status IN ?", id, []string{"active", "reserved"}).
		Preload("Tenant").
		Order("CASE status WHEN 'active' THEN 0 ELSE 1 END").
		First(&contract).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return &room, nil, err
	}

	if contract.ID == 0 {
		return &room, nil, nil
	}
	return &room, &contract, nil
}

// List 分页获取楼栋下的房间列表，支持楼层和户型筛选
func (s *RoomService) List(buildingID uint, page, size int, floor, layout string) ([]models.Room, int64, error) {
	var rooms []models.Room
	query := s.DB.Where("building_id = ?", buildingID)

	if floor != "" {
		query = query.Where("floor = ?", floor)
	}
	if layout != "" {
		query = query.Where("layout = ?", layout)
	}

	var total int64
	if err := query.Model(&models.Room{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Preload("Media").Order("room_number").Offset((page - 1) * size).Limit(size).Find(&rooms).Error
	return rooms, total, err
}

// Create 创建房间
func (s *RoomService) Create(room *models.Room) error {
	return s.DB.Create(room).Error
}

// Update 更新房间信息
func (s *RoomService) Update(id uint, updates map[string]interface{}) error {
	return s.DB.Model(&models.Room{}).Where("id = ?", id).Updates(updates).Error
}

// Delete 删除房间及关联的媒体资源
func (s *RoomService) Delete(id uint) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("room_id = ?", id).Delete(&models.RoomMedia{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Room{}, id).Error
	})
}

// UpdateStatus 更新房间状态（vacant/rented）
func (s *RoomService) UpdateStatus(id uint, status string) error {
	return s.DB.Model(&models.Room{}).Where("id = ?", id).Update("status", status).Error
}

// GetMedia 根据ID获取房间媒体资源
func (s *RoomService) GetMedia(id uint) (*models.RoomMedia, error) {
	var media models.RoomMedia
	err := s.DB.First(&media, id).Error
	return &media, err
}

// DeleteMedia 删除房间媒体资源
func (s *RoomService) DeleteMedia(id uint) error {
	return s.DB.Delete(&models.RoomMedia{}, id).Error
}

// CreateMedia 创建房间媒体资源
func (s *RoomService) CreateMedia(media *models.RoomMedia) error {
	return s.DB.Create(media).Error
}

// GetActiveContract 获取房间的活跃或已预订合同（含租客信息），活跃优先
func (s *RoomService) GetActiveContract(roomID uint) (*models.RentalContract, error) {
	var contract models.RentalContract
	err := s.DB.Where("room_id = ? AND status IN ?", roomID, []string{"active", "reserved"}).
		Preload("Tenant").
		Order("CASE status WHEN 'active' THEN 0 ELSE 1 END").
		First(&contract).Error
	if err != nil {
		return nil, err
	}
	return &contract, nil
}

// GetReservedContract 获取房间的已预订（定金）合同（含租客信息）
func (s *RoomService) GetReservedContract(roomID uint) (*models.RentalContract, error) {
	var contract models.RentalContract
	err := s.DB.Where("room_id = ? AND status = ?", roomID, "reserved").
		Preload("Tenant").
		First(&contract).Error
	if err != nil {
		return nil, err
	}
	return &contract, nil
}

// GetActiveContractPublic 获取房间的活跃合同（公开，不含租客信息）
func (s *RoomService) GetActiveContractPublic(roomID uint) (*models.RentalContract, error) {
	var contract models.RentalContract
	err := s.DB.Where("room_id = ? AND status = ?", roomID, "active").
		First(&contract).Error
	if err != nil {
		return nil, err
	}
	return &contract, nil
}

// CreateContract 创建租赁合同
func (s *RoomService) CreateContract(contract *models.RentalContract) error {
	return s.DB.Create(contract).Error
}

// UpdateContract 更新租赁合同信息
func (s *RoomService) UpdateContract(id uint, updates map[string]interface{}) error {
	return s.DB.Model(&models.RentalContract{}).Where("id = ?", id).Updates(updates).Error
}

// EndContract 结束合同（标记为ended）
func (s *RoomService) EndContract(id uint) error {
	return s.DB.Model(&models.RentalContract{}).Where("id = ?", id).Update("status", "ended").Error
}

// CreateTenant 创建租客信息
func (s *RoomService) CreateTenant(tenant *models.Tenant) error {
	return s.DB.Create(tenant).Error
}

// GetRoomsWithExpiringContract 获取即将到期合同的房间列表
func (s *RoomService) GetRoomsWithExpiringContract(buildingID uint, days int) ([]models.Room, error) {
	var rooms []models.Room
	err := s.DB.Where("building_id = ? AND status = ?", buildingID, "rented").
		Preload("Media").
		Find(&rooms).Error
	return rooms, err
}

// GetFutureReservation 获取房间的未来的预定合同（房间有其他 active 合同的情况下）
func (s *RoomService) GetFutureReservation(roomID uint) (*models.RentalContract, error) {
	var contract models.RentalContract
	err := s.DB.Where("room_id = ? AND status = ?", roomID, "reserved").
		Preload("Tenant").
		First(&contract).Error
	if err != nil {
		return nil, err
	}
	return &contract, nil
}

// HasFutureReservation 检查房间是否有未来预定
func (s *RoomService) HasFutureReservation(roomID uint) bool {
	var count int64
	s.DB.Model(&models.RentalContract{}).
		Where("room_id = ? AND status = ?", roomID, "reserved").
		Count(&count)
	return count > 0
}
