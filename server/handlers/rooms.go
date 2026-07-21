// Package handlers 处理房间的增删改查及合同管理等 HTTP 请求
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

// UpdateRoomStatusReq 更新房间状态请求参数（出租/退租/预订/取消预订）
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

// List 分页获取房间列表，支持楼层、户型、状态筛选
func (h *RoomHandler) List(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}

	page, size := utils.ParsePage(c)
	floor := c.Query("floor")
	layout := c.Query("layout")
	requestedStatus := c.Query("status")

	rooms, total, err := h.RoomService.List(bid, page, size, floor, layout)
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

	if requestedStatus != "" {
		var filtered []RoomWithThumbnail
		for _, r := range result {
			if r.Status == requestedStatus {
				filtered = append(filtered, r)
			}
		}
		result = filtered
		total = int64(len(filtered))
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
		CurrentContract   *models.RentalContract `json:"current_contract,omitempty"`
		FutureReservation *models.RentalContract `json:"future_reservation,omitempty"`
		EndDate           string                  `json:"end_date"`
	}
	var futureReservation *models.RentalContract
	if rc, err := h.RoomService.GetFutureReservation(uint(rid)); err == nil {
		futureReservation = rc
	}

	detail := RoomDetail{Room: *room, CurrentContract: contract, FutureReservation: futureReservation}
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
		if err.Error() == "房间号已存在" {
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

	if req.Status != "rented" && req.Status != "vacant" && req.Status != "reserved" {
		utils.Error(c, http.StatusBadRequest, "无效的状态值，仅支持 rented、vacant 或 reserved")
		return
	}

	userID, _ := c.Get("user_id")
	uid, _ := userID.(uint)

	if req.Status == "reserved" {
		h.reserveRoom(c, room, &req)
		return
	}

	if req.Status == "rented" {
		var reservedContract *models.RentalContract
		if room.Status == "reserved" {
			if rc, err := h.RoomService.GetReservedContract(room.ID); err == nil {
				reservedContract = rc
			}
		}

		tx := h.DB.Begin()
		if tx.Error != nil {
			utils.Error(c, http.StatusInternalServerError, "服务器错误")
			return
		}

		if err := tx.Model(&models.RentalContract{}).Where("room_id = ? AND status = ?", room.ID, "active").Update("status", "ended").Error; err != nil {
			tx.Rollback()
			logger.Log.Error().Err(err).Uint("room_id", room.ID).Msg("结束旧合同失败")
			utils.Error(c, http.StatusInternalServerError, "结束旧合同失败")
			return
		}

		var contract models.RentalContract
		if reservedContract != nil {
			contract = *reservedContract
			updates := map[string]interface{}{
				"rent_price":     req.RentPrice,
				"management_fee": req.ManagementFee,
				"deposit":        req.Deposit,
				"start_date":     req.StartDate,
				"end_date":       req.EndDate,
				"status":         "active",
			}
			if req.TenantName != "" {
				if err := tx.Model(&models.Tenant{}).Where("id = ?", contract.TenantID).
					Updates(map[string]interface{}{"name": req.TenantName, "phone": req.TenantPhone}).Error; err != nil {
					tx.Rollback()
					logger.Log.Error().Err(err).Msg("更新租客信息失败")
					utils.Error(c, http.StatusInternalServerError, "确认签约失败")
					return
				}
			}
			if err := tx.Model(&models.RentalContract{}).Where("id = ?", contract.ID).Updates(updates).Error; err != nil {
				tx.Rollback()
				logger.Log.Error().Err(err).Msg("确认签约更新合同失败")
				utils.Error(c, http.StatusInternalServerError, "确认签约失败")
				return
			}
			contract.RentPrice = req.RentPrice
			contract.Deposit = req.Deposit
			contract.StartDate = req.StartDate
			contract.EndDate = req.EndDate
			contract.Status = "active"
		} else {
			tenant := models.Tenant{
				Name:  req.TenantName,
				Phone: req.TenantPhone,
			}
			if err := tx.Create(&tenant).Error; err != nil {
				tx.Rollback()
				logger.Log.Error().Err(err).Msg("创建租客失败")
				utils.Error(c, http.StatusInternalServerError, "创建失败")
				return
			}

			contract = models.RentalContract{
				RoomID:     room.ID,
				BuildingID: room.BuildingID,
				TenantID:   tenant.ID,
				RentPrice:  req.RentPrice,
				Deposit:    req.Deposit,
				StartDate:  req.StartDate,
				EndDate:    req.EndDate,
				Status:     "active",
			}
			if err := tx.Create(&contract).Error; err != nil {
				tx.Rollback()
				logger.Log.Error().Err(err).Msg("创建合同失败")
				utils.Error(c, http.StatusInternalServerError, "创建失败")
				return
			}
		}

		// 如果已预收款项，跳过账单创建
		isPrepaid := reservedContract != nil && reservedContract.Prepaid

		if !isPrepaid {
			now := utils.Now()
			datePart := now.Format("20060102")
			var count int64
			tx.Model(&models.Bill{}).
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
					PaidStatus:  "paid",
					CreatedBy:   uid,
				}
				if err := tx.Create(&rentBill).Error; err != nil {
					tx.Rollback()
					logger.Log.Error().Err(err).Msg("创建出租租金账单失败")
					utils.Error(c, http.StatusInternalServerError, "创建出租账单失败")
					return
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
					Description: "押金：出租押金（已收款）",
					BillDate:    now.Format("2006-01-02"),
					PaidStatus:  "paid",
					CreatedBy:   uid,
				}
				if err := tx.Create(&depositBill).Error; err != nil {
					tx.Rollback()
					logger.Log.Error().Err(err).Msg("创建出租押金账单失败")
					utils.Error(c, http.StatusInternalServerError, "创建出租账单失败")
					return
				}
				count++
			}
		}

		if err := tx.Where("room_id = ? AND type = ? AND status = ?", room.ID, "reserved_overdue", "pending").
			Delete(&models.Task{}).Error; err != nil {
			tx.Rollback()
			logger.Log.Error().Err(err).Msg("清理预订超时任务失败")
			utils.Error(c, http.StatusInternalServerError, "状态更新失败")
			return
		}
		if err := tx.Model(&models.Room{}).Where("id = ?", room.ID).Update("status", "rented").Error; err != nil {
			tx.Rollback()
			logger.Log.Error().Err(err).Msg("更新房间状态失败")
			utils.Error(c, http.StatusInternalServerError, "状态更新失败")
			return
		}
		tx.Commit()
		utils.SuccessWithMsg(c, "状态更新成功", nil)
		return
	} else if req.Status == "vacant" {
		if room.Status == "reserved" {
			h.cancelReservation(c, room, &req)
			return
		}

		// 退租：退还押金为必填项，未填写（nil）视为未确认，拒绝提交
		if req.RefundedDeposit == nil {
			utils.Error(c, http.StatusBadRequest, "请填写实际退还给租客的押金金额（如全额退还请填原押金，无需退还请填0）")
			return
		}
		if *req.RefundedDeposit < 0 {
			utils.Error(c, http.StatusBadRequest, "退还押金金额不能为负数")
			return
		}

		tx := h.DB.Begin()
		if tx.Error != nil {
			utils.Error(c, http.StatusInternalServerError, "服务器错误")
			return
		}

		if err := tx.Model(&models.RentalContract{}).Where("room_id = ? AND status = ?", room.ID, "active").Update("status", "ended").Error; err != nil {
			tx.Rollback()
			logger.Log.Error().Err(err).Uint("room_id", room.ID).Msg("结束合同失败")
			utils.Error(c, http.StatusInternalServerError, "结束合同失败")
			return
		}

		if *req.RefundedDeposit > 0 {
			now := utils.Now()
			datePart := now.Format("20060102")
			var count int64
			tx.Model(&models.Bill{}).
				Where("building_id = ? AND bill_no LIKE ?", room.BuildingID, "B"+datePart+"%").
				Count(&count)

			bill := models.Bill{
				BillNo:      fmt.Sprintf("B%s%05d", datePart, count+1),
				Type:        "expense",
				Subtype:     "押金退还",
				Amount:      *req.RefundedDeposit,
				BuildingID:  room.BuildingID,
				RoomID:      &room.ID,
				Description: "押金退还：退租押金支出（已退款）",
				BillDate:    now.Format("2006-01-02"),
				PaidStatus:  "paid",
				CreatedBy:   uid,
			}
			if err := tx.Create(&bill).Error; err != nil {
				tx.Rollback()
				logger.Log.Error().Err(err).Msg("创建押金退还账单失败")
				utils.Error(c, http.StatusInternalServerError, "创建退还账单失败")
				return
			}
		}

		hasFutureReservation := h.RoomService.HasFutureReservation(room.ID)
		targetStatus := "vacant"
		if hasFutureReservation {
			targetStatus = "reserved"
		}
		if err := tx.Model(&models.Room{}).Where("id = ?", room.ID).Update("status", targetStatus).Error; err != nil {
			tx.Rollback()
			logger.Log.Error().Err(err).Msg("更新房间状态失败")
			utils.Error(c, http.StatusInternalServerError, "状态更新失败")
			return
		}
		tx.Commit()

		if hasFutureReservation {
			utils.SuccessWithMsg(c, "退租成功，下一租客已预定，请完成签约入住", nil)
		} else {
			utils.SuccessWithMsg(c, "状态更新成功", nil)
		}
		return
	}

	utils.SuccessWithMsg(c, "状态更新成功", nil)
}

