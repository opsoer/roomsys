// 统计服务，提供数据看板、趋势分析等统计查询能力
package services

import (
	"fmt"
	"time"

	"rental-server/database"
	"rental-server/models"

	"gorm.io/gorm"
)

// StatsService 统计服务
type StatsService struct {
	DB *gorm.DB
}

// NewStatsService 创建统计服务实例
func NewStatsService(db *gorm.DB) *StatsService {
	return &StatsService{DB: db}
}

// OverviewData 概览统计数据
type OverviewData struct {
	TotalPV           int64              `json:"total_pv"`
	TotalUV           int64              `json:"total_uv"`
	YesterdayPV       int64              `json:"yesterday_pv"`
	YesterdayUV       int64              `json:"yesterday_uv"`
	TodayPV           int64              `json:"today_pv"`
	TodayUV           int64              `json:"today_uv"`
	TotalLandlordView int64              `json:"total_landlord_view"`
	PhoneRate         float64            `json:"phone_rate"`
	VacancyRate       float64            `json:"vacancy_rate"`
	BuildingRank      []BuildingRankItem `json:"building_rank"`
}

// BuildingRankItem 楼栋排行数据项
type BuildingRankItem struct {
	BuildingID   uint   `json:"building_id"`
	BuildingName string `json:"building_name"`
	PV           int64  `json:"pv"`
	UV           int64  `json:"uv"`
	VacantCount  int64  `json:"vacant_count"`
	RoomCount    int64  `json:"room_count"`
	LandlordView int64  `json:"landlord_view"`
}

// TrendItem 趋势数据项
type TrendItem struct {
	Date         string `json:"date"`
	PV           int64  `json:"pv"`
	UV           int64  `json:"uv"`
	LandlordView int64  `json:"landlord_view"`
}

// PriceRefItem 价格参考数据项
type PriceRefItem struct {
	District string  `json:"district"`
	Layout   string  `json:"layout"`
	MinPrice float64 `json:"min_price"`
	MaxPrice float64 `json:"max_price"`
	AvgPrice float64 `json:"avg_price"`
	Count    int64   `json:"count"`
}

// BuildingDetailData 楼栋详情统计数据
type BuildingDetailData struct {
	BuildingID   uint           `json:"building_id"`
	BuildingName string         `json:"building_name"`
	PV           int64          `json:"pv"`
	UV           int64          `json:"uv"`
	LandlordView int64          `json:"landlord_view"`
	PhoneRate    float64        `json:"phone_rate"`
	RoomRank     []RoomRankItem `json:"room_rank"`
}

// RoomRankItem 房间排行数据项
type RoomRankItem struct {
	RoomID     uint   `json:"room_id"`
	RoomNumber string `json:"room_number"`
	Layout     string `json:"layout"`
	PV         int64  `json:"pv"`
	Status     string `json:"status"`
}

// MyBuildingStatsData 楼栋管理员统计概览数据
type MyBuildingStatsData struct {
	PV            int64          `json:"pv"`
	UV            int64          `json:"uv"`
	TodayPV       int64          `json:"today_pv"`
	TodayUV       int64          `json:"today_uv"`
	LandlordView  int64          `json:"landlord_view"`
	PhoneRate     float64        `json:"phone_rate"`
	RoomTotal     int64          `json:"room_total"`
	RoomVacant    int64          `json:"room_vacant"`
	RoomRented    int64          `json:"room_rented"`
	OccupancyRate float64        `json:"occupancy_rate"`
	RoomRank      []RoomRankItem `json:"room_rank"`
}

// fillTrendDays 把按天聚合结果补零填充为 [start, end] 的连续日序列。
// GROUP BY 只返回有数据的日期，缺行会让趋势图 x 轴跳日、趋势失真。
func fillTrendDays(rows []TrendItem, start, end time.Time) []TrendItem {
	byDate := make(map[string]TrendItem, len(rows))
	for _, r := range rows {
		byDate[r.Date] = r
	}
	days := int(end.Sub(start).Hours()/24) + 1
	result := make([]TrendItem, 0, days)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		if item, ok := byDate[key]; ok {
			result = append(result, item)
		} else {
			result = append(result, TrendItem{Date: key})
		}
	}
	return result
}

