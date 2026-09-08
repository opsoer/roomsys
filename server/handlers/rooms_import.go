// Package handlers 批量导入在租房间：创建房间的同时录入租客、合同与本月账单。
// 适用于接管已有租客的公寓（房源大多已在租），每间房的租客与租期各不相同，
// 因此按"共用房间信息 + 逐间租约明细"的方式提交，单间失败不影响其他房间。
// 租客按"姓名+手机号"自动复用（同批次内及系统中已有的），不会重复建档。
package handlers

import (
	"errors"
	"net/http"
	"time"

	"rental-server/logger"
	"rental-server/models"
	"rental-server/services"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ImportRentedRoomItem 导入在租房间的单间记录（房间基础信息 + 该房间的租约信息）
type ImportRentedRoomItem struct {
	RoomNumber           string   `json:"room_number" binding:"required"`
	Floor                string   `json:"floor" binding:"required"`
	Layout               string   `json:"layout" binding:"required"`
	Description          string   `json:"description"`
	RentPrice            *float64 `json:"rent_price" binding:"required"`
	DepositMonths        *uint    `json:"deposit_months" binding:"required"`
	ManagementFee        *float64 `json:"management_fee" binding:"required"`
	ElectricityUnitPrice *float64 `json:"electricity_unit_price" binding:"required"`
	WaterUnitPrice       *float64 `json:"water_unit_price" binding:"required"`
	TenantName           string   `json:"tenant_name" binding:"required"`
	TenantPhone          string   `json:"tenant_phone"`
	Deposit              float64  `json:"deposit" binding:"gte=0"`
	StartDate            string   `json:"start_date" binding:"required"`
	EndDate              string   `json:"end_date" binding:"required"`
	RecordDepositBill    bool     `json:"record_deposit_bill"` // 历史租约押金是以前收的，默认不补记
}

// ImportRentedReq 批量导入在租房间请求
type ImportRentedReq struct {
	Rooms []ImportRentedRoomItem `json:"rooms" binding:"required,dive"`
}

// ImportRented 批量导入在租房间：每间房独立事务，依次创建房间（状态=rented）、
// 租客、生效合同和本月租金账单（押金账单可选），并逐间返回成功/失败结果。
func (h *RoomHandler) ImportRented(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}

	var req ImportRentedReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	if len(req.Rooms) == 0 {
		utils.Error(c, http.StatusBadRequest, "请至少填写一条房间记录")
		return
	}
	if len(req.Rooms) > 200 {
		utils.Error(c, http.StatusBadRequest, "单次最多导入200间")
		return
	}

	userID, _ := c.Get("user_id")
	uid, _ := userID.(uint)

	type importFailure struct {
		RoomNumber string `json:"room_number"`
		Reason     string `json:"reason"`
	}
	successCount := 0
	failures := []importFailure{}
	tenantCache := map[string]uint{} // "姓名|手机号" -> 租客ID，避免同批导入重复建租客

	for _, item := range req.Rooms {
		if reason := validateImportRentedItem(item); reason != "" {
			failures = append(failures, importFailure{item.RoomNumber, reason})
			continue
		}
		if err := h.importOneRentedRoom(bid, uid, item, tenantCache); err != nil {
			logger.Log.Error().Err(err).Str("room_number", item.RoomNumber).Msg("导入在租房间失败")
			failures = append(failures, importFailure{item.RoomNumber, err.Error()})
			continue
		}
		successCount++
	}

	utils.Success(c, gin.H{"success_count": successCount, "failures": failures})
}

