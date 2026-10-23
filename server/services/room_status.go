// 房间状态机：出租、预订、取消预订、退租。
//
// 业务规则与事务边界都在这一层，handlers 只做「取参数 → 调服务 → 返回响应」。
// 错误一律用 utils.BizError 携带 HTTP 状态码与业务码，由 handler 用 utils.Fail 统一输出。
package services

import (
	"rental-server/models"
	"rental-server/utils"
)

// RoomStatusParams 房间状态变更参数（出租/预订/退租共用）。
type RoomStatusParams struct {
	// UserID 操作人（记账用）。
	UserID uint
	// TenantName 租客姓名。
	TenantName string
	// TenantPhone 租客电话。
	TenantPhone string
	// RentPrice 月租金。
	RentPrice float64
	// ManagementFee 月管理费（出租/预订必填）。
	ManagementFee float64
	// Deposit 押金。
	Deposit float64
	// EarnestMoney 预订定金。
	EarnestMoney float64
	// StartDate 起租日。
	StartDate string
	// EndDate 到期日。
	EndDate string
	// RefundedDeposit 实际退还金额（退租/取消预订必填，指针区分「未填」与「填 0」）。
	RefundedDeposit *float64
	// RecordDepositBill 是否记录押金账单（nil 表示按起租日自动判断）。
	RecordDepositBill *bool
}

// RentRoom 出租：结束旧合同、创建/激活合同、生成账单、置房间为 rented。
// 若房间已有预订合同，则视为「确认签约」：预订合同转 active 并按新参数更新。
// 注意：
//   - 被结束的旧 active 合同直接置 ended，不生成押金退还账单（押金结算走 VacateRoom）；
//   - 预订合同若已预收（Prepaid），确认签约时不重复生成租金/押金账单；
//   - 事务内顺带清理该房间的预订超时(reserved_overdue)待办（软删除，Task 带 DeletedAt）。
func (s *RoomService) RentRoom(room *models.Room, p RoomStatusParams) error {
	if p.ManagementFee <= 0 {
		return utils.BadRequest("请填写管理费")
	}

	var reservedContract *models.RentalContract
	if room.Status == "reserved" {
		if rc, err := s.GetReservedContract(room.ID); err == nil {
			reservedContract = rc
		}
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return utils.Internal(tx.Error, "服务器错误")
	}

	// 旧合同结束
	if err := tx.Model(&models.RentalContract{}).
		Where("room_id = ? AND status = ?", room.ID, "active").
		Update("status", "ended").Error; err != nil {
		tx.Rollback()
		return utils.Internal(err, "结束旧合同失败")
	}

	contract := models.RentalContract{
		RoomID:        room.ID,
		BuildingID:    room.BuildingID,
		RentPrice:     p.RentPrice,
		ManagementFee: p.ManagementFee,
		Deposit:       p.Deposit,
		StartDate:     p.StartDate,
		EndDate:       p.EndDate,
		Status:        "active",
	}

	if reservedContract != nil {
		contract.ID = reservedContract.ID
		contract.TenantID = reservedContract.TenantID
		updates := map[string]interface{}{
			"rent_price":     p.RentPrice,
			"management_fee": p.ManagementFee,
			"deposit":        p.Deposit,
			"start_date":     p.StartDate,
			"end_date":       p.EndDate,
			"status":         "active",
		}
		if p.TenantName != "" {
			if err := tx.Model(&models.Tenant{}).Where("id = ?", contract.TenantID).
				Updates(map[string]interface{}{"name": p.TenantName, "phone": p.TenantPhone}).Error; err != nil {
				tx.Rollback()
				return utils.Internal(err, "确认签约失败")
			}
		}
		if err := tx.Model(&models.RentalContract{}).Where("id = ?", contract.ID).Updates(updates).Error; err != nil {
			tx.Rollback()
			return utils.Internal(err, "确认签约失败")
		}
	} else {
		tenant := models.Tenant{Name: p.TenantName, Phone: p.TenantPhone}
		if err := tx.Create(&tenant).Error; err != nil {
			tx.Rollback()
			return utils.Internal(err, "创建失败")
		}
		contract.TenantID = tenant.ID
		if err := tx.Create(&contract).Error; err != nil {
			tx.Rollback()
			return utils.Internal(err, "创建失败")
		}
	}

	// 已预收款项的预订合同确认签约时不再重复建账单
	isPrepaid := reservedContract != nil && reservedContract.Prepaid
	if !isPrepaid {
		if err := CreateRentedBills(tx, room.BuildingID, room.ID, p.UserID, RentBillsParams{
			RentPrice:         p.RentPrice,
			ManagementFee:     p.ManagementFee,
			Deposit:           p.Deposit,
			StartDate:         p.StartDate,
			EndDate:           p.EndDate,
			RecordDepositBill: p.RecordDepositBill,
		}); err != nil {
			tx.Rollback()
			return utils.Internal(err, "创建出租账单失败")
		}
	}

	// 清理预订超时待办
	if err := tx.Where("room_id = ? AND type = ? AND status = ?", room.ID, "reserved_overdue", "pending").
		Delete(&models.Task{}).Error; err != nil {
		tx.Rollback()
		return utils.Internal(err, "状态更新失败")
	}
	if err := tx.Model(&models.Room{}).Where("id = ?", room.ID).Update("status", "rented").Error; err != nil {
		tx.Rollback()
		return utils.Internal(err, "状态更新失败")
	}
	if err := tx.Commit().Error; err != nil {
		return utils.Internal(err, "状态更新失败")
	}
	return nil
}