// Overview 获取全局概览统计（PV、UV、转化率、空置率、楼栋排行）
func (s *StatsService) Overview() (*OverviewData, error) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	var result OverviewData

	s.DB.Model(&models.PageView{}).Select("COUNT(*)").Scan(&result.TotalPV)
	s.DB.Model(&models.PageView{}).Select("COUNT(DISTINCT ip)").Scan(&result.TotalUV)
	s.DB.Model(&models.PageView{}).Where("created_at >= ?", todayStart).Select("COUNT(*)").Scan(&result.TodayPV)
	s.DB.Model(&models.PageView{}).Where("created_at >= ?", todayStart).Select("COUNT(DISTINCT ip)").Scan(&result.TodayUV)
	yesterdayStart := todayStart.AddDate(0, 0, -1)
	s.DB.Model(&models.PageView{}).Where("created_at >= ? AND created_at < ?", yesterdayStart, todayStart).Select("COUNT(*)").Scan(&result.YesterdayPV)
	s.DB.Model(&models.PageView{}).Where("created_at >= ? AND created_at < ?", yesterdayStart, todayStart).Select("COUNT(DISTINCT ip)").Scan(&result.YesterdayUV)
	s.DB.Model(&models.PageView{}).Where("page_type = ?", "landlord_view").Select("COUNT(*)").Scan(&result.TotalLandlordView)

	// 获电率：房东获取数 / 总浏览量 * 100
	if result.TotalPV > 0 {
		result.PhoneRate = float64(result.TotalLandlordView) / float64(result.TotalPV) * 100
	}

	// 空置率
	var totalRooms, vacantRooms int64
	s.DB.Model(&models.Room{}).Select("COUNT(*)").Scan(&totalRooms)
	s.DB.Model(&models.Room{}).Where("status = ?", "vacant").Select("COUNT(*)").Scan(&vacantRooms)
	if totalRooms > 0 {
		result.VacancyRate = float64(vacantRooms) / float64(totalRooms) * 100
	}

	// 楼栋排行
	rows, err := s.DB.Raw(`
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
			var item BuildingRankItem
			var landlordView int64
			if err := rows.Scan(&item.BuildingID, &item.BuildingName, &item.PV, &item.UV, &landlordView); err == nil {
				item.LandlordView = landlordView
				item.RoomCount = 0
				item.VacantCount = 0
				result.BuildingRank = append(result.BuildingRank, item)
			}
		}
	}

	// 填充楼栋的房间数和空置数：单条 GROUP BY 聚合，替代逐楼栋两条查询的 N+1 写法
	if len(result.BuildingRank) > 0 {
		ids := make([]uint, 0, len(result.BuildingRank))
		for _, b := range result.BuildingRank {
			ids = append(ids, b.BuildingID)
		}
		type roomAgg struct {
			BuildingID uint
			Total      int64
			Vacant     int64
		}
		var aggs []roomAgg
		s.DB.Model(&models.Room{}).
			Select("building_id, COUNT(*) as total, COALESCE(SUM(CASE WHEN status = 'vacant' THEN 1 ELSE 0 END), 0) as vacant").
			Where("building_id IN ?", ids).
			Group("building_id").
			Scan(&aggs)
		aggBy := make(map[uint]roomAgg, len(aggs))
		for _, a := range aggs {
			aggBy[a.BuildingID] = a
		}
		for i, b := range result.BuildingRank {
			if a, ok := aggBy[b.BuildingID]; ok {
				result.BuildingRank[i].RoomCount = a.Total
				result.BuildingRank[i].VacantCount = a.Vacant
			}
		}
	}

	return &result, nil
}

// Trend 获取PV/UV趋势数据（按天聚合后补零填充为连续日序列）
func (s *StatsService) Trend(cutoff, todayStart time.Time) ([]TrendItem, error) {
	dayExpr := database.DayExpr(s.DB, "created_at")
	rows, err := s.DB.Raw(fmt.Sprintf(`
		SELECT %s as date,
		       COUNT(*) as pv,
		       COUNT(DISTINCT ip) as uv,
		       SUM(CASE WHEN page_type = 'landlord_view' THEN 1 ELSE 0 END) as landlord_view
		FROM page_views
		WHERE created_at >= ?
		GROUP BY %s
		ORDER BY date
	`, dayExpr, dayExpr), cutoff).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []TrendItem
	for rows.Next() {
		var item TrendItem
		if err := rows.Scan(&item.Date, &item.PV, &item.UV, &item.LandlordView); err == nil {
			result = append(result, item)
		}
	}
	return fillTrendDays(result, cutoff, todayStart), nil
}

// PriceReference 获取租金价格参考（按区域和户型分组）
func (s *StatsService) PriceReference() ([]PriceRefItem, error) {
	var result []PriceRefItem
	if err := s.DB.Raw(`
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
}

