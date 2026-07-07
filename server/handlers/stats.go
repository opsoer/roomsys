package handlers

import (
	"net/http"
	"strconv"
	"time"

	"rental-server/models"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type StatsHandler struct {
	DB *gorm.DB
}

type overviewData struct {
	TotalPV          int64         `json:"total_pv"`
	TotalUV          int64         `json:"total_uv"`
	TodayPV          int64         `json:"today_pv"`
	TodayUV          int64         `json:"today_uv"`
	TotalLandlordView int64        `json:"total_landlord_view"`
	ConversionRate   float64       `json:"conversion_rate"`
	VacancyRate      float64       `json:"vacancy_rate"`
	BuildingRank     []buildingRankItem `json:"building_rank"`
}

type buildingRankItem struct {
	BuildingID   uint   `json:"building_id"`
	BuildingName string `json:"building_name"`
	PV           int64  `json:"pv"`
	UV           int64  `json:"uv"`
	VacantCount  int64  `json:"vacant_count"`
	RoomCount    int64  `json:"room_count"`
	LandlordView int64  `json:"landlord_view"`
}

type trendItem struct {
	Date string `json:"date"`
	PV   int64  `json:"pv"`
	UV   int64  `json:"uv"`
}

type priceRefItem struct {
	District string `json:"district"`
	Layout   string `json:"layout"`
	MinPrice float64 `json:"min_price"`
	MaxPrice float64 `json:"max_price"`
	AvgPrice float64 `json:"avg_price"`
	Count    int64   `json:"count"`
}

type buildingDetailData struct {
	BuildingID     uint             `json:"building_id"`
	BuildingName   string           `json:"building_name"`
	PV             int64            `json:"pv"`
	UV             int64            `json:"uv"`
	LandlordView   int64            `json:"landlord_view"`
	ConversionRate float64          `json:"conversion_rate"`
	RoomRank       []roomRankItem   `json:"room_rank"`
}

type roomRankItem struct {
	RoomID     uint   `json:"room_id"`
	RoomNumber string `json:"room_number"`
	Layout     string `json:"layout"`
	PV         int64  `json:"pv"`
	Status     string `json:"status"`
}

func (h *StatsHandler) Overview(c *gin.Context) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	data, err := utils.CacheGetOrSet("stats_overview", 5*time.Minute, func() (interface{}, error) {
		var result overviewData

		h.DB.Model(&models.PageView{}).Select("COUNT(*)").Scan(&result.TotalPV)
		h.DB.Model(&models.PageView{}).Select("COUNT(DISTINCT ip)").Scan(&result.TotalUV)
		h.DB.Model(&models.PageView{}).Where("created_at >= ?", todayStart).Select("COUNT(*)").Scan(&result.TodayPV)
		h.DB.Model(&models.PageView{}).Where("created_at >= ?", todayStart).Select("COUNT(DISTINCT ip)").Scan(&result.TodayUV)
		h.DB.Model(&models.PageView{}).Where("page_type = ?", "landlord_view").Select("COUNT(*)").Scan(&result.TotalLandlordView)

		// 转化率：本月新合同 / 本月房间详情PV
		var newContracts int64
		h.DB.Model(&models.RentalContract{}).Where("created_at >= ? AND status = ?", monthStart, "active").Select("COUNT(*)").Scan(&newContracts)
		var roomPV int64
		h.DB.Model(&models.PageView{}).Where("page_type = ? AND created_at >= ?", "room_detail", monthStart).Select("COUNT(*)").Scan(&roomPV)
		if roomPV > 0 {
			result.ConversionRate = float64(newContracts) / float64(roomPV) * 100
		}

		// 空置率
		var totalRooms, vacantRooms int64
		h.DB.Model(&models.Room{}).Select("COUNT(*)").Scan(&totalRooms)
		h.DB.Model(&models.Room{}).Where("status = ?", "vacant").Select("COUNT(*)").Scan(&vacantRooms)
		if totalRooms > 0 {
			result.VacancyRate = float64(vacantRooms) / float64(totalRooms) * 100
		}

		// 楼栋排行
		rows, err := h.DB.Raw(`
			SELECT pv.building_id, b.name,
			       COALESCE(pv.pv, 0) as pv,
			       COALESCE(pv.uv, 0) as uv,
			       COALESCE(lv.cnt, 0) as landlord_view
			FROM (
				SELECT building_id, COUNT(*) as pv, COUNT(DISTINCT ip) as uv
				FROM page_views
				WHERE page_type IN ('building_detail','room_detail')
				  AND building_id > 0
				  AND created_at >= ?
				GROUP BY building_id
			) pv
			JOIN buildings b ON b.id = pv.building_id
			LEFT JOIN (
				SELECT building_id, COUNT(*) as cnt
				FROM page_views
				WHERE page_type = 'landlord_view'
				  AND created_at >= ?
				GROUP BY building_id
			) lv ON lv.building_id = pv.building_id
			ORDER BY pv DESC LIMIT 20
		`, monthStart, monthStart).Rows()
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var item buildingRankItem
				var landlordView int64
				if err := rows.Scan(&item.BuildingID, &item.BuildingName, &item.PV, &item.UV, &landlordView); err == nil {
					item.LandlordView = landlordView
					item.RoomCount = 0
					item.VacantCount = 0
					result.BuildingRank = append(result.BuildingRank, item)
				}
			}
		}

		// 填充楼栋的房间数和空置数
		for i, b := range result.BuildingRank {
			var roomCount, vacantCount int64
			h.DB.Model(&models.Room{}).Where("building_id = ?", b.BuildingID).Select("COUNT(*)").Scan(&roomCount)
			h.DB.Model(&models.Room{}).Where("building_id = ? AND status = ?", b.BuildingID, "vacant").Select("COUNT(*)").Scan(&vacantCount)
			result.BuildingRank[i].RoomCount = roomCount
			result.BuildingRank[i].VacantCount = vacantCount
		}

		return &result, nil
	})
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "查询失败")
		return
	}
	utils.Success(c, gin.H{"overview": data})
}