// ReserveRoom 预订：空房直接置 reserved；已出租房创建「未来预订」合同（房间状态不变）。
func (s *RoomService) ReserveRoom(room *models.Room, p RoomStatusParams) error {
	if room.Status == "reserved" {
		return utils.BadRequest("该房间已被预订")
	}
	if p.TenantName == "" {
		return utils.BadRequest("请填写租客姓名")
	}
	if p.EarnestMoney <= 0 {
		return utils.BadRequest("定金金额必须大于0")
	}
	if p.ManagementFee <= 0 {
		return utils.BadRequest("请填写管理费")
	}

	isFutureReservation := room.Status == "rented" || room.Status == "expiring" || room.Status == "expired"
	if isFutureReservation {
		if s.HasFutureReservation(room.ID) {
			return utils.BadRequest("该房间已有未来预订记录")
		}
		currentContract, err := s.GetActiveContract(room.ID)
		if err != nil {
			return utils.BadRequest("该房间无活跃合同")
		}
		if p.StartDate != "" && currentContract.EndDate != "" && p.StartDate < currentContract.EndDate {
			// 起租日必须不早于当前合同的到期日（字符串 YYYY-MM-DD 字典序即时间序；
			// 任一方为空则不拦截）
			return utils.BadRequest("未来预定的起租日期不能早于当前合同的结束日期")
		}
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return utils.Internal(tx.Error, "服务器错误")
	}

	tenant := models.Tenant{Name: p.TenantName, Phone: p.TenantPhone}
	if err := tx.Create(&tenant).Error; err != nil {
		tx.Rollback()
		return utils.Internal(err, "预订失败")
	}

	contract := models.RentalContract{
		RoomID:        room.ID,
		BuildingID:    room.BuildingID,
		TenantID:      tenant.ID,
		RentPrice:     p.RentPrice,
		ManagementFee: p.ManagementFee,
		Deposit:       p.Deposit,
		EarnestMoney:  p.EarnestMoney,
		StartDate:     p.StartDate,
		EndDate:       p.EndDate,
		Status:        "reserved",
	}
	if err := tx.Create(&contract).Error; err != nil {
		tx.Rollback()
		return utils.Internal(err, "预订失败")
	}

	if !isFutureReservation {
		if err := tx.Model(&models.Room{}).Where("id = ?", room.ID).Update("status", "reserved").Error; err != nil {
			tx.Rollback()
			return utils.Internal(err, "预订失败")
		}
	}
	if err := tx.Commit().Error; err != nil {
		return utils.Internal(err, "预订失败")
	}
	return nil
}