// BuildingDetail 获取单个楼栋的详细统计数据（含房间浏览排行）
func (s *StatsService) BuildingDetail(buildingID uint) (*BuildingDetailData, error) {
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	var result BuildingDetailData
	result.BuildingID = buildingID

	var building models.Building
	if err := s.DB.First(&building, buildingID).Error; err != nil {
		return nil, err
	}
	result.BuildingName = building.Name

	s.DB.Model(&models.PageView{}).
		Where("(page_type = ? OR page_type = ?) AND building_id = ?", "building_detail", "room_detail", buildingID).
		Select("COUNT(*)").Scan(&result.PV)
	s.DB.Model(&models.PageView{}).
		Where("(page_type = ? OR page_type = ?) AND building_id = ?", "building_detail", "room_detail", buildingID).
		Select("COUNT(DISTINCT ip)").Scan(&result.UV)
	s.DB.Model(&models.PageView{}).
		Where("page_type = ? AND building_id = ?", "landlord_view", buildingID).
		Select("COUNT(*)").Scan(&result.LandlordView)

	// 获电率
	if result.PV > 0 {
		result.PhoneRate = float64(result.LandlordView) / float64(result.PV) * 100
	}

	// 房间排行
	rows, err := s.DB.Raw(`
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
			var item RoomRankItem
			if err := rows.Scan(&item.RoomID, &item.RoomNumber, &item.Layout, &item.PV, &item.Status); err == nil {
				result.RoomRank = append(result.RoomRank, item)
			}
		}
	}

	return &result, nil
}

// MyBuildingStats 获取指定楼栋的统计概览（楼栋管理员 API，自动 scope 到自己的 building_id）
func (s *StatsService) MyBuildingStats(bid uint) (*MyBuildingStatsData, error) {
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var data MyBuildingStatsData

	s.DB.Model(&models.PageView{}).Where("(page_type = ? OR page_type = ?) AND building_id = ?", "building_detail", "room_detail", bid).Select("COUNT(*)").Scan(&data.PV)
	s.DB.Model(&models.PageView{}).Where("(page_type = ? OR page_type = ?) AND building_id = ?", "building_detail", "room_detail", bid).Select("COUNT(DISTINCT ip)").Scan(&data.UV)
	s.DB.Model(&models.PageView{}).Where("(page_type = ? OR page_type = ?) AND building_id = ? AND created_at >= ?", "building_detail", "room_detail", bid, todayStart).Select("COUNT(*)").Scan(&data.TodayPV)
	s.DB.Model(&models.PageView{}).Where("(page_type = ? OR page_type = ?) AND building_id = ? AND created_at >= ?", "building_detail", "room_detail", bid, todayStart).Select("COUNT(DISTINCT ip)").Scan(&data.TodayUV)
	s.DB.Model(&models.PageView{}).Where("page_type = ? AND building_id = ?", "landlord_view", bid).Select("COUNT(*)").Scan(&data.LandlordView)

	if data.PV > 0 {
		data.PhoneRate = float64(data.LandlordView) / float64(data.PV) * 100
	}

	// 房间状态概况：单条聚合查出总数与空置数，出租率按非空置口径计算
	var roomAgg struct {
		Total  int64
		Vacant int64
	}
	s.DB.Model(&models.Room{}).
		Where("building_id = ?", bid).
		Select("COUNT(*) as total, COALESCE(SUM(CASE WHEN status = 'vacant' THEN 1 ELSE 0 END), 0) as vacant").
		Scan(&roomAgg)
	data.RoomTotal = roomAgg.Total
	data.RoomVacant = roomAgg.Vacant
	data.RoomRented = roomAgg.Total - roomAgg.Vacant
	if data.RoomTotal > 0 {
		data.OccupancyRate = float64(data.RoomRented) / float64(data.RoomTotal) * 100
	}

	rows, err := s.DB.Raw(`
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
			var item RoomRankItem
			if err := rows.Scan(&item.RoomID, &item.RoomNumber, &item.Layout, &item.PV, &item.Status); err == nil {
				data.RoomRank = append(data.RoomRank, item)
			}
		}
	}
	return &data, nil
}

// MyBuildingTrend 获取指定楼栋的趋势数据（按天聚合后补零填充为连续日序列）
func (s *StatsService) MyBuildingTrend(bid uint, cutoff, todayStart time.Time) ([]TrendItem, error) {
	dayExpr := database.DayExpr(s.DB, "created_at")
	rows, err := s.DB.Raw(fmt.Sprintf(`
		SELECT %s as date,
		       COUNT(*) as pv,
		       COUNT(DISTINCT ip) as uv,
		       SUM(CASE WHEN page_type = 'landlord_view' THEN 1 ELSE 0 END) as landlord_view
		FROM page_views
		WHERE (page_type IN ('building_detail','room_detail','landlord_view')) AND building_id = ? AND created_at >= ?
		GROUP BY %s
		ORDER BY date
	`, dayExpr, dayExpr), bid, cutoff).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []TrendItem
	for rows.Next() {
		var item TrendItem
		if err := rows.Scan(&item.Date, &item.PV, &item.UV, &item.LandlordView); err == nil {
			result = append(result, item)
		}
	}
	return fillTrendDays(result, cutoff, todayStart), nil
}