// validateImportRentedItem 校验单间导入记录，返回失败原因（空串表示通过）
func validateImportRentedItem(item ImportRentedRoomItem) string {
	if len(item.RoomNumber) > 20 {
		return "房间号不能超过20个字符"
	}
	if *item.RentPrice <= 0 {
		return "月租金必须大于0"
	}
	if *item.ManagementFee < 0 || *item.ElectricityUnitPrice < 0 || *item.WaterUnitPrice < 0 || item.Deposit < 0 {
		return "价格信息不能为负数"
	}
	if *item.DepositMonths > 3 {
		return "押金月数不能超过3"
	}
	if err := validateContractDates(item.StartDate, item.EndDate); err != nil {
		return err.Error()
	}
	// 导入的是"在租"房间，租约已整体过期的不允许录入
	end, _ := time.Parse("2006-01-02", item.EndDate)
	now := utils.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if end.Before(today) {
		return "租约到期日期不能早于今天，已到期的租约无需导入"
	}
	return ""
}

// resolveTenant 复用租客：同批次内或系统中已有"姓名+手机号"完全一致的租客时直接复用，
// 避免同一租客被重复建档；手机号为空或无匹配时新建。
func resolveTenant(tx *gorm.DB, cache map[string]uint, name, phone string) (uint, error) {
	key := name + "|" + phone
	if phone != "" {
		if id, ok := cache[key]; ok {
			return id, nil
		}
		var existing models.Tenant
		if err := tx.Where("name = ? AND phone = ?", name, phone).First(&existing).Error; err == nil {
			cache[key] = existing.ID
			return existing.ID, nil
		}
	}
	tenant := models.Tenant{Name: name, Phone: phone}
	if err := tx.Create(&tenant).Error; err != nil {
		return 0, errors.New("创建租客失败")
	}
	if phone != "" {
		cache[key] = tenant.ID
	}
	return tenant.ID, nil
}

// importOneRentedRoom 在单个事务内创建一间在租房间及其租客、合同、本月账单
func (h *RoomHandler) importOneRentedRoom(bid, uid uint, item ImportRentedRoomItem, tenantCache map[string]uint) error {
	tx := h.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	commit := false
	defer func() {
		if !commit {
			tx.Rollback()
		}
	}()

	room := models.Room{
		BuildingID:           bid,
		RoomNumber:           item.RoomNumber,
		Floor:                item.Floor,
		Layout:               item.Layout,
		Description:          item.Description,
		Status:               "rented",
		RentPrice:            item.RentPrice,
		DepositMonths:        item.DepositMonths,
		ManagementFee:        item.ManagementFee,
		ElectricityUnitPrice: item.ElectricityUnitPrice,
		WaterUnitPrice:       item.WaterUnitPrice,
	}
	// 复用 RoomService.Create，获得与普通创建一致的房间号查重
	if err := services.NewRoomService(tx).Create(&room); err != nil {
		return err
	}

	tenantID, err := resolveTenant(tx, tenantCache, item.TenantName, item.TenantPhone)
	if err != nil {
		return err
	}

	contract := models.RentalContract{
		RoomID:        room.ID,
		BuildingID:    bid,
		TenantID:      tenantID,
		RentPrice:     *item.RentPrice,
		ManagementFee: *item.ManagementFee,
		Deposit:       item.Deposit,
		StartDate:     item.StartDate,
		EndDate:       item.EndDate,
		Status:        "active",
	}
	if err := tx.Create(&contract).Error; err != nil {
		return errors.New("创建合同失败")
	}

	recordDeposit := item.RecordDepositBill
	if err := createRentedBills(tx, bid, room.ID, uid, rentedBillsParams{
		RentPrice:         *item.RentPrice,
		ManagementFee:     *item.ManagementFee,
		Deposit:           item.Deposit,
		StartDate:         item.StartDate,
		EndDate:           item.EndDate,
		RecordDepositBill: &recordDeposit,
	}); err != nil {
		return errors.New("创建本月租金账单失败")
	}

	if err := tx.Commit().Error; err != nil {
		logger.Log.Error().Err(err).Str("room_number", item.RoomNumber).Msg("导入在租房间提交事务失败")
		return errors.New("保存数据失败")
	}
	commit = true
	return nil
}