// CancelReservation 取消预订：按定金差额记违约收支、合同置 cancelled、房间回 vacant。
// 前提：房间当前状态为 reserved（由 handler 保证，service 未二次校验）。
// 不要对「已出租但存在未来预订合同」的房间调用，否则会把房间错误置回 vacant；
// 该场景退租后清理预订请走 VacateRoom。
func (s *RoomService) CancelReservation(room *models.Room, p RoomStatusParams) error {
	contract, err := s.GetReservedContract(room.ID)
	if err != nil {
		return utils.NotFound("无预订记录")
	}
	if p.RefundedDeposit == nil {
		return utils.BadRequest("请填写实际退还给租客的定金金额（如定金不退请填0，房东违约赔付请填超过定金的金额）")
	}
	refunded := *p.RefundedDeposit
	if refunded < 0 {
		return utils.BadRequest("退还金额不能为负数")
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return utils.Internal(tx.Error, "服务器错误")
	}

	if err := CreateEarnestBreachBill(tx, room.BuildingID, room.ID, p.UserID, contract.EarnestMoney, refunded); err != nil {
		tx.Rollback()
		return utils.Internal(err, "创建违约账单失败")
	}
	if err := tx.Model(&models.RentalContract{}).Where("id = ?", contract.ID).
		Update("status", "cancelled").Error; err != nil {
		tx.Rollback()
		return utils.Internal(err, "取消预订失败")
	}
	if err := tx.Where("room_id = ? AND type = ? AND status = ?", room.ID, "reserved_overdue", "pending").
		Delete(&models.Task{}).Error; err != nil {
		tx.Rollback()
		return utils.Internal(err, "取消预订失败")
	}
	if err := tx.Model(&models.Room{}).Where("id = ?", room.ID).Update("status", "vacant").Error; err != nil {
		tx.Rollback()
		return utils.Internal(err, "取消预订失败")
	}
	if err := tx.Commit().Error; err != nil {
		return utils.Internal(err, "取消预订失败")
	}
	return nil
}

// VacateRoom 退租：结束合同、生成押金退还账单、房间置空（或 reserved，若存在未来预订）。
// 返回值 hasFutureReservation 表示房间是否因存在未来预订而保持 reserved。
func (s *RoomService) VacateRoom(room *models.Room, p RoomStatusParams) (hasFutureReservation bool, err error) {
	var activeContract int64
	s.DB.Model(&models.RentalContract{}).
		Where("room_id = ? AND status = ?", room.ID, "active").
		Count(&activeContract)
	if activeContract == 0 {
		return false, utils.BadRequest("该房间当前没有生效中的合同，无法办理退租")
	}
	if p.RefundedDeposit == nil {
		return false, utils.BadRequest("请填写实际退还给租客的押金金额（如全额退还请填原押金，无需退还请填0）")
	}
	if *p.RefundedDeposit < 0 {
		return false, utils.BadRequest("退还押金金额不能为负数")
	}

	var originalDeposit float64
	s.DB.Model(&models.RentalContract{}).
		Where("room_id = ? AND status = ?", room.ID, "active").
		Select("deposit").Scan(&originalDeposit)

	tx := s.DB.Begin()
	if tx.Error != nil {
		return false, utils.Internal(tx.Error, "服务器错误")
	}

	if err := tx.Model(&models.RentalContract{}).
		Where("room_id = ? AND status = ?", room.ID, "active").
		Update("status", "ended").Error; err != nil {
		tx.Rollback()
		return false, utils.Internal(err, "结束合同失败")
	}

	if err := CreateDepositRefundBill(tx, room.BuildingID, room.ID, p.UserID, originalDeposit, *p.RefundedDeposit); err != nil {
		tx.Rollback()
		return false, utils.Internal(err, "创建退还账单失败")
	}

	// 未来预订检查在事务外读取，与事务内"置 ended"存在极小并发窗口：
	// 此刻若恰有新预订提交，本次置为 vacant/reserved 可能与并发侧的
	// HasFutureReservation 结果产生偏差，最终以各事务的提交顺序为准。
	hasFutureReservation = s.HasFutureReservation(room.ID)
	targetStatus := "vacant"
	if hasFutureReservation {
		targetStatus = "reserved"
	}
	if err := tx.Model(&models.Room{}).Where("id = ?", room.ID).Update("status", targetStatus).Error; err != nil {
		tx.Rollback()
		return false, utils.Internal(err, "状态更新失败")
	}
	// 房间已退租，同步完成定时任务生成的到期退租待办，避免待办计数虚高
	if err := tx.Model(&models.Task{}).
		Where("room_id = ? AND type = ? AND status = ?", room.ID, "expired_room", "pending").
		Update("status", "completed").Error; err != nil {
		tx.Rollback()
		return false, utils.Internal(err, "完成到期退租待办失败")
	}
	if err := tx.Commit().Error; err != nil {
		return false, utils.Internal(err, "退租事务提交失败")
	}
	return hasFutureReservation, nil
}
