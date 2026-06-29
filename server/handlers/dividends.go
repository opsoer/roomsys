package handlers

import (
	"fmt"
	"net/http"
	"time"

	"rental-server/logger"
	"rental-server/models"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DividendHandler struct {
	DB *gorm.DB
}

func (h *DividendHandler) List(c *gin.Context) {
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
	var dividends []models.Dividend
	if err := h.DB.Where("building_id = ?", bid).
		Preload("Shareholder").
		Order("settle_month desc, created_at desc").
		Find(&dividends).Error; err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("查询分红记录失败")
	}
	logger.Log.Debug().Uint("building_id", bid).Int("count", len(dividends)).Msg("查询分红记录")
	utils.Success(c, gin.H{"dividends": dividends})
}

func (h *DividendHandler) Calculate(c *gin.Context) {
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
	if month == "" {
		month = utils.Now().AddDate(0, -1, 0).Format("2006-01")
	}
	var shareholders []models.Shareholder
	if err := h.DB.Where("building_id = ?", bid).Find(&shareholders).Error; err != nil || len(shareholders) == 0 {
		logger.Log.Warn().Uint("building_id", bid).Msg("分红计算失败: 未配置股东")
		utils.Error(c, http.StatusBadRequest, "请先配置股东信息")
		return
	}
	type TotalRow struct {
		Type  string
		Total float64
	}
	var rows []TotalRow
	if err := h.DB.Model(&models.Bill{}).
		Select("type, SUM(amount) as total").
		Where("building_id = ? AND DATE_FORMAT(bill_date, '%Y-%m') = ?", bid, month).
		Group("type").
		Find(&rows).Error; err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("查询分红账单失败")
	}
	var totalIncome, totalExpense float64
	for _, r := range rows {
		if r.Type == "income" {
			totalIncome = r.Total
		} else {
			totalExpense = r.Total
		}
	}
	netProfit := totalIncome - totalExpense
	if netProfit <= 0 {
		logger.Log.Info().Uint("building_id", bid).Str("month", month).Float64("net_profit", netProfit).Msg("分红计算: 无净利润")
		utils.SuccessWithMsg(c, fmt.Sprintf("%s 无净利润，不分红", month), gin.H{
			"month":         month,
			"total_income":  totalIncome,
			"total_expense": totalExpense,
			"net_profit":    netProfit,
			"results":       []gin.H{},
		})
		return
	}
	type Result struct {
		models.Shareholder
		DividendAmount float64 `json:"dividend_amount"`
	}
	var results []Result
	for _, s := range shareholders {
		amount := netProfit * s.ShareRatio / 100
		results = append(results, Result{
			Shareholder:    s,
			DividendAmount: amount,
		})
	}
	logger.Log.Info().Uint("building_id", bid).Str("month", month).Float64("net_profit", netProfit).Int("shareholders", len(results)).Msg("分红预览计算完成")
	utils.Success(c, gin.H{
		"month":         month,
		"total_income":  totalIncome,
		"total_expense": totalExpense,
		"net_profit":    netProfit,
		"results":       results,
	})
}

type SettleDividendReq struct {
	Month string `json:"month" binding:"required"`
}

func (h *DividendHandler) Settle(c *gin.Context) {
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
	userID := c.GetUint("user_id")
	var req SettleDividendReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Warn().Uint("building_id", bid).Msg("分红结算请求参数错误")
		utils.Error(c, http.StatusBadRequest, "请选择结算月份")
		return
	}
	var shareholders []models.Shareholder
	if err := h.DB.Where("building_id = ?", bid).Find(&shareholders).Error; err != nil || len(shareholders) == 0 {
		logger.Log.Warn().Uint("building_id", bid).Msg("分红结算失败: 未配置股东")
		utils.Error(c, http.StatusBadRequest, "请先配置股东信息")
		return
	}
	type TotalRow struct {
		Type  string
		Total float64
	}
	var rows []TotalRow
	if err := h.DB.Model(&models.Bill{}).
		Select("type, SUM(amount) as total").
		Where("building_id = ? AND DATE_FORMAT(bill_date, '%Y-%m') = ?", bid, req.Month).
		Group("type").
		Find(&rows).Error; err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("查询分红结算账单失败")
	}
	var totalIncome, totalExpense float64
	for _, r := range rows {
		if r.Type == "income" {
			totalIncome = r.Total
		} else {
			totalExpense = r.Total
		}
	}
	netProfit := totalIncome - totalExpense

	tx := h.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Where("building_id = ? AND settle_month = ?", bid, req.Month).Delete(&models.Dividend{}).Error; err != nil {
		tx.Rollback()
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("删除旧分红记录失败")
		utils.Error(c, http.StatusInternalServerError, "结算失败")
		return
	}

	created := []gin.H{}
	for _, s := range shareholders {
		amount := float64(0)
		if netProfit > 0 {
			amount = netProfit * s.ShareRatio / 100
		}
		dividend := models.Dividend{
			BuildingID:     bid,
			SettleMonth:    req.Month,
			TotalIncome:    totalIncome,
			TotalExpense:   totalExpense,
			NetProfit:      netProfit,
			ShareholderID:  s.ID,
			DividendAmount: amount,
			SettledBy:      userID,
		}
		if err := tx.Create(&dividend).Error; err != nil {
			tx.Rollback()
			logger.Log.Error().Err(err).Uint("building_id", bid).Msg("创建分红记录失败")
			utils.Error(c, http.StatusInternalServerError, "结算失败")
			return
		}
		created = append(created, gin.H{
			"shareholder":     s.Name,
			"share_ratio":     s.ShareRatio,
			"dividend_amount": amount,
		})
	}

	tx.Commit()

	logger.Log.Info().
		Uint("building_id", bid).
		Str("month", req.Month).
		Float64("net_profit", netProfit).
		Int("shareholders", len(created)).
		Msg("分红已结算")
	utils.Created(c, fmt.Sprintf("%s 分红已结算", req.Month), gin.H{
		"month":      req.Month,
		"net_profit": netProfit,
		"results":    created,
	})
}

