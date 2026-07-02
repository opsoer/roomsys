package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"rental-server/logger"
	"rental-server/models"
	"rental-server/services"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DividendHandler struct {
	DB              *gorm.DB
	DividendService *services.DividendService
}

func (h *DividendHandler) List(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	dividends, err := h.DividendService.List(bid)
	if err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("查询分红记录失败")
		utils.Error(c, http.StatusInternalServerError, "查询分红记录失败")
		return
	}
	logger.Log.Debug().Uint("building_id", bid).Int("count", len(dividends)).Msg("查询分红记录")
	utils.Success(c, gin.H{"dividends": dividends})
}

func (h *DividendHandler) Calculate(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	month := c.Query("month")
	if month == "" {
		month = utils.Now().AddDate(0, -1, 0).Format("2006-01")
	}
	shareholders, err := h.DividendService.GetShareholders(bid)
	if err != nil || len(shareholders) == 0 {
		logger.Log.Warn().Uint("building_id", bid).Msg("分红计算失败: 未配置股东")
		utils.Error(c, http.StatusBadRequest, "请先配置股东信息")
		return
	}
	summary := utils.QueryMonthlyFinance(h.DB, bid, month)
	netProfit := summary.NetProfit
	if netProfit <= 0 {
		logger.Log.Info().Uint("building_id", bid).Str("month", month).Float64("net_profit", netProfit).Msg("分红计算: 无净利润")
		utils.SuccessWithMsg(c, fmt.Sprintf("%s 无净利润，不分红", month), gin.H{
			"month":         month,
			"total_income":  summary.TotalIncome,
			"total_expense": summary.TotalExpense,
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
		"total_income":  summary.TotalIncome,
		"total_expense": summary.TotalExpense,
		"net_profit":    netProfit,
		"results":       results,
	})
}

type SettleDividendReq struct {
	Month string `json:"month" binding:"required"`
}

func (h *DividendHandler) Settle(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	userID := c.GetUint("user_id")
	var req SettleDividendReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Warn().Uint("building_id", bid).Msg("分红结算请求参数错误")
		utils.Error(c, http.StatusBadRequest, "请选择结算月份")
		return
	}
	shareholders, err := h.DividendService.GetShareholders(bid)
	if err != nil || len(shareholders) == 0 {
		logger.Log.Warn().Uint("building_id", bid).Msg("分红结算失败: 未配置股东")
		utils.Error(c, http.StatusBadRequest, "请先配置股东信息")
		return
	}
	summary := utils.QueryMonthlyFinance(h.DB, bid, req.Month)
	netProfit := summary.NetProfit

	tx := h.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		} else if tx.Error != nil {
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
			TotalIncome:    summary.TotalIncome,
			TotalExpense:   summary.TotalExpense,
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
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	shareholders, err := h.DividendService.GetShareholders(bid)
	if err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("查询股东列表失败")
		utils.Error(c, http.StatusInternalServerError, "查询股东列表失败")
		return
	}
	utils.Success(c, gin.H{"shareholders": shareholders})
}

type CreateShareholderReq struct {
	Name       string  `json:"name" binding:"required"`
	ShareRatio float64 `json:"share_ratio" binding:"required"`
}

func (h *DividendHandler) CreateShareholder(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
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
	if err := h.DividendService.CreateShareholder(&sh); err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("创建股东失败")
		utils.Error(c, http.StatusInternalServerError, "创建失败")
		return
	}
	logger.Log.Info().Uint("shareholder_id", sh.ID).Str("name", sh.Name).Float64("ratio", sh.ShareRatio).Uint("building_id", bid).Msg("股东创建成功")
	utils.Created(c, "创建成功", gin.H{"shareholder": sh})
}

func (h *DividendHandler) UpdateShareholder(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	id := c.Param("id")
	shareholderID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的股东ID")
		return
	}
	var req struct {
		Name       string  `json:"name"`
		ShareRatio float64 `json:"share_ratio"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.ShareRatio > 0 {
		updates["share_ratio"] = req.ShareRatio
	}
	if err := h.DividendService.UpdateShareholder(uint(shareholderID), updates); err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("更新股东失败")
		utils.Error(c, http.StatusInternalServerError, "更新失败")
		return
	}
	utils.SuccessWithMsg(c, "更新成功", nil)
}

func (h *DividendHandler) DeleteShareholder(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	id := c.Param("id")
	shareholderID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的股东ID")
		return
	}
	if err := h.DividendService.DeleteShareholder(uint(shareholderID)); err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("删除股东失败")
		utils.Error(c, http.StatusInternalServerError, "删除失败")
		return
	}
	utils.SuccessWithMsg(c, "删除成功", nil)
}

func (h *DividendHandler) Predict(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	months := 3
	if m := c.Query("months"); m != "" {
		if parsed, err := strconv.Atoi(m); err == nil {
			months = parsed
		}
	}
	predictions, err := h.DividendService.Predict(bid, months)
	if err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("获取预测数据失败")
		utils.Error(c, http.StatusInternalServerError, "获取预测失败")
		return
	}
	utils.Success(c, gin.H{"predictions": predictions})
}
