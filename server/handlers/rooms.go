// 房间处理器，处理房间的增删改查及合同管理等HTTP请求
package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"rental-server/config"
	"rental-server/logger"
	"rental-server/models"
	"rental-server/services"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RoomHandler 房间处理器
type RoomHandler struct {
	DB          *gorm.DB
	Cfg         *config.Config
	RoomService *services.RoomService
}

// hasAuth 检查请求是否包含有效的认证令牌
func (h *RoomHandler) hasAuth(c *gin.Context) bool {
	tokenStr := c.GetHeader("Authorization")
	if tokenStr == "" || !strings.HasPrefix(tokenStr, "Bearer ") {
		return false
	}
	tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")
	_, err := utils.ParseToken(tokenStr, h.Cfg.JWTSecret)
	return err == nil
}

// CreateRoomReq 创建房间请求参数
type CreateRoomReq struct {
	RoomNumber           string   `json:"room_number" binding:"required"`
	Floor                string   `json:"floor" binding:"required"`
	Layout               string   `json:"layout" binding:"required"`
	Description          string   `json:"description"`
	RentPrice            *float64 `json:"rent_price" binding:"required"`
	DepositMonths        *uint    `json:"deposit_months" binding:"required"`
	ManagementFee        *float64 `json:"management_fee" binding:"required"`
	ElectricityUnitPrice *float64 `json:"electricity_unit_price" binding:"required"`
	WaterUnitPrice       *float64 `json:"water_unit_price" binding:"required"`
}

// UpdateRoomReq 更新房间请求参数
type UpdateRoomReq struct {
	RoomNumber           string   `json:"room_number"`
	Floor                string   `json:"floor"`
	Layout               string   `json:"layout"`
	Description          string   `json:"description"`
	RentPrice            *float64 `json:"rent_price"`
	DepositMonths        *uint    `json:"deposit_months"`
	ManagementFee        *float64 `json:"management_fee"`
	ElectricityUnitPrice *float64 `json:"electricity_unit_price"`
	WaterUnitPrice       *float64 `json:"water_unit_price"`
}

// UpdateRoomStatusReq 更新房间状态请求参数（出租/退租）
type UpdateRoomStatusReq struct {
	Status          string   `json:"status" binding:"required"`
	TenantName      string   `json:"tenant_name"`
	TenantPhone     string   `json:"tenant_phone"`
	RentPrice       float64  `json:"rent_price" binding:"gte=0"`
	ManagementFee   float64  `json:"management_fee" binding:"gte=0"`
	Deposit         float64  `json:"deposit" binding:"gte=0"`
	EarnestMoney    float64  `json:"earnest_money" binding:"gte=0"`
	StartDate       string   `json:"start_date"`
	EndDate         string   `json:"end_date"`
	RefundedDeposit *float64 `json:"refunded_deposit"`
}

// UpdateContractReq 续租合同请求参数
type UpdateContractReq struct {
	EndDate   string  `json:"end_date" binding:"required"`
	RentPrice float64 `json:"rent_price"`
}

// GetPublic 获取公开房间详情（含租期状态）
func (h *RoomHandler) GetPublic(c *gin.Context) {
	roomID := c.Param("rid")
	buildingID := c.Param("id")
	rid, err := strconv.ParseUint(roomID, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的房间ID")
		return
	}
	bid, err := strconv.ParseUint(buildingID, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的公寓ID")
		return
	}

	room, contract, err := h.RoomService.GetWithContract(uint(rid))
	if err != nil {
		utils.Error(c, http.StatusNotFound, "房间不存在")
		return
	}
	if room.BuildingID != uint(bid) {
		utils.Error(c, http.StatusNotFound, "房间不存在")
		return
	}

	type RoomDetail struct {
		models.Room
		CurrentContract *models.RentalContract `json:"current_contract,omitempty"`
		EndDate         string                  `json:"end_date"`
	}
	detail := RoomDetail{Room: *room}
	if contract != nil {
		detail.EndDate = contract.EndDate
		detail.Room.Status = utils.DynamicRoomStatus(detail.Room.Status, detail.EndDate)
	}
	if h.hasAuth(c) && contract != nil {
		tokenStr := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		claims, err := utils.ParseToken(tokenStr, h.Cfg.JWTSecret)
		if err == nil && claims.BuildingID > 0 && claims.BuildingID == room.BuildingID {
			detail.CurrentContract = contract
		}
	}
	go utils.RecordPageView(h.DB, "room_detail", uint(rid), uint(bid), utils.GetRealIP(c))
	utils.Success(c, gin.H{"room": detail})
}