func (h *DividendHandler) GetShareholders(c *gin.Context) {
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
	var shareholders []models.Shareholder
	if err := h.DB.Where("building_id = ?", bid).Find(&shareholders).Error; err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("查询股东列表失败")
	}
	utils.Success(c, gin.H{"shareholders": shareholders})
}

type CreateShareholderReq struct {
	Name       string  `json:"name" binding:"required"`
	ShareRatio float64 `json:"share_ratio" binding:"required"`
}

func (h *DividendHandler) CreateShareholder(c *gin.Context) {
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
	var req CreateShareholderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Warn().Uint("building_id", bid).Msg("创建股东请求参数错误")
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	sh := models.Shareholder{
		BuildingID: bid,
		Name:       req.Name,
		ShareRatio: req.ShareRatio,
	}
	if err := h.DB.Create(&sh).Error; err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("创建股东失败")
		utils.Error(c, http.StatusInternalServerError, "创建失败")
		return
	}
	logger.Log.Info().Uint("shareholder_id", sh.ID).Str("name", sh.Name).Float64("ratio", sh.ShareRatio).Uint("building_id", bid).Msg("股东创建成功")
	utils.Created(c, "创建成功", gin.H{"shareholder": sh})
}

func (h *DividendHandler) UpdateShareholder(c *gin.Context) {
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
	var sh models.Shareholder
	if err := h.DB.Where("id = ? AND building_id = ?", id, bid).First(&sh).Error; err != nil {
		logger.Log.Warn().Str("id", id).Uint("building_id", bid).Msg("更新股东失败: 不存在")
		utils.Error(c, http.StatusNotFound, "股东不存在")
		return
	}
	var req CreateShareholderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Warn().Uint("shareholder_id", sh.ID).Msg("更新股东请求参数错误")
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	if err := h.DB.Model(&sh).Updates(map[string]interface{}{
		"name":        req.Name,
		"share_ratio": req.ShareRatio,
	}).Error; err != nil {
		logger.Log.Error().Err(err).Uint("shareholder_id", sh.ID).Msg("更新股东失败")
		utils.Error(c, http.StatusInternalServerError, "更新失败")
		return
	}
	logger.Log.Info().Uint("shareholder_id", sh.ID).Str("name", req.Name).Float64("ratio", req.ShareRatio).Msg("股东信息更新成功")
	utils.Success(c, gin.H{"shareholder": sh})
}

func (h *DividendHandler) DeleteShareholder(c *gin.Context) {
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
	var sh models.Shareholder
	if err := h.DB.Where("id = ? AND building_id = ?", id, bid).First(&sh).Error; err != nil {
		logger.Log.Warn().Str("id", id).Uint("building_id", bid).Msg("删除股东失败: 不存在")
		utils.Error(c, http.StatusNotFound, "股东不存在")
		return
	}
	if err := h.DB.Delete(&sh).Error; err != nil {
		logger.Log.Error().Err(err).Uint("shareholder_id", sh.ID).Msg("删除股东失败")
		utils.Error(c, http.StatusInternalServerError, "删除失败")
		return
	}
	logger.Log.Info().Uint("shareholder_id", sh.ID).Str("name", sh.Name).Msg("股东已删除")
	utils.SuccessWithMsg(c, "删除成功", nil)
}

func (h *DividendHandler) Predict(c *gin.Context) {
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
	monthsStr := c.DefaultQuery("months", "3")
	m := 3
	fmt.Sscanf(monthsStr, "%d", &m)
	if m < 1 {
		m = 1
	}
	if m > 12 {
		m = 12
	}

	now := utils.Now()
	type Prediction struct {
		Month     string  `json:"month"`
		Rent      float64 `json:"rent"`
		Deposit   float64 `json:"deposit"`
		Available float64 `json:"available"`
	}
	predictions := []Prediction{}

	for i := 0; i < m; i++ {
		month := now.AddDate(0, i, 0).Format("2006-01")
		monthStart := now.AddDate(0, i, 0).Format("2006-01-02")
		firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, i, 0)
		monthEnd := firstOfMonth.AddDate(0, 1, -1).Format("2006-01-02")

		var totalRent float64
		h.DB.Model(&models.RentalContract{}).
			Select("COALESCE(SUM(rent_price), 0)").
			Where("building_id = ? AND status = ?", bid, "active").
			Where("start_date <= ?", monthEnd).
			Where("end_date >= ? OR end_date = '' OR end_date IS NULL", monthStart).
			Scan(&totalRent)

		var newDeposits float64
		h.DB.Model(&models.RentalContract{}).
			Select("COALESCE(SUM(deposit), 0)").
			Where("building_id = ? AND status = ?", bid, "active").
			Where("start_date >= ?", monthStart).
			Where("start_date <= ?", monthEnd).
			Scan(&newDeposits)

		predictions = append(predictions, Prediction{
			Month:     month,
			Rent:      totalRent,
			Deposit:   newDeposits,
			Available: totalRent,
		})
	}

	logger.Log.Debug().Uint("building_id", bid).Int("months", m).Msg("分红预测计算完成")
	utils.Success(c, gin.H{"predictions": predictions})
}
