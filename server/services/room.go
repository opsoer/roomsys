// 房间服务，提供房间及合同的增删改查、租客管理
package services

import (
	"errors"

	"rental-server/models"
	"rental-server/utils"

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

// List 分页获取楼栋下的房间列表，支持楼层、户型、状态筛选
// 双模式分页：lastID > 0 时走游标分页（按 room_number + id 定位下一批），不再重查总数（total 返回 -1）；
// 旧客户端未传 lastKey 时兜底为 id 游标（排序同步切换为 id ASC，保证不重复、不遗漏）
func (s *RoomService) List(buildingID uint, page, lastID, size int, floor, layout, status, lastKey string) ([]models.Room, int64, error) {
	var rooms []models.Room
	query := s.DB.Where("building_id = ?", buildingID)

	if floor != "" {
		query = query.Where("floor = ?", floor)
	}
	if layout != "" {
		query = query.Where("layout = ?", layout)
	}

	// 状态筛选下沉到 SQL，与 DynamicRoomStatus 的动态判定保持一致，
	// 保证 total 与分页结果一致（避免内存过滤后 total 虚高/偏小）
	if status != "" {
		today := utils.Now().Format("2006-01-02")
		thirtyDaysLater := utils.Now().AddDate(0, 0, 30).Format("2006-01-02")
		switch status {
		case "vacant":
			query = query.Where("status = ?", "vacant")
		case "reserved":
			query = query.Where("status = ? OR id IN (SELECT room_id FROM rental_contracts WHERE status = ?)", "reserved", "reserved")
		case "expiring":
			query = query.Where("status = ? AND id IN (SELECT room_id FROM rental_contracts WHERE status = ? AND end_date != '' AND end_date >= ? AND end_date < ?)",
				"rented", "active", today, thirtyDaysLater)
		case "rented":
			query = query.Where("status = ? AND id NOT IN (SELECT room_id FROM rental_contracts WHERE status = ? AND end_date != '' AND end_date < ?)",
				"rented", "active", thirtyDaysLater)
		}
	}

	idCursor := lastID > 0 && lastKey == ""
	if lastID > 0 {
		if lastKey != "" {
			query = query.Where("(room_number > ? OR (room_number = ? AND id > ?))", lastKey, lastKey, lastID)
		} else {
			query = query.Where("id > ?", lastID)
		}
	}

	var total int64
	if lastID == 0 {
		if err := query.Model(&models.Room{}).Count(&total).Error; err != nil {
			return nil, 0, err
		}
	} else {
		total = -1
	}

	q := query.Preload("Media")
	if idCursor {
		q = q.Order("id ASC")
	} else {
		q = q.Order("room_number ASC, id ASC")
	}
	if lastID > 0 {
		err := q.Limit(size).Find(&rooms).Error
		return rooms, total, err
	}
	err := q.Offset((page - 1) * size).Limit(size).Find(&rooms).Error
	return rooms, total, err
}

// Create 创建房间
func (s *RoomService) Create(room *models.Room) error {
	var count int64
	s.DB.Model(&models.Room{}).
		Where("building_id = ? AND room_number = ?", room.BuildingID, room.RoomNumber).
		Count(&count)
	if count > 0 {
		return errors.New("房间号已存在")
	}
	return s.DB.Create(room).Error
}

// Update 更新房间信息
func (s *RoomService) Update(id uint, updates map[string]interface{}) error {
	return s.DB.Model(&models.Room{}).Where("id = ?", id).Updates(updates).Error
}

// Delete 删除房间及关联的媒体资源、合同、租客和任务
func (s *RoomService) Delete(id uint) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("room_id = ?", id).Delete(&models.RoomMedia{}).Error; err != nil {
			return err
		}
		var tenantIDs []uint
		tx.Model(&models.RentalContract{}).Where("room_id = ?", id).Pluck("tenant_id", &tenantIDs)
		if err := tx.Where("room_id = ?", id).Delete(&models.RentalContract{}).Error; err != nil {
			return err
		}
		if len(tenantIDs) > 0 {
			if err := tx.Where("id IN ? AND id NOT IN (SELECT tenant_id FROM rental_contracts WHERE room_id <> ? AND deleted_at IS NULL)",
				tenantIDs, id).Delete(&models.Tenant{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("room_id = ?", id).Delete(&models.Task{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Room{}, id).Error
	})
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

// ListBuildingContracts 获取公寓全部合同（含租客与房间信息），支持房间/状态/关键词筛选，
// 从新到旧排列；分页与房间列表保持同一约定（page 或 last_id+last_key 游标，lastID=0 时返回 total）
func (s *RoomService) ListBuildingContracts(buildingID uint, page, lastID, size int, roomID uint, status, keyword, lastKey string) ([]models.RentalContract, int64, error) {
	var contracts []models.RentalContract
	query := s.DB.Where("building_id = ?", buildingID)

	if roomID > 0 {
		query = query.Where("room_id = ?", roomID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where(
			"(room_id IN (SELECT id FROM rooms WHERE deleted_at IS NULL AND room_number LIKE ?)"+
				" OR tenant_id IN (SELECT id FROM tenants WHERE deleted_at IS NULL AND (name LIKE ? OR phone LIKE ?)))",
			like, like, like,
		)
	}

	// 游标分页：start_date DESC, id DESC（start_date 为 YYYY-MM-DD 字符串，字典序即时间序）
	if lastID > 0 && lastKey != "" {
		query = query.Where("(start_date < ? OR (start_date = ? AND id < ?))", lastKey, lastKey, lastID)
	}

	var total int64 = -1
	if lastID == 0 {
		if err := query.Model(&models.RentalContract{}).Count(&total).Error; err != nil {
			return nil, 0, err
		}
	}

	q := query.Preload("Tenant").Preload("Room").Order("start_date DESC, id DESC")
	if lastID > 0 {
		err := q.Limit(size).Find(&contracts).Error
		return contracts, total, err
	}
	err := q.Offset((page - 1) * size).Limit(size).Find(&contracts).Error
	return contracts, total, err
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

// UpdateContract 更新租赁合同信息
func (s *RoomService) UpdateContract(id uint, updates map[string]interface{}) error {
	return s.DB.Model(&models.RentalContract{}).Where("id = ?", id).Updates(updates).Error
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