// GetActiveContractPublic 获取公开活跃合同信息
func (h *RoomHandler) GetActiveContractPublic(c *gin.Context) {
	roomID := c.Param("rid")
	bid, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的公寓ID")
		return
	}
	rid, err := strconv.ParseUint(roomID, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的房间ID")
		return
	}

	room, err := h.RoomService.GetByID(uint(rid))
	if err != nil || room.BuildingID != uint(bid) {
		utils.Error(c, http.StatusNotFound, "房间不存在")
		return
	}

	contract, err := h.RoomService.GetActiveContractPublic(uint(rid))
	if err != nil {
		utils.Error(c, http.StatusNotFound, "无有效合同")
		return
	}
	utils.Success(c, gin.H{"contract": contract})
}

// List 分页获取房间列表
func (h *RoomHandler) List(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}

	page, size := utils.ParsePage(c)
	rooms, total, err := h.RoomService.List(bid, page, size)
	if err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("查询房间列表失败")
		utils.Error(c, http.StatusInternalServerError, "查询失败")
		return
	}

	roomIDs := make([]uint, len(rooms))
	for i, r := range rooms {
		roomIDs[i] = r.ID
	}
	var contracts []models.RentalContract
	h.DB.Where("room_id IN ? AND status = ?", roomIDs, "active").Find(&contracts)
	contractMap := make(map[uint]string)
	for _, ct := range contracts {
		contractMap[ct.RoomID] = ct.EndDate
	}

	type RoomWithThumbnail struct {
		models.Room
		Thumbnail string `json:"thumbnail"`
		EndDate   string `json:"end_date"`
	}
	var result []RoomWithThumbnail
	for _, r := range rooms {
		thumb := ""
		for _, m := range r.Media {
			if m.Category == "cover" && m.Type == "image" {
				if m.ThumbnailPath != "" {
					thumb = m.ThumbnailPath
				} else {
					thumb = m.FilePath
				}
				break
			}
		}
		if thumb == "" {
			for _, m := range r.Media {
				if m.Type == "image" {
					if m.ThumbnailPath != "" {
						thumb = m.ThumbnailPath
					} else {
						thumb = m.FilePath
					}
					break
				}
			}
		}
		endDate := contractMap[r.ID]
		r.Status = utils.DynamicRoomStatus(r.Status, endDate)
		result = append(result, RoomWithThumbnail{Room: r, Thumbnail: thumb, EndDate: endDate})
	}
	utils.Success(c, gin.H{"rooms": result, "total": total, "page": page, "size": size})
}

// Get 获取单个房间详情
func (h *RoomHandler) Get(c *gin.Context) {
	roomID := c.Param("id")
	rid, err := strconv.ParseUint(roomID, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的房间ID")
		return
	}

	room, contract, err := h.RoomService.GetWithContract(uint(rid))
	if err != nil {
		utils.Error(c, http.StatusNotFound, "房间不存在")
		return
	}

	type RoomDetail struct {
		models.Room
		CurrentContract *models.RentalContract `json:"current_contract,omitempty"`
		EndDate         string                  `json:"end_date"`
	}
	detail := RoomDetail{Room: *room, CurrentContract: contract}
	if contract != nil {
		detail.EndDate = contract.EndDate
		detail.Room.Status = utils.DynamicRoomStatus(detail.Room.Status, detail.EndDate)
	}
	utils.Success(c, gin.H{"room": detail})
}

