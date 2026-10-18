// Package handlers 提供数据看板、趋势分析等统计功能的 HTTP 接口
package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"rental-server/services"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// StatsHandler 统计处理器
type StatsHandler struct {
	DB           *gorm.DB
	StatsService *services.StatsService
}

// Overview 获取全局概览统计（PV、UV、转化率、空置率、楼栋排行）
func (h *StatsHandler) Overview(c *gin.Context) {
	data, err := utils.CacheGetOrSet("stats_overview", 5*time.Minute, func() (interface{}, error) {
		return h.StatsService.Overview()
	})
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "查询失败")
		return
	}
	utils.Success(c, gin.H{"overview": data})
}

// Trend 获取PV/UV趋势数据
func (h *StatsHandler) Trend(c *gin.Context) {
	daysStr := c.DefaultQuery("days", "30")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days <= 0 || days > 365 {
		days = 30
	}
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	cutoff := todayStart.AddDate(0, 0, -(days - 1))
	cacheKey := fmt.Sprintf("stats_trend_%d", days)

	data, err := utils.CacheGetOrSet(cacheKey, 5*time.Minute, func() (interface{}, error) {
		return h.StatsService.Trend(cutoff, todayStart)
	})
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "查询失败")
		return
	}
	utils.Success(c, gin.H{"trend": data})
}

// PriceReference 获取租金价格参考（按区域和户型分组）
func (h *StatsHandler) PriceReference(c *gin.Context) {
	data, err := utils.CacheGetOrSet("stats_price_ref", 5*time.Minute, func() (interface{}, error) {
		return h.StatsService.PriceReference()
	})
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "查询失败")
		return
	}
	utils.Success(c, gin.H{"price_reference": data})
}

// BuildingDetail 获取单个楼栋的详细统计数据
func (h *StatsHandler) BuildingDetail(c *gin.Context) {
	idStr := c.Param("id")
	buildingID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的公寓ID")
		return
	}
	result, err := h.StatsService.BuildingDetail(uint(buildingID))
	if err != nil {
		utils.Error(c, http.StatusNotFound, "公寓不存在")
		return
	}
	utils.Success(c, gin.H{"building_stats": result})
}

// ===== 楼栋管理员 API（自动 scope 到自己的 building_id）=====

// MyBuildingStats 获取当前管理员所属楼栋的统计概览
func (h *StatsHandler) MyBuildingStats(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	cacheKey := "my_stats_" + strconv.Itoa(int(bid))

	data, err := utils.CacheGetOrSet(cacheKey, 5*time.Minute, func() (interface{}, error) {
		return h.StatsService.MyBuildingStats(bid)
	})
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "查询失败")
		return
	}
	utils.Success(c, gin.H{"stats": data})
}

// MyBuildingTrend 获取当前管理员所属楼栋的趋势数据
func (h *StatsHandler) MyBuildingTrend(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	daysStr := c.DefaultQuery("days", "30")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days <= 0 || days > 365 {
		days = 30
	}
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	cutoff := todayStart.AddDate(0, 0, -(days - 1))
	cacheKey := "my_trend_" + strconv.Itoa(int(bid)) + "_" + daysStr

	data, err := utils.CacheGetOrSet(cacheKey, 5*time.Minute, func() (interface{}, error) {
		return h.StatsService.MyBuildingTrend(bid, cutoff, todayStart)
	})
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "查询失败")
		return
	}
	utils.Success(c, gin.H{"trend": data})
}

// 公开埋点 API

// landlordViewReq 记录房东浏览的请求参数
type landlordViewReq struct {
	BuildingID uint `json:"building_id" binding:"required"`
}

// RecordLandlordView 记录房东浏览PV（公开埋点）
func (h *StatsHandler) RecordLandlordView(c *gin.Context) {
	var req landlordViewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	utils.RecordPageView(h.DB, "landlord_view", req.BuildingID, req.BuildingID, utils.GetRealIP(c))
	c.JSON(http.StatusOK, utils.APIResponse{Code: 0, Message: "ok"})
}