func (h *StatsHandler) Trend(c *gin.Context) {
	daysStr := c.DefaultQuery("days", "30")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days <= 0 || days > 365 {
		days = 30
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	cacheKey := "stats_trend_" + daysStr

	data, err := utils.CacheGetOrSet(cacheKey, 5*time.Minute, func() (interface{}, error) {
		var result []trendItem
		rows, err := h.DB.Raw(`
			SELECT DATE(created_at) as date, COUNT(*) as pv, COUNT(DISTINCT ip) as uv
			FROM page_views
			WHERE created_at >= ?
			GROUP BY DATE(created_at)
			ORDER BY date
		`, cutoff).Rows()
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var item trendItem
			if err := rows.Scan(&item.Date, &item.PV, &item.UV); err == nil {
				result = append(result, item)
			}
		}
		return result, nil
	})
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "查询失败")
		return
	}
	utils.Success(c, gin.H{"trend": data})
}

func (h *StatsHandler) PriceReference(c *gin.Context) {
	data, err := utils.CacheGetOrSet("stats_price_ref", 5*time.Minute, func() (interface{}, error) {
		var result []priceRefItem
		if err := h.DB.Raw(`
			SELECT b.district, r.layout,
			       MIN(r.rent_price) as min_price,
			       MAX(r.rent_price) as max_price,
			       ROUND(AVG(r.rent_price), 0) as avg_price,
			       COUNT(*) as count
			FROM rooms r
			JOIN buildings b ON b.id = r.building_id
			WHERE r.rent_price IS NOT NULL AND b.status = 'active'
			GROUP BY b.district, r.layout
			ORDER BY b.district, r.layout
		`).Scan(&result).Error; err != nil {
			return nil, err
		}
		return result, nil
	})
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "查询失败")
		return
	}
	utils.Success(c, gin.H{"price_reference": data})
}

func (h *StatsHandler) BuildingDetail(c *gin.Context) {
	idStr := c.Param("id")
	buildingID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的公寓ID")
		return
	}
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	var result buildingDetailData
	result.BuildingID = uint(buildingID)

	var building models.Building
	if err := h.DB.First(&building, buildingID).Error; err != nil {
		utils.Error(c, http.StatusNotFound, "公寓不存在")
		return
	}
	result.BuildingName = building.Name

	h.DB.Model(&models.PageView{}).
		Where("(page_type = ? OR page_type = ?) AND building_id = ?", "building_detail", "room_detail", buildingID).
		Select("COUNT(*)").Scan(&result.PV)
	h.DB.Model(&models.PageView{}).
		Where("(page_type = ? OR page_type = ?) AND building_id = ?", "building_detail", "room_detail", buildingID).
		Select("COUNT(DISTINCT ip)").Scan(&result.UV)
	h.DB.Model(&models.PageView{}).
		Where("page_type = ? AND building_id = ?", "landlord_view", buildingID).
		Select("COUNT(*)").Scan(&result.LandlordView)

	// 转化率
	var newContracts int64
	h.DB.Model(&models.RentalContract{}).Where("building_id = ? AND created_at >= ? AND status = ?", buildingID, monthStart, "active").Select("COUNT(*)").Scan(&newContracts)
	var roomPV int64
	h.DB.Model(&models.PageView{}).Where("page_type = ? AND building_id = ? AND created_at >= ?", "room_detail", buildingID, monthStart).Select("COUNT(*)").Scan(&roomPV)
	if roomPV > 0 {
		result.ConversionRate = float64(newContracts) / float64(roomPV) * 100
	}

	// 房间排行
	rows, err := h.DB.Raw(`
		SELECT pv.resource_id, r.room_number, r.layout, pv.pv, r.status
		FROM (
			SELECT resource_id, COUNT(*) as pv
			FROM page_views
			WHERE page_type = 'room_detail'
			  AND building_id = ?
			  AND created_at >= ?
			GROUP BY resource_id
		) pv
		JOIN rooms r ON r.id = pv.resource_id AND r.building_id = ?
		ORDER BY pv DESC LIMIT 50
	`, buildingID, monthStart, buildingID).Rows()
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var item roomRankItem
			if err := rows.Scan(&item.RoomID, &item.RoomNumber, &item.Layout, &item.PV, &item.Status); err == nil {
				result.RoomRank = append(result.RoomRank, item)
			}
		}
	}

	utils.Success(c, gin.H{"building_stats": result})
}