// Create 创建新房间
func (h *RoomHandler) Create(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}

	var req CreateRoomReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	if *req.RentPrice <= 0 || *req.ManagementFee < 0 || *req.ElectricityUnitPrice < 0 || *req.WaterUnitPrice < 0 {
		utils.Error(c, http.StatusBadRequest, "价格信息不能为负数")
		return
	}
	if *req.DepositMonths > 3 {
		utils.Error(c, http.StatusBadRequest, "押金月数不能超过3")
		return
	}

	room := models.Room{
		BuildingID:           bid,
		RoomNumber:           req.RoomNumber,
		Floor:                req.Floor,
		Layout:               req.Layout,
		Description:          req.Description,
		Status:               "vacant",
		RentPrice:            req.RentPrice,
		DepositMonths:        req.DepositMonths,
		ManagementFee:        req.ManagementFee,
		ElectricityUnitPrice: req.ElectricityUnitPrice,
		WaterUnitPrice:       req.WaterUnitPrice,
	}

	if err := h.RoomService.Create(&room); err != nil {
		if strings.Contains(err.Error(), "Duplicate") || strings.Contains(err.Error(), "UNIQUE") {
			utils.Error(c, http.StatusConflict, "房间号已存在")
			return
		}
		logger.Log.Error().Err(err).Msg("创建房间失败")
		utils.Error(c, http.StatusInternalServerError, "创建失败")
		return
	}

	utils.Created(c, "创建成功", gin.H{"room": room})
}

