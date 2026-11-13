// 统计服务，提供数据看板、趋势分析等统计查询能力。
//
// 存储约定（2026-11 重构）：平台级累计数（总 PV/UV/趋势/今日昨日）来自聚合表
// stat_totals / stat_ips / stat_daily_ips / stat_daily（building_id=0 平台行），
// 由写入路径 utils.ApplyPageViewBatch 随每次访问同步维护——只增不减，
// 删除公寓与 90 天明细清理都不会让平台总数回落；
// 楼栋级统计（热度排行/楼栋详情/楼栋概览/房间排行）中 PV 类来自 stat_daily
// 楼栋行（随公寓删除一并清零），去重 UV 类仍按 page_views 明细（90 天保留期内精确）。
package services

import (
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

// dayKey 把时间截断为本地日期键，与写入端 utils.ApplyPageViewBatch 的日键一致。
func dayKey(t time.Time) string {
	return t.Format("2006-01-02")
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
// 注意：本函数使用裸 time.Now()（未走 utils.Now()），不跟随全局时间偏移；
// 在启用时间偏移的环境（测试/回放）下，统计窗口与业务时间会产生偏差。
func (s *StatsService) Overview() (*OverviewData, error) {
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	var result OverviewData

	// 平台累计计数器：总 PV / 总获电数（只增不减，删除公寓不影响）
	var totals models.StatTotal
	if err := s.DB.First(&totals, 1).Error; err == nil {
		result.TotalPV = totals.TotalPV
		result.TotalLandlordView = totals.TotalLandlordView
	}
	// 总 UV：全历史去重 IP（stat_ips 行数，只增不减）
	s.DB.Model(&models.StatIP{}).Select("COUNT(*)").Scan(&result.TotalUV)

	// 今日/昨日 PV：平台日计数行（stat_daily building_id=0）
	todayKey := dayKey(now)
	yesterdayKey := dayKey(now.AddDate(0, 0, -1))
	var todayRow, yesterdayRow models.StatDaily
	if err := s.DB.Where("day = ? AND building_id = 0", todayKey).First(&todayRow).Error; err == nil {
		result.TodayPV = todayRow.PV
	}
	if err := s.DB.Where("day = ? AND building_id = 0", yesterdayKey).First(&yesterdayRow).Error; err == nil {
		result.YesterdayPV = yesterdayRow.PV
	}
	// 今日/昨日 UV：每日去重 IP
	s.DB.Model(&models.StatDailyIP{}).Where("day = ?", todayKey).Select("COUNT(*)").Scan(&result.TodayUV)
	s.DB.Model(&models.StatDailyIP{}).Where("day = ?", yesterdayKey).Select("COUNT(*)").Scan(&result.YesterdayUV)

	// 获电率：landlord_view 事件数 / 平台 PV 总量 * 100（浮点百分比）。
	// Overview 的分母是全部 page_type 的 PV（stat_totals.total_pv，不过滤类型），
	// BuildingDetail/MyBuildingStats 的分母是 stat_daily.detail_pv（building_detail+
	// room_detail 两类合计）—— 三处口径不同，各自注释勿混淆；
	// 若调整分母口径，务必同步修改三处保持含义一致。
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

	// 楼栋排行：PV（detail 两类口径）/获电数来自 stat_daily 月窗聚合，随公寓删除一并清零；
	// UV 来自 page_views 月窗去重（月窗恒在 90 天明细保留期内）。
	// 「存活且未软删」「detail_pv>0 才入榜」的过滤必须下推到聚合查询里再 LIMIT——
	// 若先取 Top20 再过滤，迁移回填或异步竞争留下的已删楼栋孤儿行会把活跃公寓挤出榜单。
	monthKey := dayKey(monthStart)
	type rankAgg struct {
		BuildingID   uint
		BuildingName string
		DetailPV     int64
		LV           int64
	}
	var aggs []rankAgg
	s.DB.Table("stat_daily").
		Select("stat_daily.building_id, b.name AS building_name, "+
			"COALESCE(SUM(stat_daily.detail_pv), 0) AS detail_pv, "+
			"COALESCE(SUM(stat_daily.landlord_view), 0) AS lv").
		Joins("JOIN buildings b ON b.id = stat_daily.building_id AND b.deleted_at IS NULL").
		Where("stat_daily.day >= ? AND stat_daily.building_id > 0", monthKey).
		Group("stat_daily.building_id, b.name").
		Having("COALESCE(SUM(stat_daily.detail_pv), 0) > 0").
		Order("detail_pv DESC").
		Limit(20).
		Scan(&aggs)

	type uvAgg struct {
		BuildingID uint
		UV         int64
	}
	var uvs []uvAgg
	s.DB.Table("page_views").
		Select("building_id, COUNT(DISTINCT ip) AS uv").
		Where("page_type IN ('building_detail','room_detail') AND building_id > 0 AND created_at >= ?", monthStart).
		Group("building_id").
		Scan(&uvs)
	uvBy := make(map[uint]int64, len(uvs))
	for _, r := range uvs {
		uvBy[r.BuildingID] = r.UV
	}

	for _, a := range aggs {
		result.BuildingRank = append(result.BuildingRank, BuildingRankItem{
			BuildingID:   a.BuildingID,
			BuildingName: a.BuildingName,
			PV:           a.DetailPV,
			UV:           uvBy[a.BuildingID],
			LandlordView: a.LV,
		})
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

// Trend 获取PV/UV趋势数据（平台日计数聚合，补零填充为连续日序列）。
// PV/获电数来自 stat_daily 平台行（building_id=0），UV 来自 stat_daily_ips 按日去重
// —— 删除公寓与 90 天明细清理都不影响平台日计数，历史趋势稳定。
func (s *StatsService) Trend(cutoff, todayStart time.Time) ([]TrendItem, error) {
	cutoffKey := dayKey(cutoff)
	var rows []struct {
		Day          string
		PV           int64
		LandlordView int64
	}
	s.DB.Model(&models.StatDaily{}).
		Select("day, pv, landlord_view").
		Where("day >= ? AND building_id = 0", cutoffKey).
		Order("day ASC").
		Scan(&rows)

	var uvRows []struct {
		Day string
		UV  int64
	}
	s.DB.Model(&models.StatDailyIP{}).
		Select("day, COUNT(*) AS uv").
		Where("day >= ?", cutoffKey).
		Group("day").
		Scan(&uvRows)
	uvBy := make(map[string]int64, len(uvRows))
	for _, r := range uvRows {
		uvBy[r.Day] = r.UV
	}

	result := make([]TrendItem, 0, len(rows))
	for _, r := range rows {
		result = append(result, TrendItem{Date: r.Day, PV: r.PV, UV: uvBy[r.Day], LandlordView: r.LandlordView})
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
// 注意：使用裸 time.Now()（未走 utils.Now()），不跟随全局时间偏移（同 Overview）。
// PV/获电数来自 stat_daily 楼栋行累计（随公寓删除清零）；UV 按 page_views 去重
// （明细 90 天保留期内精确，口径与重构前一致）。
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

	var dailyAgg struct {
		PV int64
		LV int64
	}
	s.DB.Model(&models.StatDaily{}).
		Select("COALESCE(SUM(detail_pv), 0) AS pv, COALESCE(SUM(landlord_view), 0) AS lv").
		Where("building_id = ?", buildingID).
		Scan(&dailyAgg)
	result.PV = dailyAgg.PV
	result.LandlordView = dailyAgg.LV
	s.DB.Model(&models.PageView{}).
		Where("(page_type = ? OR page_type = ?) AND building_id = ?", "building_detail", "room_detail", buildingID).
		Select("COUNT(DISTINCT ip)").Scan(&result.UV)

	// 获电率：landlord_view 事件数 / detail_pv（building_detail+room_detail 两类合计）* 100。
	// 分母口径与 Overview 全量 PV 不同，见 Overview 处注释。
	if result.PV > 0 {
		result.PhoneRate = float64(result.LandlordView) / float64(result.PV) * 100
	}

	// 房间排行：r.deleted_at IS NULL 保证软删除房间不漏出排行
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
		JOIN rooms r ON r.id = pv.resource_id AND r.building_id = ? AND r.deleted_at IS NULL
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
// 注意：使用裸 time.Now()（未走 utils.Now()），不跟随全局时间偏移（同 Overview）。
// PV/获电数/今日 PV 来自 stat_daily 楼栋行（随公寓删除清零）；UV/今日 UV 按
// page_views 去重（口径与重构前一致）。
func (s *StatsService) MyBuildingStats(bid uint) (*MyBuildingStatsData, error) {
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	todayKey := dayKey(now)

	var data MyBuildingStatsData

	var agg struct {
		PV int64
		LV int64
	}
	s.DB.Model(&models.StatDaily{}).
		Select("COALESCE(SUM(detail_pv), 0) AS pv, COALESCE(SUM(landlord_view), 0) AS lv").
		Where("building_id = ?", bid).
		Scan(&agg)
	data.PV = agg.PV
	data.LandlordView = agg.LV
	var todayRow models.StatDaily
	if err := s.DB.Where("day = ? AND building_id = ?", todayKey, bid).First(&todayRow).Error; err == nil {
		data.TodayPV = todayRow.DetailPV
	}
	s.DB.Model(&models.PageView{}).
		Where("(page_type = ? OR page_type = ?) AND building_id = ?", "building_detail", "room_detail", bid).
		Select("COUNT(DISTINCT ip)").Scan(&data.UV)
	s.DB.Model(&models.PageView{}).
		Where("(page_type = ? OR page_type = ?) AND building_id = ? AND created_at >= ?", "building_detail", "room_detail", bid, todayStart).
		Select("COUNT(DISTINCT ip)").Scan(&data.TodayUV)

	if data.PV > 0 {
		data.PhoneRate = float64(data.LandlordView) / float64(data.PV) * 100
	}

	// 房间状态概况：单条聚合查出总数与空置数，出租率按非空置口径计算。
	// 注意：这里直接用 rooms.status 原始 DB 值（vacant/rented/reserved），
	// 不做 expiring/expired 动态派生（与 building 列表的动态状态口径不同）。
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

	// 房间排行：r.deleted_at IS NULL 保证软删除房间不漏出排行
	rows, err := s.DB.Raw(`
		SELECT pv.resource_id, r.room_number, r.layout, pv.pv, r.status
		FROM (
			SELECT resource_id, COUNT(*) as pv
			FROM page_views
			WHERE page_type = 'room_detail' AND building_id = ? AND created_at >= ?
			GROUP BY resource_id
		) pv
		JOIN rooms r ON r.id = pv.resource_id AND r.deleted_at IS NULL
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

// MyBuildingTrend 获取指定楼栋的趋势数据（楼栋日计数聚合，补零填充为连续日序列）。
// PV（=detail_pv+landlord_view，与重构前三类 page_type 口径一致）与获电数来自
// stat_daily 楼栋行，随公寓删除清零；UV 仍按 page_views 日去重（明细保留期内）。
func (s *StatsService) MyBuildingTrend(bid uint, cutoff, todayStart time.Time) ([]TrendItem, error) {
	cutoffKey := dayKey(cutoff)
	var rows []struct {
		Day          string
		DetailPV     int64
		LandlordView int64
	}
	s.DB.Model(&models.StatDaily{}).
		Select("day, detail_pv, landlord_view").
		Where("day >= ? AND building_id = ?", cutoffKey, bid).
		Order("day ASC").
		Scan(&rows)

	dayExpr := database.DayExpr(s.DB, "created_at")
	var uvRows []struct {
		Day string
		UV  int64
	}
	s.DB.Table("page_views").
		Select(dayExpr+" AS day, COUNT(DISTINCT ip) AS uv").
		Where("page_type IN ('building_detail','room_detail','landlord_view') AND building_id = ? AND created_at >= ?", bid, cutoff).
		Group(dayExpr).
		Scan(&uvRows)
	uvBy := make(map[string]int64, len(uvRows))
	for _, r := range uvRows {
		uvBy[r.Day] = r.UV
	}

	result := make([]TrendItem, 0, len(rows))
	for _, r := range rows {
		result = append(result, TrendItem{Date: r.Day, PV: r.DetailPV + r.LandlordView, UV: uvBy[r.Day], LandlordView: r.LandlordView})
	}
	return fillTrendDays(result, cutoff, todayStart), nil
}
