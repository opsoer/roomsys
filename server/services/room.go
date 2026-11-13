// 房间服务，提供房间及合同的增删改查、租客管理
package services

import (
	"errors"

	"rental-server/database"
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

	// 状态筛选下沉到 SQL，与 DynamicRoomStatus 的动态判定对齐，保证 total 与分页结果一致。
	// 与动态判定存在两处已知偏差（有意保留）：
	//   - "reserved" 同时命中「已出租但存在未来预订合同」的房间（这类房间动态状态
	//     仍是 rented/expiring，未来预订也算一种可预订来源）；
	//   - switch 无 default：传入未支持的状态值（如 "expired"）时不加任何过滤。
	// "expiring" 边界与 DynamicRoomStatus 一致：到期日 < today+10（恰好10天整不算）。
	if status != "" {
		today := utils.Now().Format("2006-01-02")
		expiringCutoff := utils.Now().AddDate(0, 0, utils.ExpiringSoonDays).Format("2006-01-02")
		switch status {
		case "vacant":
			query = query.Where("status = ?", "vacant")
		case "reserved":
			query = query.Where("status = ? OR id IN (SELECT room_id FROM rental_contracts WHERE status = ?)", "reserved", "reserved")
		case "expiring":
			query = query.Where("status = ? AND id IN (SELECT room_id FROM rental_contracts WHERE status = ? AND end_date != '' AND end_date >= ? AND end_date < ?)",
				"rented", "active", today, expiringCutoff)
		case "rented":
			query = query.Where("status = ? AND id NOT IN (SELECT room_id FROM rental_contracts WHERE status = ? AND end_date != '' AND end_date < ?)",
				"rented", "active", expiringCutoff)
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

// ErrRoomNumberExists 房间号在本公寓内已存在。
var ErrRoomNumberExists = errors.New("房间号已存在")

// Create 创建房间
func (s *RoomService) Create(room *models.Room) error {
	var count int64
	s.DB.Model(&models.Room{}).
		Where("building_id = ? AND room_number = ?", room.BuildingID, room.RoomNumber).
		Count(&count)
	if count > 0 {
		return ErrRoomNumberExists
	}
	if err := s.DB.Create(room).Error; err != nil {
		return err
	}
	utils.ClearStatsCache()
	return nil
}

// Update 更新房间信息
func (s *RoomService) Update(id uint, updates map[string]interface{}) error {
	if err := s.DB.Model(&models.Room{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return err
	}
	utils.ClearStatsCache()
	return nil
}

// Delete 删除房间及关联的媒体资源、合同、租客和任务。
// 全部为软删除；租客仅删除未被其他房间的未删除合同引用的（子查询排除），
// 账单保留但 room_id 置空（软删不触发外键 SET NULL，这里显式处理）。
// 监控数据：删除该房间的 room_detail 明细，并按天扣减楼栋 stat_daily 的
// pv/detail_pv（平台行不动，平台总量只增不减）。注意 page_views 只保留 90 天，
// 更早的 room_detail 贡献无法从明细回算，楼栋历史 PV 会保留这部分。
func (s *RoomService) Delete(id uint) error {
	var room models.Room
	if err := s.DB.First(&room, id).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		if room.ID != 0 {
			// 监控数据：按天统计该房间的 room_detail 事件，扣减楼栋 stat_daily
			dayExpr := database.DayExpr(tx, "created_at")
			var dayCounts []struct {
				Day string
				Cnt int64
			}
			if err := tx.Model(&models.PageView{}).
				Select(dayExpr+" AS day, COUNT(*) AS cnt").
				Where("page_type = ? AND resource_id = ? AND building_id = ?", "room_detail", id, room.BuildingID).
				Group(dayExpr).
				Scan(&dayCounts).Error; err != nil {
				return err
			}
			for _, dc := range dayCounts {
				if err := tx.Model(&models.StatDaily{}).
					Where("day = ? AND building_id = ?", dc.Day, room.BuildingID).
					Updates(map[string]interface{}{
						"pv":        gorm.Expr("CASE WHEN pv >= ? THEN pv - ? ELSE 0 END", dc.Cnt, dc.Cnt),
						"detail_pv": gorm.Expr("CASE WHEN detail_pv >= ? THEN detail_pv - ? ELSE 0 END", dc.Cnt, dc.Cnt),
					}).Error; err != nil {
					return err
				}
			}
			if err := tx.Where("page_type = ? AND resource_id = ? AND building_id = ?", "room_detail", id, room.BuildingID).
				Delete(&models.PageView{}).Error; err != nil {
				return err
			}
			// 账单保留，但房间已删：显式置空 room_id（软删不触发外键 SET NULL）
			if err := tx.Model(&models.Bill{}).Where("room_id = ?", id).Update("room_id", nil).Error; err != nil {
				return err
			}
		}
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
	}); err != nil {
		return err
	}
	utils.ClearStatsCache()
	return nil
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

// UpdateContract 更新租赁合同信息
func (s *RoomService) UpdateContract(id uint, updates map[string]interface{}) error {
	return s.DB.Model(&models.RentalContract{}).Where("id = ?", id).Updates(updates).Error
}

// GetFutureReservation 获取房间的预订(reserved)合同（含租客信息），供「当前租客退租后
// 与预订租客续签」等场景使用。本函数不校验房间是否有 active 合同，是否共存由调用方保证。
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
