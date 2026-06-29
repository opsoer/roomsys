package handlers

import (
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"rental-server/config"
	"rental-server/logger"
	"rental-server/models"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type RoomHandler struct {
	DB  *gorm.DB
	Cfg *config.Config
}

func (h *RoomHandler) hasAuth(c *gin.Context) bool {
	tokenStr := c.GetHeader("Authorization")
	if tokenStr == "" || !strings.HasPrefix(tokenStr, "Bearer ") {
		return false
	}
	tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")
	_, err := utils.ParseToken(tokenStr, h.Cfg.JWTSecret)
	return err == nil
}

type CreateRoomReq struct {
	RoomNumber  string `json:"room_number" binding:"required"`
	Floor       string `json:"floor" binding:"required"`
	Layout      string `json:"layout" binding:"required"`
	Description string `json:"description"`
}

type UpdateRoomReq struct {
	RoomNumber  string `json:"room_number"`
	Floor       string `json:"floor"`
	Layout      string `json:"layout"`
	Description string `json:"description"`
}

type UpdateRoomStatusReq struct {
	Status          string   `json:"status" binding:"required"`
	TenantName      string   `json:"tenant_name"`
	TenantPhone     string   `json:"tenant_phone"`
	RentPrice       float64  `json:"rent_price"`
	Deposit         float64  `json:"deposit"`
	EarnestMoney    float64  `json:"earnest_money"`
	StartDate       string   `json:"start_date"`
	EndDate         string   `json:"end_date"`
	RefundedDeposit *float64 `json:"refunded_deposit"`
}

type UpdateContractReq struct {
	EndDate   string  `json:"end_date" binding:"required"`
	RentPrice float64 `json:"rent_price"`
}

// 公开：获取房间详情
func (h *RoomHandler) GetPublic(c *gin.Context) {
	roomID := c.Param("rid")
	buildingID := c.Param("id")
	var room models.Room
	if err := h.DB.Preload("Media", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order asc")
	}).Where("id = ? AND building_id = ?", roomID, buildingID).First(&room).Error; err != nil {
		utils.Error(c, http.StatusNotFound, "房间不存在")
		return
	}
	type RoomDetail struct {
		models.Room
		CurrentContract *models.RentalContract `json:"current_contract,omitempty"`
		EndDate         string                  `json:"end_date"`
	}
	detail := RoomDetail{Room: room}
	if h.hasAuth(c) {
		var contract models.RentalContract
		if room.Status == "rented" || room.Status == "expiring" {
			h.DB.Where("room_id = ? AND status = ?", room.ID, "active").
				Preload("Tenant").
				First(&contract)
			if contract.ID != 0 {
				detail.CurrentContract = &contract
				detail.EndDate = contract.EndDate
			}
		}
	}
	utils.Success(c, gin.H{"room": detail})
}

// 公开：获取有效合同（未登录时隐藏金额字段）
func (h *RoomHandler) GetActiveContractPublic(c *gin.Context) {
	roomID := c.Param("rid")
	buildingID := c.Param("id")
	var contract models.RentalContract
	if err := h.DB.Where("room_id = ? AND status = ? AND building_id = ?", roomID, "active", buildingID).
		Preload("Tenant").Preload("Room").
		First(&contract).Error; err != nil {
		utils.Success(c, gin.H{"contract": nil})
		return
	}
	if !h.hasAuth(c) {
		utils.Success(c, gin.H{"contract": nil})
		return
	}
	utils.Success(c, gin.H{"contract": contract})
}

