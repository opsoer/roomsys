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

type BillHandler struct {
	DB *gorm.DB
}

type CreateBillReq struct {
	Type        string  `json:"type" binding:"required"`
	Subtype     string  `json:"subtype" binding:"required"`
	Amount      float64 `json:"amount" binding:"required"`
	RoomID      *uint   `json:"room_id"`
	Description string  `json:"description"`
	BillDate    string  `json:"bill_date" binding:"required"`
}

type UpdateBillReq struct {
	Type         string   `json:"type"`
	Subtype      string   `json:"subtype"`
	Amount       *float64 `json:"amount"`
	RoomID       *uint    `json:"room_id"`
	Description  string   `json:"description"`
	BillDate     string   `json:"bill_date"`
	ModifyReason string   `json:"modify_reason"`
}

func (h *BillHandler) List(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
	var bills []models.Bill
	query := h.DB.Preload("Room").Where("building_id = ?", bid)
	if t := c.Query("type"); t != "" {
		query = query.Where("type = ?", t)
	}
	if subtype := c.Query("subtype"); subtype != "" {
		query = query.Where("subtype = ?", subtype)
	}
	if roomID := c.Query("room_id"); roomID != "" {
		query = query.Where("room_id = ?", roomID)
	}
	if start := c.Query("start_date"); start != "" {
		query = query.Where("bill_date >= ?", start)
	}
	if end := c.Query("end_date"); end != "" {
		query = query.Where("bill_date <= ?", end)
	}
	query.Order("bill_date desc, created_at desc").Find(&bills)
	c.JSON(http.StatusOK, gin.H{"bills": bills})
}

func (h *BillHandler) Create(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
	var req CreateBillReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if req.Type != "income" && req.Type != "expense" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "类型必须为 income 或 expense"})
		return
	}
	if req.Subtype == "其他" && req.Description == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "子类型为其他时，备注不能为空"})
		return
	}
	userID, _ := c.Get("user_id")
	billNo := fmt.Sprintf("B%s%04d", utils.Now().Format("20060102150405"), utils.Now().UnixMilli()%10000)
	bill := models.Bill{
		BillNo:      billNo,
		Type:        req.Type,
		Subtype:     req.Subtype,
		Amount:      req.Amount,
		BuildingID:  bid,
		RoomID:      req.RoomID,
		Description: req.Description,
		BillDate:    req.BillDate,
		CreatedBy:   userID.(uint),
	}
	if err := h.DB.Create(&bill).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建账单失败"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"bill": bill})
}

func (h *BillHandler) Update(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
	id := c.Param("id")
	var bill models.Bill
	if err := h.DB.Where("id = ? AND building_id = ?", id, bid).First(&bill).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "账单不存在"})
		return
	}
	var req UpdateBillReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if req.Amount == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写修改金额"})
		return
	}
	if req.ModifyReason == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写修改原因"})
		return
	}
	oldAmount := bill.Amount
	newAmount := *req.Amount
	updates := map[string]interface{}{
		"amount": newAmount,
	}
	oldDesc := bill.Description
	updates["description"] = fmt.Sprintf("%s | 修改原因 %s,金额从 %.2f 变为 %.2f", oldDesc, req.ModifyReason, oldAmount, newAmount)
	h.DB.Model(&bill).Updates(updates)
	c.JSON(http.StatusOK, gin.H{"bill": bill})
}

func (h *BillHandler) Delete(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
	id := c.Param("id")
	var bill models.Bill
	if err := h.DB.Where("id = ? AND building_id = ?", id, bid).First(&bill).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "账单不存在"})
		return
	}
	h.DB.Delete(&bill)
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

func (h *BillHandler) Stats(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
	month := c.Query("month")
	year := c.Query("year")

	type StatRow struct {
		Type   string  `json:"type"`
		Subtype string  `json:"subtype"`
		Total  float64 `json:"total"`
	}
	var rows []StatRow
	query := h.DB.Model(&models.Bill{}).
		Select("type, subtype, SUM(amount) as total").
		Where("building_id = ?", bid).
		Group("type, subtype").
		Order("type, subtype")

	if year != "" {
		query = query.Where("DATE_FORMAT(bill_date, '%Y') = ?", year)
	} else if month != "" {
		query = query.Where("DATE_FORMAT(bill_date, '%Y-%m') = ?", month)
	} else {
		month = utils.Now().Format("2006-01")
		query = query.Where("DATE_FORMAT(bill_date, '%Y-%m') = ?", month)
	}
	query.Find(&rows)

	var totalIncome, totalExpense float64
	incomeDetail := []gin.H{}
	expenseDetail := []gin.H{}
	for _, r := range rows {
		item := gin.H{"subtype": r.Subtype, "total": r.Total}
		if r.Type == "income" {
			totalIncome += r.Total
			incomeDetail = append(incomeDetail, item)
		} else {
			totalExpense += r.Total
			expenseDetail = append(expenseDetail, item)
		}
	}
	resp := gin.H{
		"total_income":   totalIncome,
		"total_expense":  totalExpense,
		"net_profit":     totalIncome - totalExpense,
		"income_detail":  incomeDetail,
		"expense_detail": expenseDetail,
	}
	if year != "" {
		resp["year"] = year
	} else {
		resp["month"] = month
	}
	c.JSON(http.StatusOK, resp)
}

