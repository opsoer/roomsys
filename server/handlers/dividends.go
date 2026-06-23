package handlers

import (
	"fmt"
	"net/http"

	"rental-server/models"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DividendHandler struct {
	DB *gorm.DB
}

func (h *DividendHandler) List(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
	var dividends []models.Dividend
	h.DB.Where("building_id = ?", bid).
		Preload("Shareholder").
		Order("settle_month desc, created_at desc").
		Find(&dividends)
	c.JSON(http.StatusOK, gin.H{"dividends": dividends})
}

func (h *DividendHandler) Calculate(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
	month := c.Query("month")
	if month == "" {
		month = utils.Now().AddDate(0, -1, 0).Format("2006-01")
	}
	var shareholders []models.Shareholder
	if err := h.DB.Where("building_id = ?", bid).Find(&shareholders).Error; err != nil || len(shareholders) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请先配置股东信息"})
		return
	}
	type TotalRow struct {
		Type  string
		Total float64
	}
	var rows []TotalRow
	h.DB.Model(&models.Bill{}).
		Select("type, SUM(amount) as total").
		Where("building_id = ? AND DATE_FORMAT(bill_date, '%Y-%m') = ?", bid, month).
		Group("type").
		Find(&rows)
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
		c.JSON(http.StatusOK, gin.H{
			"message":       fmt.Sprintf("%s 无净利润，不分红", month),
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
	c.JSON(http.StatusOK, gin.H{
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
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
	var req SettleDividendReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择结算月份"})
		return
	}
	var shareholders []models.Shareholder
	if err := h.DB.Where("building_id = ?", bid).Find(&shareholders).Error; err != nil || len(shareholders) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请先配置股东信息"})
		return
	}
	type TotalRow struct {
		Type  string
		Total float64
	}
	var rows []TotalRow
	h.DB.Model(&models.Bill{}).
		Select("type, SUM(amount) as total").
		Where("building_id = ? AND DATE_FORMAT(bill_date, '%Y-%m') = ?", bid, req.Month).
		Group("type").
		Find(&rows)
	var totalIncome, totalExpense float64
	for _, r := range rows {
		if r.Type == "income" {
			totalIncome = r.Total
		} else {
			totalExpense = r.Total
		}
	}
	netProfit := totalIncome - totalExpense
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
		}
		if err := h.DB.Create(&dividend).Error; err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("%s 已结算过", req.Month)})
			return
		}
		created = append(created, gin.H{
			"shareholder":     s.Name,
			"share_ratio":     s.ShareRatio,
			"dividend_amount": amount,
		})
	}
	c.JSON(http.StatusCreated, gin.H{
		"message":    fmt.Sprintf("%s 分红已结算", req.Month),
		"month":      req.Month,
		"net_profit": netProfit,
		"results":    created,
	})
}

func (h *DividendHandler) GetShareholders(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
	var shareholders []models.Shareholder
	h.DB.Where("building_id = ?", bid).Find(&shareholders)
	c.JSON(http.StatusOK, gin.H{"shareholders": shareholders})
}

type CreateShareholderReq struct {
	Name       string  `json:"name" binding:"required"`
	ShareRatio float64 `json:"share_ratio" binding:"required"`
}

func (h *DividendHandler) CreateShareholder(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
	var req CreateShareholderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	sh := models.Shareholder{
		BuildingID: bid,
		Name:       req.Name,
		ShareRatio: req.ShareRatio,
	}
	if err := h.DB.Create(&sh).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"shareholder": sh})
}

func (h *DividendHandler) UpdateShareholder(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
	id := c.Param("id")
	var sh models.Shareholder
	if err := h.DB.Where("id = ? AND building_id = ?", id, bid).First(&sh).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "股东不存在"})
		return
	}
	var req CreateShareholderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	h.DB.Model(&sh).Updates(map[string]interface{}{
		"name":        req.Name,
		"share_ratio": req.ShareRatio,
	})
	c.JSON(http.StatusOK, gin.H{"shareholder": sh})
}

func (h *DividendHandler) DeleteShareholder(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
	id := c.Param("id")
	var sh models.Shareholder
	if err := h.DB.Where("id = ? AND building_id = ?", id, bid).First(&sh).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "股东不存在"})
		return
	}
	h.DB.Delete(&sh)
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

func (h *DividendHandler) Predict(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
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
		monthEnd := now.AddDate(0, i+1, -0).Format("2006-01-02")

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

		available := totalRent - newDeposits
		if available < 0 {
			available = 0
		}

		predictions = append(predictions, Prediction{
			Month:     month,
			Rent:      totalRent,
			Deposit:   newDeposits,
			Available: available,
		})
	}

	c.JSON(http.StatusOK, gin.H{"predictions": predictions})
}