// ===== 楼栋管理员 API（自动 scope 到自己的 building_id）=====

func (h *StatsHandler) MyBuildingStats(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	cacheKey := "my_stats_" + strconv.Itoa(int(bid))

	data, err := utils.CacheGetOrSet(cacheKey, 5*time.Minute, func() (interface{}, error) {
		type myStats struct {
			PV             int64          `json:"pv"`
			UV             int64          `json:"uv"`
			TodayPV        int64          `json:"today_pv"`
			TodayUV        int64          `json:"today_uv"`
			LandlordView   int64          `json:"landlord_view"`
			ConversionRate float64        `json:"conversion_rate"`
			RoomRank       []roomRankItem `json:"room_rank"`
		}
		var s myStats
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		h.DB.Model(&models.PageView{}).Where("(page_type = ? OR page_type = ?) AND building_id = ?", "building_detail", "room_detail", bid).Select("COUNT(*)").Scan(&s.PV)
		h.DB.Model(&models.PageView{}).Where("(page_type = ? OR page_type = ?) AND building_id = ?", "building_detail", "room_detail", bid).Select("COUNT(DISTINCT ip)").Scan(&s.UV)
		h.DB.Model(&models.PageView{}).Where("(page_type = ? OR page_type = ?) AND building_id = ? AND created_at >= ?", "building_detail", "room_detail", bid, todayStart).Select("COUNT(*)").Scan(&s.TodayPV)
		h.DB.Model(&models.PageView{}).Where("(page_type = ? OR page_type = ?) AND building_id = ? AND created_at >= ?", "building_detail", "room_detail", bid, todayStart).Select("COUNT(DISTINCT ip)").Scan(&s.TodayUV)
		h.DB.Model(&models.PageView{}).Where("page_type = ? AND building_id = ?", "landlord_view", bid).Select("COUNT(*)").Scan(&s.LandlordView)

		var newContracts int64
		h.DB.Model(&models.RentalContract{}).Where("building_id = ? AND created_at >= ? AND status = ?", bid, monthStart, "active").Select("COUNT(*)").Scan(&newContracts)
		var roomPV int64
		h.DB.Model(&models.PageView{}).Where("page_type = ? AND building_id = ? AND created_at >= ?", "room_detail", bid, monthStart).Select("COUNT(*)").Scan(&roomPV)
		if roomPV > 0 {
			s.ConversionRate = float64(newContracts) / float64(roomPV) * 100
		}

		rows, err := h.DB.Raw(`
			SELECT pv.resource_id, r.room_number, r.layout, pv.pv, r.status
			FROM (
				SELECT resource_id, COUNT(*) as pv
				FROM page_views
				WHERE page_type = 'room_detail' AND building_id = ? AND created_at >= ?
				GROUP BY resource_id
			) pv
			JOIN rooms r ON r.id = pv.resource_id
			ORDER BY pv DESC LIMIT 50
		`, bid, monthStart).Rows()
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var item roomRankItem
				if err := rows.Scan(&item.RoomID, &item.RoomNumber, &item.Layout, &item.PV, &item.Status); err == nil {
					s.RoomRank = append(s.RoomRank, item)
				}
			}
		}
		return &s, nil
	})
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "查询失败")
		return
	}
	utils.Success(c, gin.H{"stats": data})
}

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
	cutoff := time.Now().AddDate(0, 0, -days)
	cacheKey := "my_trend_" + strconv.Itoa(int(bid)) + "_" + daysStr

	data, err := utils.CacheGetOrSet(cacheKey, 5*time.Minute, func() (interface{}, error) {
		var result []trendItem
		rows, err := h.DB.Raw(`
			SELECT DATE(created_at) as date, COUNT(*) as pv, COUNT(DISTINCT ip) as uv
			FROM page_views
			WHERE (page_type = ? OR page_type = ?) AND building_id = ? AND created_at >= ?
			GROUP BY DATE(created_at)
			ORDER BY date
		`, "building_detail", "room_detail", bid, cutoff).Rows()
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var item trendItem
			if err := rows.Scan(&item.Date, &item.PV, &item.UV); err == nil {
				result = append(result, item)
			}
		}
		return result, nil
	})
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "查询失败")
		return
	}
	utils.Success(c, gin.H{"trend": data})
}

// 公开埋点 API

type landlordViewReq struct {
	BuildingID uint `json:"building_id" binding:"required"`
}

func (h *StatsHandler) RecordLandlordView(c *gin.Context) {
	var req landlordViewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	utils.RecordPageView(h.DB, "landlord_view", req.BuildingID, req.BuildingID, utils.GetRealIP(c))
	c.JSON(http.StatusOK, utils.APIResponse{Code: 0, Message: "ok"})
}
