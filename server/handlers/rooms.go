package handlers

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"time"

	"rental-server/models"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type RoomHandler struct {
	DB *gorm.DB
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

// 公开：获取公寓下房间列表
func (h *RoomHandler) ListPublic(c *gin.Context) {
	buildingID := c.Param("bid")
	var rooms []models.Room
	query := h.DB.Preload("Media", func(db *gorm.DB) *gorm.DB {
		return db.Where("type = ?", "image").Order("FIELD(category,'cover','gallery'), sort_order asc")
	}).Where("building_id = ?", buildingID)
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
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
	c.JSON(http.StatusOK, gin.H{"rooms": result})
}

// 公开：获取房间详情
func (h *RoomHandler) GetPublic(c *gin.Context) {
	roomID := c.Param("rid")
	buildingID := c.Param("id")
	var room models.Room
	if err := h.DB.Preload("Media", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order asc")
	}).Where("id = ? AND building_id = ?", roomID, buildingID).First(&room).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "房间不存在"})
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
	c.JSON(http.StatusOK, gin.H{"room": detail})
}

// 公开：获取有效合同
func (h *RoomHandler) GetActiveContractPublic(c *gin.Context) {
	roomID := c.Param("rid")
	buildingID := c.Param("id")
	var contract models.RentalContract
	if err := h.DB.Where("room_id = ? AND status = ? AND building_id = ?", roomID, "active", buildingID).
		Preload("Tenant").Preload("Room").
		First(&contract).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "无有效合同"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"contract": contract})
}

// 管理员：创建房间
func (h *RoomHandler) Create(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
	var req CreateRoomReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
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
		c.JSON(http.StatusConflict, gin.H{"error": "房间号已存在"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"room": room})
}

// 管理员：获取该楼栋所有房间
func (h *RoomHandler) List(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
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
	c.JSON(http.StatusOK, gin.H{"rooms": result})
}

// 管理员：获取房间详情
func (h *RoomHandler) Get(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
	roomID := c.Param("id")
	var room models.Room
	if err := h.DB.Preload("Media", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order asc")
	}).Where("id = ? AND building_id = ?", roomID, bid).First(&room).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "房间不存在"})
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
	c.JSON(http.StatusOK, gin.H{"room": detail})
}

// 管理员：更新房间信息
func (h *RoomHandler) Update(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
	roomID := c.Param("id")
	var room models.Room
	if err := h.DB.Where("id = ? AND building_id = ?", roomID, bid).First(&room).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "房间不存在"})
		return
	}
	var req UpdateRoomReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
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
	h.DB.Model(&room).Updates(updates)
	c.JSON(http.StatusOK, gin.H{"room": room})
}

// 管理员：删除房间
func (h *RoomHandler) Delete(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
	roomID := c.Param("id")
	var room models.Room
	if err := h.DB.Where("id = ? AND building_id = ?", roomID, bid).First(&room).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "房间不存在"})
		return
	}
	if room.Status == "rented" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "房间已出租，无法删除"})
		return
	}
	h.DB.Delete(&room)
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// 管理员：修改房间状态
func (h *RoomHandler) UpdateStatus(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
	roomID := c.Param("id")
	var room models.Room
	if err := h.DB.Where("id = ? AND building_id = ?", roomID, bid).First(&room).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "房间不存在"})
		return
	}
	var req UpdateRoomStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	switch req.Status {
	case "vacant", "rented":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "状态值无效"})
		return
	}
	if req.Status == "rented" && req.TenantName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "出租需填写租客姓名"})
		return
	}
	if req.Status == "rented" && room.Status == "expiring" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "即将退租的房间请先修改退租日期，不能直接设为已出租"})
		return
	}
	if req.Status == "rented" && req.RentPrice <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写租金金额"})
		return
	}
	if req.Status == "rented" && req.StartDate != "" && req.EndDate != "" {
		start, err1 := time.Parse("2006-01-02", req.StartDate)
		end, err2 := time.Parse("2006-01-02", req.EndDate)
		if err1 == nil && err2 == nil && !end.After(start) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "退租日期必须大于起租日期"})
			return
		}
	}

	if req.Status == "rented" {
		var tenant models.Tenant
		h.DB.Where("name = ? AND phone = ?", req.TenantName, req.TenantPhone).First(&tenant)
		if tenant.ID == 0 {
			tenant = models.Tenant{
				Name:  req.TenantName,
				Phone: req.TenantPhone,
			}
			h.DB.Create(&tenant)
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
		userID, _ := c.Get("user_id")
		createProratedRentBill(h.DB, room, contract, userID.(uint), bid)
		newStatus := "rented"
		if endDateParsed, err := time.Parse("2006-01-02", endDate); err == nil {
			if utils.Until(endDateParsed) < 30*24*time.Hour {
				newStatus = "expiring"
			}
		}
		h.DB.Model(&room).Update("status", newStatus)
		c.JSON(http.StatusOK, gin.H{"message": "出租成功", "status": newStatus})
		return
	} else if req.Status == "vacant" {
		if req.RefundedDeposit != nil {
			userID, _ := c.Get("user_id")
			handleDepositRefund(h.DB, room, *req.RefundedDeposit, userID.(uint), bid)
		}
		h.DB.Model(&models.RentalContract{}).
			Where("room_id = ? AND status = ?", room.ID, "active").
			Updates(map[string]interface{}{
				"status":   "ended",
				"end_date": utils.Now().Format("2006-01-02"),
			})
		h.DB.Model(&room).Update("status", "vacant")
	}
	c.JSON(http.StatusOK, gin.H{"message": "状态更新成功", "status": req.Status})
}