func (h *RoomHandler) Create(c *gin.Context) {
	buildingID, exists := c.Get("building_id")
	if !exists {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	bid, ok := buildingID.(uint)
	if !ok {
		utils.Error(c, http.StatusInternalServerError, "服务器错误")
		return
	}
	var req CreateRoomReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Warn().Uint("building_id", bid).Msg("创建房间请求参数错误")
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	room := models.Room{
		BuildingID:  bid,
		RoomNumber:  req.RoomNumber,
		Floor:       req.Floor,
		Layout:      req.Layout,
		Description: req.Description,
		Status:      "vacant",
	}
	if err := h.DB.Create(&room).Error; err != nil {
		logger.Log.Warn().Err(err).Uint("building_id", bid).Str("room_number", req.RoomNumber).Msg("创建房间失败: 房间号重复")
		utils.Error(c, http.StatusConflict, "房间号已存在")
		return
	}
	logger.Log.Info().Uint("room_id", room.ID).Uint("building_id", bid).Str("room_number", room.RoomNumber).Msg("房间创建成功")
	utils.Created(c, "创建成功", gin.H{"room": room})
}

// 管理员：获取该楼栋所有房间
func (h *RoomHandler) List(c *gin.Context) {
	buildingID, exists := c.Get("building_id")
	if !exists {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	bid, ok := buildingID.(uint)
	if !ok {
		utils.Error(c, http.StatusInternalServerError, "服务器错误")
		return
	}
	var rooms []models.Room
	query := h.DB.Preload("Media", func(db *gorm.DB) *gorm.DB {
		return db.Where("type = ?", "image").Order("FIELD(category,'cover','gallery'), sort_order asc")
	}).Where("building_id = ?", bid)
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if floor := c.Query("floor"); floor != "" {
		query = query.Where("floor = ?", floor)
	}
	if layout := c.Query("layout"); layout != "" {
		query = query.Where("layout = ?", layout)
	}
	if err := query.Find(&rooms).Error; err != nil {
		utils.Error(c, http.StatusInternalServerError, "查询失败")
		return
	}
	type RoomVO struct {
		ID          uint   `json:"id"`
		RoomNumber  string `json:"room_number"`
		Floor       string `json:"floor"`
		Layout      string `json:"layout"`
		Status      string `json:"status"`
		Description string `json:"description"`
		Thumbnail   string `json:"thumbnail"`
		EndDate     string `json:"end_date"`
	}
	result := make([]RoomVO, 0)
	for _, r := range rooms {
		vo := RoomVO{
			ID:          r.ID,
			RoomNumber:  r.RoomNumber,
			Floor:       r.Floor,
			Layout:      r.Layout,
			Status:      r.Status,
			Description: r.Description,
		}
		for _, m := range r.Media {
			if m.Category == "cover" {
				vo.Thumbnail = m.FilePath
				break
			}
		}
		if vo.Thumbnail == "" && len(r.Media) > 0 {
			vo.Thumbnail = r.Media[0].FilePath
		}
		var contract models.RentalContract
		if r.Status == "rented" || r.Status == "expiring" {
			h.DB.Where("room_id = ? AND status = ?", r.ID, "active").Select("end_date").First(&contract)
			vo.EndDate = contract.EndDate
		}
		result = append(result, vo)
	}
	utils.Success(c, gin.H{"rooms": result})
}

// 管理员：获取房间详情
func (h *RoomHandler) Get(c *gin.Context) {
	buildingID, exists := c.Get("building_id")
	if !exists {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	bid, ok := buildingID.(uint)
	if !ok {
		utils.Error(c, http.StatusInternalServerError, "服务器错误")
		return
	}
	roomID := c.Param("id")
	var room models.Room
	if err := h.DB.Preload("Media", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order asc")
	}).Where("id = ? AND building_id = ?", roomID, bid).First(&room).Error; err != nil {
		utils.Error(c, http.StatusNotFound, "房间不存在")
		return
	}
	type RoomDetail struct {
		models.Room
		CurrentContract *models.RentalContract `json:"current_contract,omitempty"`
		EndDate         string                  `json:"end_date"`
	}
	detail := RoomDetail{Room: room}
	var contract models.RentalContract
	if room.Status == "rented" || room.Status == "expiring" {
		h.DB.Where("room_id = ? AND status = ?", room.ID, "active").
			Preload("Tenant").
			First(&contract)
		if contract.ID != 0 {
			detail.CurrentContract = &contract
			detail.EndDate = contract.EndDate
		}
	}
	utils.Success(c, gin.H{"room": detail})
}

func (h *RoomHandler) Update(c *gin.Context) {
	buildingID, exists := c.Get("building_id")
	if !exists {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	bid, ok := buildingID.(uint)
	if !ok {
		utils.Error(c, http.StatusInternalServerError, "服务器错误")
		return
	}
	roomID := c.Param("id")
	var room models.Room
	if err := h.DB.Where("id = ? AND building_id = ?", roomID, bid).First(&room).Error; err != nil {
		logger.Log.Warn().Str("id", roomID).Uint("building_id", bid).Msg("更新房间失败: 房间不存在")
		utils.Error(c, http.StatusNotFound, "房间不存在")
		return
	}
	var req UpdateRoomReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Warn().Uint("room_id", room.ID).Msg("更新房间请求参数错误")
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	updates := map[string]interface{}{}
	if req.RoomNumber != "" {
		var dup models.Room
		if h.DB.Where("building_id = ? AND room_number = ? AND id != ?", bid, req.RoomNumber, roomID).First(&dup).Error == nil {
			logger.Log.Warn().Uint("room_id", room.ID).Str("room_number", req.RoomNumber).Msg("更新房间失败: 房间号重复")
			utils.Error(c, http.StatusConflict, "该公寓房间号已存在")
			return
		}
		updates["room_number"] = req.RoomNumber
	}
	if req.Floor != "" {
		updates["floor"] = req.Floor
	}
	if req.Layout != "" {
		updates["layout"] = req.Layout
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if err := h.DB.Model(&room).Updates(updates).Error; err != nil {
		logger.Log.Error().Err(err).Uint("room_id", room.ID).Msg("更新房间失败")
		utils.Error(c, http.StatusInternalServerError, "更新失败")
		return
	}
	logger.Log.Info().Uint("room_id", room.ID).Uint("building_id", bid).Msg("房间信息更新成功")
	utils.Success(c, gin.H{"room": room})
}

func (h *RoomHandler) Delete(c *gin.Context) {
	buildingID, exists := c.Get("building_id")
	if !exists {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	bid, ok := buildingID.(uint)
	if !ok {
		utils.Error(c, http.StatusInternalServerError, "服务器错误")
		return
	}
	roomID := c.Param("id")
	var room models.Room
	if err := h.DB.Where("id = ? AND building_id = ?", roomID, bid).First(&room).Error; err != nil {
		logger.Log.Warn().Str("id", roomID).Uint("building_id", bid).Msg("删除房间失败: 房间不存在")
		utils.Error(c, http.StatusNotFound, "房间不存在")
		return
	}
	if room.Status == "rented" || room.Status == "expiring" {
		logger.Log.Warn().Uint("room_id", room.ID).Str("status", room.Status).Msg("删除房间失败: 房间有活跃合同")
		utils.Error(c, http.StatusBadRequest, "房间有活跃合同，无法删除")
		return
	}
	if err := h.DB.Delete(&room).Error; err != nil {
		logger.Log.Error().Err(err).Uint("room_id", room.ID).Msg("删除房间失败")
		utils.Error(c, http.StatusInternalServerError, "删除失败")
		return
	}
	logger.Log.Info().Uint("room_id", room.ID).Str("room_number", room.RoomNumber).Uint("building_id", bid).Msg("房间已删除")
	utils.SuccessWithMsg(c, "删除成功", nil)
}

func (h *RoomHandler) UpdateStatus(c *gin.Context) {
	buildingID, exists := c.Get("building_id")
	if !exists {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	bid, ok := buildingID.(uint)
	if !ok {
		utils.Error(c, http.StatusInternalServerError, "服务器错误")
		return
	}
	roomID := c.Param("id")
	var room models.Room
	if err := h.DB.Where("id = ? AND building_id = ?", roomID, bid).First(&room).Error; err != nil {
		logger.Log.Warn().Str("id", roomID).Uint("building_id", bid).Msg("修改房间状态失败: 房间不存在")
		utils.Error(c, http.StatusNotFound, "房间不存在")
		return
	}
	var req UpdateRoomStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Warn().Uint("room_id", room.ID).Msg("修改房间状态请求参数错误")
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	switch req.Status {
	case "vacant", "rented":
	default:
		logger.Log.Warn().Uint("room_id", room.ID).Str("status", req.Status).Msg("修改房间状态失败: 无效的状态值")
		utils.Error(c, http.StatusBadRequest, "状态值无效")
		return
	}
	if req.Status == "rented" && req.TenantName == "" {
		logger.Log.Warn().Uint("room_id", room.ID).Msg("出租房间失败: 缺少租客姓名")
		utils.Error(c, http.StatusBadRequest, "出租需填写租客姓名")
		return
	}
	if req.Status == "rented" && room.Status == "expiring" {
		logger.Log.Warn().Uint("room_id", room.ID).Msg("出租房间失败: 即将退租的房间请先修改退租日期")
		utils.Error(c, http.StatusBadRequest, "即将退租的房间请先修改退租日期，不能直接设为已出租")
		return
	}
	if req.Status == "rented" && req.RentPrice <= 0 {
		logger.Log.Warn().Uint("room_id", room.ID).Msg("出租房间失败: 缺少租金金额")
		utils.Error(c, http.StatusBadRequest, "请填写租金金额")
		return
	}
	if req.Status == "rented" && req.StartDate != "" && req.EndDate != "" {
		start, err1 := time.Parse("2006-01-02", req.StartDate)
		end, err2 := time.Parse("2006-01-02", req.EndDate)
		if err1 == nil && err2 == nil && !end.After(start) {
			logger.Log.Warn().Uint("room_id", room.ID).Msg("出租房间失败: 退租日期必须大于起租日期")
			utils.Error(c, http.StatusBadRequest, "退租日期必须大于起租日期")
			return
		}
	}

	if req.Status == "rented" {
		var existingContract models.RentalContract
		if err := h.DB.Where("room_id = ? AND status = ?", room.ID, "active").First(&existingContract).Error; err == nil {
			logger.Log.Warn().Uint("room_id", room.ID).Msg("出租房间失败: 已有活跃合同")
			utils.Error(c, http.StatusBadRequest, "该房间已有活跃合同，请先退租")
			return
		}

		var tenant models.Tenant
		h.DB.Where("name = ? AND phone = ?", req.TenantName, req.TenantPhone).First(&tenant)
		if tenant.ID == 0 {
			tenant = models.Tenant{
				Name:  req.TenantName,
				Phone: req.TenantPhone,
			}
			h.DB.Create(&tenant)
			logger.Log.Info().Uint("tenant_id", tenant.ID).Str("name", tenant.Name).Msg("新租客信息已创建")
		}
		startDate := req.StartDate
		if startDate == "" {
			startDate = utils.Now().Format("2006-01-02")
		}
		endDate := req.EndDate
		if endDate == "" {
			endDate = utils.Now().AddDate(1, 0, 0).Format("2006-01-02")
		}
		contract := models.RentalContract{
			RoomID:       room.ID,
			BuildingID:   bid,
			TenantID:     tenant.ID,
			RentPrice:    req.RentPrice,
			Deposit:      req.Deposit,
			EarnestMoney: req.EarnestMoney,
			StartDate:    startDate,
			EndDate:      endDate,
			Status:       "active",
		}
		h.DB.Create(&contract)
		userID, exists := c.Get("user_id")
		if !exists {
			utils.Error(c, http.StatusUnauthorized, "未授权")
			return
		}
		uid, ok := userID.(uint)
		if !ok {
			utils.Error(c, http.StatusInternalServerError, "服务器错误")
			return
		}
		createProratedRentBill(h.DB, room, contract, uid, bid)
		newStatus := "rented"
		if startParsed, err := time.Parse("2006-01-02", startDate); err == nil && startParsed.After(utils.Now()) {
			newStatus = "vacant"
		}
		if endDateParsed, err := time.Parse("2006-01-02", endDate); err == nil {
			if utils.Until(endDateParsed) < 30*24*time.Hour {
				newStatus = "expiring"
			}
		}
		if err := h.DB.Model(&room).Update("status", newStatus).Error; err != nil {
			logger.Log.Error().Err(err).Uint("room_id", room.ID).Msg("更新房间状态失败")
		}
		logger.Log.Info().
			Uint("room_id", room.ID).
			Uint("building_id", bid).
			Uint("tenant_id", tenant.ID).
			Str("tenant_name", tenant.Name).
			Float64("rent_price", req.RentPrice).
			Float64("deposit", req.Deposit).
			Str("start_date", startDate).
			Str("end_date", endDate).
			Str("new_status", newStatus).
			Msg("房间出租成功")
		utils.SuccessWithMsg(c, "出租成功", gin.H{"status": newStatus})
		return
	} else if req.Status == "vacant" {
		tx := h.DB.Begin()
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()

		if req.RefundedDeposit != nil {
			userID, exists := c.Get("user_id")
			if !exists {
				tx.Rollback()
				utils.Error(c, http.StatusUnauthorized, "未授权")
				return
			}
			uid, ok := userID.(uint)
			if !ok {
				tx.Rollback()
				utils.Error(c, http.StatusInternalServerError, "服务器错误")
				return
			}
			handleDepositRefund(tx, room, *req.RefundedDeposit, uid, bid)
		}
		if err := tx.Model(&models.RentalContract{}).
			Where("room_id = ? AND status = ?", room.ID, "active").
			Updates(map[string]interface{}{
				"status":   "ended",
				"end_date": utils.Now().Format("2006-01-02"),
			}).Error; err != nil {
			tx.Rollback()
			logger.Log.Error().Err(err).Uint("room_id", room.ID).Msg("更新合同状态失败")
			utils.Error(c, http.StatusInternalServerError, "退租失败")
			return
		}
		if err := tx.Model(&room).Update("status", "vacant").Error; err != nil {
			tx.Rollback()
			logger.Log.Error().Err(err).Uint("room_id", room.ID).Msg("更新房间状态失败")
			utils.Error(c, http.StatusInternalServerError, "退租失败")
			return
		}
		tx.Commit()
		logger.Log.Info().
			Uint("room_id", room.ID).
			Uint("building_id", bid).
			Str("room_number", room.RoomNumber).
			Msg("房间退租成功，状态设为 vacant")
		utils.SuccessWithMsg(c, "退租成功", gin.H{"status": "vacant"})
		return
	}
	utils.SuccessWithMsg(c, "状态更新成功", gin.H{"status": req.Status})
}

func (h *RoomHandler) RenewContract(c *gin.Context) {
	buildingID, exists := c.Get("building_id")
	if !exists {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	bid, ok := buildingID.(uint)
	if !ok {
		utils.Error(c, http.StatusInternalServerError, "服务器错误")
		return
	}
	roomID := c.Param("id")
	var contract models.RentalContract
	if err := h.DB.Where("room_id = ? AND status = ? AND building_id = ?", roomID, "active", bid).First(&contract).Error; err != nil {
		logger.Log.Warn().Str("id", roomID).Uint("building_id", bid).Msg("续租失败: 无有效合同")
		utils.Error(c, http.StatusNotFound, "无有效合同")
		return
	}
	var req UpdateContractReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Warn().Uint("contract_id", contract.ID).Msg("续租请求参数错误")
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		logger.Log.Warn().Str("end_date", req.EndDate).Msg("续租失败: 日期格式错误")
		utils.Error(c, http.StatusBadRequest, "日期格式错误")
		return
	}
	startDate, err := time.Parse("2006-01-02", contract.StartDate)
	if err == nil && !endDate.After(startDate) {
		logger.Log.Warn().Uint("contract_id", contract.ID).Msg("续租失败: 结束日期必须大于起租日期")
		utils.Error(c, http.StatusBadRequest, "续租结束日期必须大于起租日期")
		return
	}

	tx := h.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	rentPrice := contract.RentPrice
	if req.RentPrice > 0 {
		rentPrice = req.RentPrice
	}

	if err := tx.Model(&contract).Update("status", "ended").Error; err != nil {
		tx.Rollback()
		logger.Log.Error().Err(err).Uint("contract_id", contract.ID).Msg("续租失败: 结束旧合同失败")
		utils.Error(c, http.StatusInternalServerError, "续租失败")
		return
	}

	newContract := models.RentalContract{
		RoomID:       contract.RoomID,
		BuildingID:   contract.BuildingID,
		TenantID:     contract.TenantID,
		RentPrice:    rentPrice,
		Deposit:      contract.Deposit,
		EarnestMoney: 0,
		RentDay:      contract.RentDay,
		PaymentCycle: contract.PaymentCycle,
		ContractFile: contract.ContractFile,
		StartDate:    contract.EndDate,
		EndDate:      req.EndDate,
		Status:       "active",
	}
	if err := tx.Create(&newContract).Error; err != nil {
		tx.Rollback()
		logger.Log.Error().Err(err).Uint("contract_id", contract.ID).Msg("续租失败: 创建新合同失败")
		utils.Error(c, http.StatusInternalServerError, "续租失败")
		return
	}

	if utils.Until(endDate) < 30*24*time.Hour {
		if err := tx.Model(&models.Room{}).Where("id = ?", roomID).Update("status", "expiring").Error; err != nil {
			tx.Rollback()
			logger.Log.Error().Err(err).Uint("room_id", contract.RoomID).Msg("更新房间状态失败")
		}
	} else {
		if err := tx.Model(&models.Room{}).Where("id = ? AND status = ?", roomID, "expiring").Update("status", "rented").Error; err != nil {
			tx.Rollback()
			logger.Log.Error().Err(err).Uint("room_id", contract.RoomID).Msg("更新房间状态失败")
		}
	}

	tx.Commit()

	logger.Log.Info().
		Uint("old_contract_id", contract.ID).
		Uint("new_contract_id", newContract.ID).
		Uint("room_id", contract.RoomID).
		Uint("building_id", bid).
		Str("new_end_date", req.EndDate).
		Float64("rent_price", rentPrice).
		Msg("合同续租成功")
	utils.SuccessWithMsg(c, "续租成功", nil)
}

// 创建按天计算的租金账单
func createProratedRentBill(db *gorm.DB, room models.Room, contract models.RentalContract, userID uint, buildingID uint) {
	startDate, err := time.Parse("2006-01-02", contract.StartDate)
	if err != nil {
		return
	}
	endDate, err := time.Parse("2006-01-02", contract.EndDate)
	if err != nil {
		return
	}
	if startDate.After(utils.Now()) {
		return
	}
	year, month, _ := startDate.Date()
	firstDay := time.Date(year, month, 1, 0, 0, 0, 0, startDate.Location())
	lastDay := firstDay.AddDate(0, 1, -1)
	billEnd := endDate
	if endDate.After(lastDay) {
		billEnd = lastDay
	}
	if startDate.After(billEnd) {
		return
	}
	daysInMonth := lastDay.Day()
	days := int(billEnd.Sub(startDate).Hours()/24) + 1
	amount := contract.RentPrice * float64(days) / float64(daysInMonth)
	amount = math.Round(amount*100) / 100
	monthStr := firstDay.Format("2006-01")
	desc := fmt.Sprintf("房间 %s %s 租金（%s至%s）合同ID:%d", room.RoomNumber, monthStr, startDate.Format("2006-01-02"), billEnd.Format("2006-01-02"), contract.ID)

	var existing models.Bill
	result := db.Where("building_id = ? AND room_id = ? AND subtype = ? AND DATE_FORMAT(bill_date, '%Y-%m') = ? AND description LIKE ?",
		buildingID, room.ID, "租金", monthStr, "%合同ID:"+fmt.Sprintf("%d", contract.ID)+"%").
		First(&existing)
	if result.Error == nil {
		oldAmount := existing.Amount
		newAmount := oldAmount + amount
		changeNote := fmt.Sprintf(" | 修改原因 新租约生效日期 %s,金额从 %.2f 变为 %.2f（新增%.2f）", contract.StartDate, oldAmount, newAmount, amount)
		newDesc := existing.Description + changeNote
		db.Model(&existing).Updates(map[string]interface{}{
			"amount":      newAmount,
			"description": newDesc,
		})
		return
	}

	billNo := utils.GenerateBillNo()
	bill := models.Bill{
		BillNo:      billNo,
		Type:        "income",
		Subtype:     "租金",
		Amount:      amount,
		BuildingID:  buildingID,
		RoomID:      &room.ID,
		Description: desc,
		BillDate:    startDate.Format("2006-01-02"),
		CreatedBy:   userID,
	}
	if err := db.Create(&bill).Error; err != nil {
		logger.Log.Error().Err(err).Uint("room_id", room.ID).Uint("building_id", buildingID).Msg("自动创建租金账单失败")
	}
}

// 处理押金退还
func handleDepositRefund(d *gorm.DB, room models.Room, refundedDeposit float64, userID uint, buildingID uint) {
	var contract models.RentalContract
	err := d.Where("room_id = ? AND status = ? AND building_id = ?", room.ID, "active", buildingID).First(&contract).Error
	if err != nil {
		err = d.Where("room_id = ? AND status = ? AND building_id = ?", room.ID, "ended", buildingID).Order("created_at desc").First(&contract).Error
	}
	if err != nil || contract.Deposit <= 0 {
		return
	}
	if refundedDeposit < 0 {
		refundedDeposit = 0
	}
	if refundedDeposit >= contract.Deposit {
		return
	}
	today := utils.Now().Format("2006-01-02")
	diff := contract.Deposit - refundedDeposit

	incomeBillNo := utils.GenerateBillNo()
	incomeBill := models.Bill{
		BillNo:      incomeBillNo,
		Type:        "income",
		Subtype:     "押金",
		Amount:      diff,
		BuildingID:  buildingID,
		RoomID:      &room.ID,
		Description: fmt.Sprintf("房间 %s %s 退租扣除押金（原押金%.2f元，退还%.2f元）", room.RoomNumber, today, contract.Deposit, refundedDeposit),
		BillDate:    today,
		CreatedBy:   userID,
	}
	if err := d.Create(&incomeBill).Error; err != nil {
		logger.Log.Error().Err(err).Uint("room_id", room.ID).Uint("building_id", buildingID).Msg("自动创建押金收入账单失败")
		return
	}

	if refundedDeposit > 0 {
		expenseBillNo := utils.GenerateBillNo()
		expenseBill := models.Bill{
			BillNo:      expenseBillNo,
			Type:        "expense",
			Subtype:     "押金退还",
			Amount:      refundedDeposit,
			BuildingID:  buildingID,
			RoomID:      &room.ID,
			Description: fmt.Sprintf("房间 %s %s 退还押金%.2f元", room.RoomNumber, today, refundedDeposit),
			BillDate:    today,
			CreatedBy:   userID,
		}
		if err := d.Create(&expenseBill).Error; err != nil {
			logger.Log.Error().Err(err).Uint("room_id", room.ID).Uint("building_id", buildingID).Msg("自动创建押金退还支出账单失败")
			return
		}
	}
}

func AutoCheckExpiringContracts(db *gorm.DB) {
	var buildings []models.Building
	db.Where("status = ?", "active").Find(&buildings)
	now := utils.Now()
	expiredCount := 0
	expiringCount := 0
	for _, building := range buildings {
		var contracts []models.RentalContract
		db.Where("status = ? AND building_id = ?", "active", building.ID).Find(&contracts)
		for _, c := range contracts {
			if c.EndDate == "" {
				continue
			}
			if endDate, err := time.Parse("2006-01-02", c.EndDate); err == nil {
				if endDate.Before(now) || endDate.Equal(now) {
					var room models.Room
					db.First(&room, c.RoomID)
					if room.Status != "vacant" && room.Status != "expired" {
						db.Model(&room).Update("status", "expired")
					}
					db.Model(&c).Update("status", "ended")
					var exist models.Task
					result := db.Where("room_id = ? AND type = ? AND status = ? AND building_id = ?", c.RoomID, "expired_room", "pending", building.ID).First(&exist)
					if result.Error != nil {
						task := models.Task{
							BuildingID:  building.ID,
							Title:       "房间 " + room.RoomNumber + " 已到期",
							Type:        "expired_room",
							Status:      "pending",
							RoomID:      &c.RoomID,
							Deposit:     c.Deposit,
							Description: "租约已到期，请处理押金退还等事宜",
						}
						db.Create(&task)
					}
					expiredCount++
					logger.Log.Info().
						Uint("contract_id", c.ID).
						Uint("room_id", c.RoomID).
						Uint("building_id", building.ID).
						Str("room_number", room.RoomNumber).
						Msg("合同到期，房间状态设为 expired")
				} else if utils.Until(endDate) < 30*24*time.Hour {
					db.Model(&models.Room{}).Where("id = ?", c.RoomID).Update("status", "expiring")
					expiringCount++
				} else {
					db.Model(&models.Room{}).Where("id = ? AND status = ?", c.RoomID, "expiring").Update("status", "rented")
				}
			}
		}
	}
	if expiredCount > 0 || expiringCount > 0 {
		logger.Log.Info().Int("expired", expiredCount).Int("expiring", expiringCount).Msg("合同到期检查完成")
	} else {
		logger.Log.Debug().Msg("合同到期检查完成，无到期合同")
	}
}

func (h *RoomHandler) GetActiveContract(c *gin.Context) {
	buildingID, exists := c.Get("building_id")
	if !exists {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	bid, ok := buildingID.(uint)
	if !ok {
		utils.Error(c, http.StatusInternalServerError, "服务器错误")
		return
	}
	roomID := c.Param("id")
	var contract models.RentalContract
	if err := h.DB.Where("room_id = ? AND status = ? AND building_id = ?", roomID, "active", bid).
		Preload("Tenant").Preload("Room").
		First(&contract).Error; err != nil {
		utils.Error(c, http.StatusNotFound, "无有效合同")
		return
	}
	utils.Success(c, gin.H{"contract": contract})
}
