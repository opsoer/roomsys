package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"
	"time"

	"rental-server/logger"
	"rental-server/models"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func isMonthSettled(db *gorm.DB, buildingID uint, billDate string) bool {
	if len(billDate) < 7 {
		return false
	}
	month := billDate[:7]
	var count int64
	db.Model(&models.Dividend{}).
		Where("building_id = ? AND settle_month = ?", buildingID, month).
		Count(&count)
	return count > 0
}

func appendModification(desc, modLog string, maxMods int) string {
	const sep = " | "
	parts := strings.SplitN(desc, sep, maxMods+1)
	if len(parts) > maxMods {
		parts = parts[:maxMods]
	}
	base := parts[0]
	for i := 1; i < len(parts); i++ {
		base += sep + parts[i]
	}
	return base + sep + modLog
}

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
	if err := query.Order("bill_date desc, created_at desc").Find(&bills).Error; err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("查询账单列表失败")
	}
	utils.Success(c, gin.H{"bills": bills})
}

func (h *BillHandler) Create(c *gin.Context) {
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
	var req CreateBillReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Warn().Uint("building_id", bid).Msg("创建账单请求参数错误")
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	if req.Type != "income" && req.Type != "expense" {
		logger.Log.Warn().Uint("building_id", bid).Str("type", req.Type).Msg("创建账单失败: 类型无效")
		utils.Error(c, http.StatusBadRequest, "类型必须为 income 或 expense")
		return
	}
	if isMonthSettled(h.DB, bid, req.BillDate) {
		logger.Log.Warn().Uint("building_id", bid).Str("bill_date", req.BillDate).Msg("创建账单失败: 该月已结算")
		utils.Error(c, http.StatusBadRequest, "该月已结算分红，无法创建账单")
		return
	}
	if req.Subtype == "其他" && req.Description == "" {
		logger.Log.Warn().Uint("building_id", bid).Msg("创建账单失败: 其他类型需备注")
		utils.Error(c, http.StatusBadRequest, "子类型为其他时，备注不能为空")
		return
	}
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
	billNo := utils.GenerateBillNo()
	bill := models.Bill{
		BillNo:      billNo,
		Type:        req.Type,
		Subtype:     req.Subtype,
		Amount:      req.Amount,
		BuildingID:  bid,
		RoomID:      req.RoomID,
		Description: req.Description,
		BillDate:    req.BillDate,
		CreatedBy:   uid,
	}
	if err := h.DB.Create(&bill).Error; err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("创建账单数据库失败")
		utils.Error(c, http.StatusInternalServerError, "创建账单失败")
		return
	}
	logger.Log.Info().
		Uint("bill_id", bill.ID).
		Str("bill_no", billNo).
		Str("type", req.Type).
		Str("subtype", req.Subtype).
		Float64("amount", req.Amount).
		Uint("building_id", bid).
		Msg("账单创建成功")
	utils.Created(c, "创建成功", gin.H{"bill": bill})
}