// reserveRoom 交定金预订房间。支持两种场景：
//   - vacant → reserved：空房预订，房间状态直接变为 reserved
//   - rented/expiring → 未来预定：房间保持 rented，创建 future reservation 合同
func (h *RoomHandler) reserveRoom(c *gin.Context, room *models.Room, req *UpdateRoomStatusReq) {
	if room.Status == "reserved" {
		utils.Error(c, http.StatusBadRequest, "该房间已被预订")
		return
	}
	if req.TenantName == "" {
		utils.Error(c, http.StatusBadRequest, "请填写租客姓名")
		return
	}
	if req.EarnestMoney <= 0 {
		utils.Error(c, http.StatusBadRequest, "定金金额必须大于0")
		return
	}

	isFutureReservation := room.Status == "rented" || room.Status == "expiring" || room.Status == "expired"

	if isFutureReservation {
		if h.RoomService.HasFutureReservation(room.ID) {
			utils.Error(c, http.StatusBadRequest, "该房间已有未来预订记录")
			return
		}
		currentContract, err := h.RoomService.GetActiveContract(room.ID)
		if err != nil {
			utils.Error(c, http.StatusBadRequest, "该房间无活跃合同")
			return
		}
		if req.StartDate != "" && currentContract.EndDate != "" && req.StartDate < currentContract.EndDate {
			utils.Error(c, http.StatusBadRequest, "未来预定的起租日期不能早于当前合同的结束日期")
			return
		}
	}

	tx := h.DB.Begin()
	if tx.Error != nil {
		utils.Error(c, http.StatusInternalServerError, "服务器错误")
		return
	}

	tenant := models.Tenant{
		Name:  req.TenantName,
		Phone: req.TenantPhone,
	}
	if err := tx.Create(&tenant).Error; err != nil {
		tx.Rollback()
		logger.Log.Error().Err(err).Msg("创建租客失败")
		utils.Error(c, http.StatusInternalServerError, "预订失败")
		return
	}

	contract := models.RentalContract{
		RoomID:        room.ID,
		BuildingID:    room.BuildingID,
		TenantID:      tenant.ID,
		RentPrice:     req.RentPrice,
		ManagementFee: req.ManagementFee,
		Deposit:       req.Deposit,
		EarnestMoney:  req.EarnestMoney,
		StartDate:     req.StartDate,
		EndDate:       req.EndDate,
		Status:        "reserved",
	}
	if err := tx.Create(&contract).Error; err != nil {
		tx.Rollback()
		logger.Log.Error().Err(err).Msg("创建预订合同失败")
		utils.Error(c, http.StatusInternalServerError, "预订失败")
		return
	}

	if !isFutureReservation {
		if err := tx.Model(&models.Room{}).Where("id = ?", room.ID).Update("status", "reserved").Error; err != nil {
			tx.Rollback()
			logger.Log.Error().Err(err).Msg("更新房间状态失败")
			utils.Error(c, http.StatusInternalServerError, "预订失败")
			return
		}
	}
	tx.Commit()

	if isFutureReservation {
		utils.SuccessWithMsg(c, "未来预订成功，当前租客退租后生效", nil)
	} else {
		utils.SuccessWithMsg(c, "预订成功", nil)
	}
}