// Update 更新房间信息
func (h *RoomHandler) Update(c *gin.Context) {
	roomID := c.Param("id")
	rid, err := strconv.ParseUint(roomID, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的房间ID")
		return
	}

	var req UpdateRoomReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	updates := map[string]interface{}{}
	if req.RoomNumber != "" {
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
	if req.RentPrice != nil {
		updates["rent_price"] = *req.RentPrice
	}
	if req.DepositMonths != nil {
		updates["deposit_months"] = *req.DepositMonths
	}
	if req.ManagementFee != nil {
		updates["management_fee"] = *req.ManagementFee
	}
	if req.ElectricityUnitPrice != nil {
		updates["electricity_unit_price"] = *req.ElectricityUnitPrice
	}
	if req.WaterUnitPrice != nil {
		updates["water_unit_price"] = *req.WaterUnitPrice
	}

	if err := h.RoomService.Update(uint(rid), updates); err != nil {
		if strings.Contains(err.Error(), "Duplicate") || strings.Contains(err.Error(), "UNIQUE") {
			utils.Error(c, http.StatusConflict, "房间号已存在")
			return
		}
		logger.Log.Error().Err(err).Msg("更新房间失败")
		utils.Error(c, http.StatusInternalServerError, "更新失败")
		return
	}

	utils.SuccessWithMsg(c, "更新成功", nil)
}

// Delete 删除房间
func (h *RoomHandler) Delete(c *gin.Context) {
	roomID := c.Param("id")
	rid, err := strconv.ParseUint(roomID, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的房间ID")
		return
	}

	var activeContractCount int64
	h.DB.Model(&models.RentalContract{}).
		Where("room_id = ? AND status = ?", rid, "active").
		Count(&activeContractCount)
	if activeContractCount > 0 {
		utils.Error(c, http.StatusBadRequest, "该房间存在活跃合同，无法删除")
		return
	}

	if err := h.RoomService.Delete(uint(rid)); err != nil {
		logger.Log.Error().Err(err).Msg("删除房间失败")
		utils.Error(c, http.StatusInternalServerError, "删除失败")
		return
	}

	utils.SuccessWithMsg(c, "删除成功", nil)
}

// UpdateStatus 更新房间状态（出租/退租），同时创建合同和账单
func (h *RoomHandler) UpdateStatus(c *gin.Context) {
	roomID := c.Param("id")
	rid, err := strconv.ParseUint(roomID, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的房间ID")
		return
	}

	var req UpdateRoomStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	room, err := h.RoomService.GetByID(uint(rid))
	if err != nil {
		utils.Error(c, http.StatusNotFound, "房间不存在")
		return
	}

	if req.Status != "rented" && req.Status != "vacant" {
		utils.Error(c, http.StatusBadRequest, "无效的状态值，仅支持 rented 或 vacant")
		return
	}

	userID, _ := c.Get("user_id")
	uid, _ := userID.(uint)

	if req.Status == "rented" {
		if err := h.DB.Model(&models.RentalContract{}).Where("room_id = ? AND status = ?", room.ID, "active").Update("status", "ended").Error; err != nil {
			logger.Log.Error().Err(err).Uint("room_id", room.ID).Msg("结束旧合同失败")
		}

		tenant := models.Tenant{
			Name:  req.TenantName,
			Phone: req.TenantPhone,
		}
		if err := h.RoomService.CreateTenant(&tenant); err != nil {
			logger.Log.Error().Err(err).Msg("创建租客失败")
			utils.Error(c, http.StatusInternalServerError, "创建失败")
			return
		}

		contract := models.RentalContract{
			RoomID:     room.ID,
			BuildingID: room.BuildingID,
			TenantID:   tenant.ID,
			RentPrice:  req.RentPrice,
			Deposit:    req.Deposit,
			StartDate:  req.StartDate,
			EndDate:    req.EndDate,
			Status:     "active",
		}
		if err := h.RoomService.CreateContract(&contract); err != nil {
			logger.Log.Error().Err(err).Msg("创建合同失败")
			utils.Error(c, http.StatusInternalServerError, "创建失败")
			return
		}

		now := utils.Now()
		datePart := now.Format("20060102")
		var count int64
		h.DB.Model(&models.Bill{}).
			Where("building_id = ? AND bill_no LIKE ?", room.BuildingID, "B"+datePart+"%").
			Count(&count)

		if req.RentPrice > 0 {
			startDate, _ := time.Parse("2006-01-02", req.StartDate)
			endDate, _ := time.Parse("2006-01-02", req.EndDate)
			monthEnd := time.Date(startDate.Year(), startDate.Month()+1, 0, 0, 0, 0, 0, startDate.Location())
			daysInMonth := monthEnd.Day()

			var billEnd time.Time
			if endDate.Before(monthEnd) || endDate.Equal(monthEnd) {
				billEnd = endDate
			} else {
				billEnd = monthEnd
			}

			var rentAmount float64
			var mgmtAmount float64
			var rentDesc string
			if startDate.Day() == 1 && billEnd.Equal(monthEnd) {
				rentAmount = float64(int(req.RentPrice*100)) / 100
				mgmtAmount = float64(int(req.ManagementFee*100)) / 100
				rentDesc = monthEnd.Format("2006-01") + "-01 ~ " + billEnd.Format("2006-01-02")
			} else {
				rentAmount = utils.CalcProratedAmount(req.RentPrice, startDate, billEnd, daysInMonth)
				mgmtAmount = utils.CalcProratedAmount(req.ManagementFee, startDate, billEnd, daysInMonth)
				rentDesc = startDate.Format("2006-01-02") + " ~ " + billEnd.Format("2006-01-02")
			}

			totalAmount := float64(int((rentAmount+mgmtAmount)*100)) / 100

			rentNo := fmt.Sprintf("B%s%05d", datePart, count+1)
			rentBill := models.Bill{
				BillNo:      rentNo,
				Type:        "income",
				Subtype:     "租金",
				Amount:      totalAmount,
				BuildingID:  room.BuildingID,
				RoomID:      &room.ID,
				Description: fmt.Sprintf("租金：%.2f元，管理费：%.2f元", rentAmount, mgmtAmount) + "（" + rentDesc + "）",
				BillDate:    now.Format("2006-01-02"),
				CreatedBy:   uid,
			}
			if err := h.DB.Create(&rentBill).Error; err != nil {
				logger.Log.Error().Err(err).Msg("创建出租租金账单失败")
			}
			count++
		}

		if req.Deposit > 0 {
			depositNo := fmt.Sprintf("B%s%05d", datePart, count+1)
			depositBill := models.Bill{
				BillNo:      depositNo,
				Type:        "income",
				Subtype:     "押金",
				Amount:      req.Deposit,
				BuildingID:  room.BuildingID,
				RoomID:      &room.ID,
				Description: "押金：出租押金",
				BillDate:    now.Format("2006-01-02"),
				CreatedBy:   uid,
			}
			if err := h.DB.Create(&depositBill).Error; err != nil {
				logger.Log.Error().Err(err).Msg("创建出租押金账单失败")
			}
		}

		h.RoomService.UpdateStatus(room.ID, "rented")
	} else if req.Status == "vacant" {
		if err := h.DB.Model(&models.RentalContract{}).Where("room_id = ? AND status = ?", room.ID, "active").Update("status", "ended").Error; err != nil {
			logger.Log.Error().Err(err).Uint("room_id", room.ID).Msg("结束合同失败")
		}

		if req.RefundedDeposit != nil && *req.RefundedDeposit > 0 {
			now := utils.Now()
			datePart := now.Format("20060102")
			var count int64
			h.DB.Model(&models.Bill{}).
				Where("building_id = ? AND bill_no LIKE ?", room.BuildingID, "B"+datePart+"%").
				Count(&count)

			bill := models.Bill{
				BillNo:      fmt.Sprintf("B%s%05d", datePart, count+1),
				Type:        "expense",
				Subtype:     "押金退还",
				Amount:      *req.RefundedDeposit,
				BuildingID:  room.BuildingID,
				RoomID:      &room.ID,
				Description: "押金退还：退租押金支出",
				BillDate:    now.Format("2006-01-02"),
				CreatedBy:   uid,
			}
			if err := h.DB.Create(&bill).Error; err != nil {
				logger.Log.Error().Err(err).Msg("创建押金退还账单失败")
				utils.Error(c, http.StatusInternalServerError, "创建退还账单失败")
				return
			}
		}
		h.RoomService.UpdateStatus(room.ID, "vacant")
	}

	utils.SuccessWithMsg(c, "状态更新成功", nil)
}

// GetActiveContract 获取房间当前活跃合同
func (h *RoomHandler) GetActiveContract(c *gin.Context) {
	roomID := c.Param("id")
	rid, err := strconv.ParseUint(roomID, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的房间ID")
		return
	}

	contract, err := h.RoomService.GetActiveContract(uint(rid))
	if err != nil {
		utils.Error(c, http.StatusNotFound, "无有效合同")
		return
	}
	utils.Success(c, gin.H{"contract": contract})
}

// RenewContract 续租合同（延长租期或调整租金）
func (h *RoomHandler) RenewContract(c *gin.Context) {
	roomID := c.Param("id")
	rid, err := strconv.ParseUint(roomID, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的房间ID")
		return
	}

	var req UpdateContractReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	contract, err := h.RoomService.GetActiveContract(uint(rid))
	if err != nil {
		utils.Error(c, http.StatusNotFound, "无有效合同")
		return
	}

	updates := map[string]interface{}{
		"end_date": req.EndDate,
	}
	if req.RentPrice > 0 {
		updates["rent_price"] = req.RentPrice
	}

	if err := h.RoomService.UpdateContract(contract.ID, updates); err != nil {
		logger.Log.Error().Err(err).Msg("续租失败")
		utils.Error(c, http.StatusInternalServerError, "续租失败")
		return
	}

	utils.SuccessWithMsg(c, "续租成功", nil)
}