func (h *BillHandler) Update(c *gin.Context) {
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
	id := c.Param("id")
	var bill models.Bill
	if err := h.DB.Where("id = ? AND building_id = ?", id, bid).First(&bill).Error; err != nil {
		logger.Log.Warn().Str("id", id).Uint("building_id", bid).Msg("更新账单失败: 账单不存在")
		utils.Error(c, http.StatusNotFound, "账单不存在")
		return
	}
	if isMonthSettled(h.DB, bid, bill.BillDate) {
		logger.Log.Warn().Uint("bill_id", bill.ID).Msg("更新账单失败: 该月已结算")
		utils.Error(c, http.StatusBadRequest, "该月已结算分红，无法修改账单")
		return
	}
	var req UpdateBillReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Warn().Uint("bill_id", bill.ID).Msg("更新账单请求参数错误")
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	if req.ModifyReason == "" {
		logger.Log.Warn().Uint("bill_id", bill.ID).Msg("更新账单失败: 缺少修改原因")
		utils.Error(c, http.StatusBadRequest, "请填写修改原因")
		return
	}
	oldAmount := bill.Amount
	newAmount := bill.Amount
	if req.Amount != nil {
		newAmount = *req.Amount
	}
	updates := map[string]interface{}{
		"amount": newAmount,
	}
	modLog := fmt.Sprintf("[%s] 修改原因: %s", utils.Now().Format("2006-01-02 15:04:05"), req.ModifyReason)
	if req.Amount != nil {
		modLog += fmt.Sprintf(", 金额: %.2f -> %.2f", oldAmount, newAmount)
	}
	const maxMods = 10
	updates["description"] = appendModification(bill.Description, modLog, maxMods)
	if err := h.DB.Model(&bill).Updates(updates).Error; err != nil {
		logger.Log.Error().Err(err).Uint("bill_id", bill.ID).Msg("更新账单失败")
		utils.Error(c, http.StatusInternalServerError, "更新失败")
		return
	}
	logger.Log.Info().
		Uint("bill_id", bill.ID).
		Str("bill_no", bill.BillNo).
		Float64("old_amount", oldAmount).
		Float64("new_amount", newAmount).
		Str("modify_reason", req.ModifyReason).
		Msg("账单更新成功")
	utils.Success(c, gin.H{"bill": bill})
}