// 管理员：续租
func (h *RoomHandler) UpdateContractEndDate(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
	roomID := c.Param("id")
	var contract models.RentalContract
	if err := h.DB.Where("room_id = ? AND status = ? AND building_id = ?", roomID, "active", bid).First(&contract).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "无有效合同"})
		return
	}
	var req UpdateContractReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "日期格式错误"})
		return
	}
	startDate, err := time.Parse("2006-01-02", contract.StartDate)
	if err == nil && !endDate.After(startDate) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "退租日期必须大于起租日期"})
		return
	}
	updates := map[string]interface{}{"end_date": req.EndDate}
	if req.RentPrice > 0 {
		updates["rent_price"] = req.RentPrice
	}
	h.DB.Model(&contract).Updates(updates)
	if utils.Until(endDate) < 30*24*time.Hour {
		h.DB.Model(&models.Room{}).Where("id = ?", roomID).Update("status", "expiring")
	} else {
		h.DB.Model(&models.Room{}).Where("id = ? AND status = ?", roomID, "expiring").Update("status", "rented")
	}
	c.JSON(http.StatusOK, gin.H{"message": "退租日期更新成功"})
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
	desc := fmt.Sprintf("房间 %s %s 租金（%s至%s）", room.RoomNumber, monthStr, startDate.Format("2006-01-02"), billEnd.Format("2006-01-02"))

	var existing models.Bill
	result := db.Where("building_id = ? AND room_id = ? AND subtype = ? AND DATE_FORMAT(bill_date, '%Y-%m') = ?", buildingID, room.ID, "租金", monthStr).First(&existing)
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

	billNo := fmt.Sprintf("B%s%04d", utils.Now().Format("20060102150405"), utils.Now().UnixMilli()%10000)
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
		log.Printf("自动创建租金账单失败: %v", err)
	}
}

// 处理押金退还
func handleDepositRefund(db *gorm.DB, room models.Room, refundedDeposit float64, userID uint, buildingID uint) {
	var contract models.RentalContract
	err := db.Where("room_id = ? AND status = ? AND building_id = ?", room.ID, "active", buildingID).First(&contract).Error
	if err != nil {
		err = db.Where("room_id = ? AND status = ? AND building_id = ?", room.ID, "ended", buildingID).Order("created_at desc").First(&contract).Error
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
	incomeBillNo := fmt.Sprintf("B%s%04d", utils.Now().Format("20060102150405"), utils.Now().UnixMilli()%10000)
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
	if err := db.Create(&incomeBill).Error; err != nil {
		log.Printf("自动创建押金收入账单失败: %v", err)
	}
}

// 检查所有即将到期的合同
func AutoCheckExpiringContracts(db *gorm.DB) {
	var buildings []models.Building
	db.Where("status = ?", "active").Find(&buildings)
	now := utils.Now()
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
				} else if utils.Until(endDate) < 30*24*time.Hour {
					db.Model(&models.Room{}).Where("id = ?", c.RoomID).Update("status", "expiring")
				} else {
					db.Model(&models.Room{}).Where("id = ? AND status = ?", c.RoomID, "expiring").Update("status", "rented")
				}
			}
		}
	}
}

func (h *RoomHandler) GetActiveContract(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
	roomID := c.Param("id")
	var contract models.RentalContract
	if err := h.DB.Where("room_id = ? AND status = ? AND building_id = ?", roomID, "active", bid).
		Preload("Tenant").Preload("Room").
		First(&contract).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "无有效合同"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"contract": contract})
}