func (h *BillHandler) Trend(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
	years := c.Query("years")
	if years == "" {
		years = "2"
	}
	endYear := utils.Now().Year()
	startYear := endYear - 2 + 1
	if y, err := time.Parse("2006", years); err == nil {
		startYear = y.Year()
		endYear = utils.Now().Year()
	}

	type MonthlyStat struct {
		Month string  `json:"month"`
		Type  string  `json:"type"`
		Total float64 `json:"total"`
	}
	var rows []MonthlyStat
	h.DB.Model(&models.Bill{}).
		Select("DATE_FORMAT(bill_date, '%Y-%m') as month, type, SUM(amount) as total").
		Where("building_id = ?", bid).
		Where("bill_date >= ?", fmt.Sprintf("%d-01-01", startYear)).
		Where("bill_date <= ?", fmt.Sprintf("%d-12-31", endYear)).
		Group("month, type").
		Order("month, type").
		Find(&rows)

	type MonthData struct {
		Month   string  `json:"month"`
		Income  float64 `json:"income"`
		Expense float64 `json:"expense"`
		Profit  float64 `json:"profit"`
	}
	monthMap := make(map[string]*MonthData)
	for _, r := range rows {
		if _, ok := monthMap[r.Month]; !ok {
			monthMap[r.Month] = &MonthData{Month: r.Month}
		}
		if r.Type == "income" {
			monthMap[r.Month].Income = r.Total
		} else {
			monthMap[r.Month].Expense = r.Total
		}
		monthMap[r.Month].Profit = monthMap[r.Month].Income - monthMap[r.Month].Expense
	}

	result := []MonthData{}
	start := time.Date(startYear, 1, 1, 0, 0, 0, 0, time.Local)
	end := time.Date(endYear, 12, 1, 0, 0, 0, 0, time.Local)
	for d := start; !d.After(end); d = d.AddDate(0, 1, 0) {
		key := d.Format("2006-01")
		if md, ok := monthMap[key]; ok {
			result = append(result, *md)
		} else {
			result = append(result, MonthData{Month: key})
		}
	}

	type Growth struct {
		Month      string   `json:"month"`
		IncomeMoM  *float64 `json:"income_mom"`
		IncomeYoY  *float64 `json:"income_yoy"`
		ExpenseMoM *float64 `json:"expense_mom"`
		ExpenseYoY *float64 `json:"expense_yoy"`
	}
	var growthList []Growth
	for i, md := range result {
		g := Growth{Month: md.Month}
		if i > 0 {
			prev := result[i-1]
			if prev.Income > 0 {
				v := (md.Income - prev.Income) / prev.Income * 100
				g.IncomeMoM = &v
			}
			if prev.Expense > 0 {
				v := (md.Expense - prev.Expense) / prev.Expense * 100
				g.ExpenseMoM = &v
			}
		}
		if i >= 12 {
			prev := result[i-12]
			if prev.Income > 0 {
				v := (md.Income - prev.Income) / prev.Income * 100
				g.IncomeYoY = &v
			}
			if prev.Expense > 0 {
				v := (md.Expense - prev.Expense) / prev.Expense * 100
				g.ExpenseYoY = &v
			}
		}
		growthList = append(growthList, g)
	}

	c.JSON(http.StatusOK, gin.H{
		"months": result,
		"growth": growthList,
	})
}

func AutoCreateMonthlyRentBills(db *gorm.DB) {
	now := utils.Now()
	year, month, _ := now.Date()
	firstDay := time.Date(year, month, 1, 0, 0, 0, 0, now.Location())
	lastDay := firstDay.AddDate(0, 1, -1)
	monthStr := firstDay.Format("2006-01")

	var buildings []models.Building
	db.Where("status = ?", "active").Find(&buildings)

	for _, building := range buildings {
		var contracts []models.RentalContract
		db.Where("status = ? AND building_id = ?", "active", building.ID).Preload("Room").Find(&contracts)

		for _, contract := range contracts {
			startDate, err := time.Parse("2006-01-02", contract.StartDate)
			if err != nil {
				continue
			}
			endDate, err := time.Parse("2006-01-02", contract.EndDate)
			if err != nil {
				continue
			}

			billStart := startDate
			if startDate.Before(firstDay) {
				billStart = firstDay
			}
			billEnd := endDate
			if endDate.After(lastDay) {
				billEnd = lastDay
			}
			if billStart.After(billEnd) {
				continue
			}

			var count int64
			db.Model(&models.Bill{}).
				Where("building_id = ? AND room_id = ? AND subtype = ? AND DATE_FORMAT(bill_date, '%Y-%m') = ?",
					building.ID, contract.RoomID, "租金", monthStr).
				Count(&count)
			if count > 0 {
				continue
			}

			daysInMonth := lastDay.Day()
			days := int(billEnd.Sub(billStart).Hours()/24) + 1
			amount := contract.RentPrice * float64(days) / float64(daysInMonth)
			amount = math.Round(amount*100) / 100
			if amount <= 0 {
				continue
			}

			billNo := fmt.Sprintf("B%s%04d", utils.Now().Format("20060102150405"), utils.Now().UnixMilli()%10000)
			roomNumber := contract.Room.RoomNumber
			bill := models.Bill{
				BillNo:      billNo,
				Type:        "income",
				Subtype:     "租金",
				Amount:      amount,
				BuildingID:  building.ID,
				RoomID:      &contract.RoomID,
				Description: fmt.Sprintf("房间 %s %s 租金（%s至%s）", roomNumber, monthStr, billStart.Format("2006-01-02"), billEnd.Format("2006-01-02")),
				BillDate:    billStart.Format("2006-01-02"),
				CreatedBy:   1,
			}
			if err := db.Create(&bill).Error; err != nil {
				log.Printf("自动创建月度租金账单失败: %v", err)
			}
		}
	}
}