func (h *BillHandler) ExportCSV(c *gin.Context) {
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

	var bills []models.Bill
	query := h.DB.Preload("Room").Where("building_id = ?", bid)
	if t := c.Query("type"); t != "" {
		query = query.Where("type = ?", t)
	}
	if subtype := c.Query("subtype"); subtype != "" {
		query = query.Where("subtype = ?", subtype)
	}
	if start := c.Query("start_date"); start != "" {
		query = query.Where("bill_date >= ?", start)
	}
	if end := c.Query("end_date"); end != "" {
		query = query.Where("bill_date <= ?", end)
	}
	if err := query.Order("bill_date desc, created_at desc").Find(&bills).Error; err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("导出账单失败")
		utils.Error(c, http.StatusInternalServerError, "导出失败")
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=bills.csv")
	writer := csv.NewWriter(c.Writer)
	writer.Write([]string{"账单编号", "类型", "子类型", "金额", "房间号", "日期", "备注", "创建时间"})
	for _, bill := range bills {
		roomNo := ""
		if bill.Room.ID > 0 {
			roomNo = bill.Room.RoomNumber
		}
		typeLabel := "收入"
		if bill.Type == "expense" {
			typeLabel = "支出"
		}
		writer.Write([]string{
			bill.BillNo,
			typeLabel,
			bill.Subtype,
			fmt.Sprintf("%.2f", bill.Amount),
			roomNo,
			bill.BillDate,
			bill.Description,
			bill.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	writer.Flush()
}

func (h *BillHandler) Delete(c *gin.Context) {
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
	id := c.Param("id")
	var bill models.Bill
	if err := h.DB.Where("id = ? AND building_id = ?", id, bid).First(&bill).Error; err != nil {
		logger.Log.Warn().Str("id", id).Uint("building_id", bid).Msg("删除账单失败: 账单不存在")
		utils.Error(c, http.StatusNotFound, "账单不存在")
		return
	}
	if isMonthSettled(h.DB, bid, bill.BillDate) {
		logger.Log.Warn().Uint("bill_id", bill.ID).Msg("删除账单失败: 该月已结算")
		utils.Error(c, http.StatusBadRequest, "该月已结算分红，无法删除账单")
		return
	}
	if err := h.DB.Delete(&bill).Error; err != nil {
		logger.Log.Error().Err(err).Uint("bill_id", bill.ID).Msg("删除账单失败")
		utils.Error(c, http.StatusInternalServerError, "删除失败")
		return
	}
	logger.Log.Info().Uint("bill_id", bill.ID).Str("bill_no", bill.BillNo).Uint("building_id", bid).Msg("账单已删除")
	utils.SuccessWithMsg(c, "删除成功", nil)
}

func (h *BillHandler) Stats(c *gin.Context) {
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
	if err := query.Find(&rows).Error; err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("查询账单统计失败")
	}

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
	fs := utils.NewMonthlyFinanceSummary(totalIncome, totalExpense)
	resp := gin.H{
		"total_income":   fs.TotalIncome,
		"total_expense":  fs.TotalExpense,
		"net_profit":     fs.NetProfit,
		"income_detail":  incomeDetail,
		"expense_detail": expenseDetail,
	}
	if year != "" {
		resp["year"] = year
	} else {
		resp["month"] = month
	}
	utils.Success(c, resp)
}

func (h *BillHandler) Trend(c *gin.Context) {
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
	if err := h.DB.Model(&models.Bill{}).
		Select("DATE_FORMAT(bill_date, '%Y-%m') as month, type, SUM(amount) as total").
		Where("building_id = ?", bid).
		Where("bill_date >= ?", fmt.Sprintf("%d-01-01", startYear)).
		Where("bill_date <= ?", fmt.Sprintf("%d-12-31", endYear)).
		Group("month, type").
		Order("month, type").
		Find(&rows).Error; err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("查询账单趋势失败")
	}

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

	utils.Success(c, gin.H{
		"months": result,
		"growth": growthList,
	})
}

func AutoCreateMonthlyRentBills(db *gorm.DB) {
	bm := utils.GetMonthBoundary(utils.Now())
	firstDay := bm.FirstDay
	lastDay := bm.LastDay
	monthStr := bm.Month

	var buildings []models.Building
	db.Where("status = ?", "active").Find(&buildings)

	createdCount := 0
	for _, building := range buildings {
		var contracts []models.RentalContract
		db.Where("status = ? AND building_id = ?", "active", building.ID).Preload("Room").Find(&contracts)

		tx := db.Begin()
		buildingCreated := 0
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

			if startDate.After(utils.Now()) {
				continue
			}

			var count int64
			tx.Model(&models.Bill{}).
				Where("building_id = ? AND room_id = ? AND subtype = ? AND DATE_FORMAT(bill_date, '%Y-%m') = ? AND description LIKE ?",
					building.ID, contract.RoomID, "租金", monthStr, "%合同ID:"+fmt.Sprintf("%d", contract.ID)+"%").
				Count(&count)
			if count > 0 {
				continue
			}

			amount := utils.CalcProratedAmount(contract.RentPrice, billStart, billEnd, lastDay.Day())
			if amount <= 0 {
				continue
			}

			billNo := utils.GenerateBillNo()
			roomNumber := contract.Room.RoomNumber
			bill := models.Bill{
				BillNo:      billNo,
				Type:        "income",
				Subtype:     "租金",
				Amount:      amount,
				BuildingID:  building.ID,
				RoomID:      &contract.RoomID,
				Description: fmt.Sprintf("房间 %s %s 租金（%s至%s）合同ID:%d", roomNumber, monthStr, billStart.Format("2006-01-02"), billEnd.Format("2006-01-02"), contract.ID),
				BillDate:    billStart.Format("2006-01-02"),
				CreatedBy:   1,
			}
			if err := tx.Create(&bill).Error; err != nil {
				tx.Rollback()
				logger.Log.Error().Err(err).Uint("building_id", building.ID).Uint("room_id", contract.RoomID).Msg("自动创建月度租金账单失败")
				buildingCreated = -1
				break
			}
			buildingCreated++
		}
		if buildingCreated >= 0 {
			tx.Commit()
			createdCount += buildingCreated
		}
	}
	if createdCount > 0 {
		logger.Log.Info().Str("month", monthStr).Int("count", createdCount).Msg("月度租金账单自动创建完成")
	} else {
		logger.Log.Debug().Str("month", monthStr).Msg("月度租金账单自动创建完成，无新账单")
	}
}