// cancelReservation 取消预订（reserved → vacant），按退还金额与定金差额记违约收入/支出。
// 退还定金为必填项：未填写（nil）视为未确认，拒绝提交；refunded==earnest 表示正常无损退定。
func (h *RoomHandler) cancelReservation(c *gin.Context, room *models.Room, req *UpdateRoomStatusReq) {
	userID, _ := c.Get("user_id")
	uid, _ := userID.(uint)

	contract, err := h.RoomService.GetReservedContract(room.ID)
	if err != nil {
		logger.Log.Error().Err(err).Uint("room_id", room.ID).Msg("查询预订合同失败")
		utils.Error(c, http.StatusNotFound, "无预订记录")
		return
	}

	earnest := contract.EarnestMoney
	if req.RefundedDeposit == nil {
		utils.Error(c, http.StatusBadRequest, "请填写实际退还给租客的定金金额（如定金不退请填0，房东违约赔付请填超过定金的金额）")
		return
	}
	refunded := *req.RefundedDeposit
	if refunded < 0 {
		utils.Error(c, http.StatusBadRequest, "退还金额不能为负数")
		return
	}

	diff := float64(int((earnest-refunded)*100)) / 100
	tx := h.DB.Begin()
	if tx.Error != nil {
		utils.Error(c, http.StatusInternalServerError, "服务器错误")
		return
	}

	if diff != 0 {
		now := utils.Now()
		datePart := now.Format("20060102")
		var count int64
		tx.Model(&models.Bill{}).
			Where("building_id = ? AND bill_no LIKE ?", room.BuildingID, "B"+datePart+"%").
			Count(&count)

		bill := models.Bill{
			BillNo:      fmt.Sprintf("B%s%05d", datePart, count+1),
			BuildingID:  room.BuildingID,
			RoomID:      &room.ID,
			Subtype:     "定金违约",
			BillDate:    now.Format("2006-01-02"),
			PaidStatus:  "paid",
			CreatedBy:   uid,
			Description: fmt.Sprintf("取消预订：原收定金%.2f元，实际退还%.2f元", earnest, refunded),
		}
		if diff > 0 {
			bill.Type = "income"
			bill.Amount = diff
			bill.Description += fmt.Sprintf("，租客违约/不租，扣留定金%.2f元", diff)
		} else {
			bill.Type = "expense"
			bill.Amount = -diff
			bill.Description += fmt.Sprintf("，房东违约/主动多退，额外赔付%.2f元", -diff)
		}
		if err := tx.Create(&bill).Error; err != nil {
			tx.Rollback()
			logger.Log.Error().Err(err).Msg("创建定金违约账单失败")
			utils.Error(c, http.StatusInternalServerError, "创建违约账单失败")
			return
		}
	}

	if err := tx.Model(&models.RentalContract{}).Where("id = ?", contract.ID).Update("status", "cancelled").Error; err != nil {
		tx.Rollback()
		logger.Log.Error().Err(err).Msg("取消预订合同失败")
		utils.Error(c, http.StatusInternalServerError, "取消预订失败")
		return
	}
	if err := tx.Where("room_id = ? AND type = ? AND status = ?", room.ID, "reserved_overdue", "pending").
		Delete(&models.Task{}).Error; err != nil {
		tx.Rollback()
		logger.Log.Error().Err(err).Msg("清理预订超时任务失败")
		utils.Error(c, http.StatusInternalServerError, "取消预订失败")
		return
	}
	if err := tx.Model(&models.Room{}).Where("id = ?", room.ID).Update("status", "vacant").Error; err != nil {
		tx.Rollback()
		logger.Log.Error().Err(err).Msg("更新房间状态失败")
		utils.Error(c, http.StatusInternalServerError, "取消预订失败")
		return
	}
	tx.Commit()
	utils.SuccessWithMsg(c, "已取消预订", nil)
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

// PrepayContractReq 预收款项请求参数（租金按合同约定起租日折算，无需手填）
type PrepayContractReq struct {
	Deposit   float64 `json:"deposit" binding:"gte=0"` // 实收押金（不含已收定金，定金单独体现在取消预订的违约账单中）
	StartDate string  `json:"start_date"`               // 起租日期（缺省取合同值，用于折算本月租金）
	EndDate   string  `json:"end_date"`                 // 结束日期（缺省取合同值）
}

// PrepayContract 预收款项：未来预定的租客在入住前缴纳押金和首期租金。
// 仅创建"押金"（全额，不扣除已收定金）和"租金"两张账单；
// 首期租金按合同约定起租日折算至当月月末（若起租日在下月，则本月不收，由定时任务后续生成）。
// 注：定金已收部分不另建账单、也不抵扣押金；仅"取消预订"会产生定金相关（违约）账单。
func (h *RoomHandler) PrepayContract(c *gin.Context) {
	roomID := c.Param("id")
	rid, err := strconv.ParseUint(roomID, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的房间ID")
		return
	}

	room, err := h.RoomService.GetByID(uint(rid))
	if err != nil {
		utils.Error(c, http.StatusNotFound, "房间不存在")
		return
	}

	var req PrepayContractReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	contract, err := h.RoomService.GetReservedContract(room.ID)
	if err != nil {
		utils.Error(c, http.StatusNotFound, "无预订合同")
		return
	}

	if contract.Prepaid {
		utils.Error(c, http.StatusBadRequest, "该合同已预收款项")
		return
	}

	startDate := req.StartDate
	if startDate == "" {
		startDate = contract.StartDate
	}
	endDate := req.EndDate
	if endDate == "" {
		endDate = contract.EndDate
	}
	// 起租日必须早于合同结束日
	if startDate != "" && endDate != "" && startDate >= endDate {
		utils.Error(c, http.StatusBadRequest, "起租日期必须早于结束日期")
		return
	}

	userID, _ := c.Get("user_id")
	uid, _ := userID.(uint)
	now := utils.Now()
	datePart := now.Format("20060102")

	tx := h.DB.Begin()
	if tx.Error != nil {
		utils.Error(c, http.StatusInternalServerError, "服务器错误")
		return
	}

	var count int64
	tx.Model(&models.Bill{}).
		Where("building_id = ? AND bill_no LIKE ?", room.BuildingID, "B"+datePart+"%").
		Count(&count)

	if req.Deposit > 0 {
		depositNo := fmt.Sprintf("B%s%05d", datePart, count+1)
		depositBill := models.Bill{
			BillNo:      depositNo,
			Type:        "income",
			Subtype:     "押金",
			Amount:      req.Deposit + contract.EarnestMoney,
			BuildingID:  room.BuildingID,
			RoomID:      &room.ID,
			Description: "押金：预收押金（已收款）",
			BillDate:    now.Format("2006-01-02"),
			PaidStatus:  "paid",
			CreatedBy:   uid,
		}
		if err := tx.Create(&depositBill).Error; err != nil {
			tx.Rollback()
			logger.Log.Error().Err(err).Msg("创建预收押金账单失败")
			utils.Error(c, http.StatusInternalServerError, "创建押金账单失败")
			return
		}
		count++
	}

	// 首期租金：按合同约定起租日折算至当月月末（不足整月按天算）
	if contract.RentPrice > 0 && startDate != "" {
		contractStart, perr := time.Parse("2006-01-02", startDate)
		if perr == nil {
			monthEnd := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location())
			// 起租日晚于本月月末，则本月不收租金，交由定时任务下月生成
			if contractStart.After(monthEnd) {
				// 无首期租金账单
			} else {
				billEnd := monthEnd
				if endDate != "" {
					if cEnd, eerr := time.Parse("2006-01-02", endDate); eerr == nil && cEnd.Before(monthEnd) {
						billEnd = cEnd
					}
				}
				daysInMonth := monthEnd.Day()
				var rentAmount, mgmtAmount float64
				var rentDesc string
				if contractStart.Day() == 1 && billEnd.Equal(monthEnd) {
					rentAmount = float64(int(contract.RentPrice*100)) / 100
					mgmtAmount = float64(int(contract.ManagementFee*100)) / 100
					rentDesc = monthEnd.Format("2006-01") + "-01 ~ " + billEnd.Format("2006-01-02")
				} else {
					rentAmount = utils.CalcProratedAmount(contract.RentPrice, contractStart, billEnd, daysInMonth)
					mgmtAmount = utils.CalcProratedAmount(contract.ManagementFee, contractStart, billEnd, daysInMonth)
					rentDesc = contractStart.Format("2006-01-02") + " ~ " + billEnd.Format("2006-01-02")
				}
				totalAmount := float64(int((rentAmount+mgmtAmount)*100)) / 100
				if totalAmount > 0 {
					rentNo := fmt.Sprintf("B%s%05d", datePart, count+1)
					rentBill := models.Bill{
						BillNo:      rentNo,
						Type:        "income",
						Subtype:     "租金",
						Amount:      totalAmount,
						BuildingID:  room.BuildingID,
						RoomID:      &room.ID,
						Description: fmt.Sprintf("租金：%.2f元，管理费：%.2f元（%s，已收款）", rentAmount, mgmtAmount, rentDesc),
						BillDate:    now.Format("2006-01-02"),
						PaidStatus:  "paid",
						CreatedBy:   uid,
					}
					if err := tx.Create(&rentBill).Error; err != nil {
						tx.Rollback()
						logger.Log.Error().Err(err).Msg("创建预收租金账单失败")
						utils.Error(c, http.StatusInternalServerError, "创建租金账单失败")
						return
					}
				}
			}
		}
	}

	if err := tx.Model(&models.RentalContract{}).Where("id = ?", contract.ID).Update("prepaid", true).Error; err != nil {
		tx.Rollback()
		logger.Log.Error().Err(err).Msg("更新合同预付款状态失败")
		utils.Error(c, http.StatusInternalServerError, "更新失败")
		return
	}

	tx.Commit()
	utils.SuccessWithMsg(c, "预收款项成功", nil)
}
