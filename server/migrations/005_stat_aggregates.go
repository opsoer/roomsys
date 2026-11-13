package migrations

import (
	"time"

	"rental-server/database"
	"rental-server/logger"
	"rental-server/models"

	"gorm.io/gorm"
)

// upStatAggregates 新增统计聚合表并把 page_views 历史数据回填进去。
//
// 背景：重构前平台总 PV/UV/趋势都是对 page_views 全表 COUNT(*) 实时遍历，
// 删除公寓或 90 天明细清理都会让"平台总数"回落。新增四张聚合表后：
//   - stat_totals    平台累计 PV/获电数（只增不减，与公寓生死解耦）
//   - stat_ips       全历史去重 IP（总 UV，只增不减）
//   - stat_daily_ips 每日去重 IP（每日平台 UV；历史行由定时任务修剪）
//   - stat_daily     按天按公寓计数（building_id=0 平台行永不删；楼栋行随公寓删）
//
// 老库升级：建表后从 page_views 一次性回填，保证升级后总数不归零；
// 新库由 001 按最新模型建表、此时 page_views 为空，回填自动跳过。
// 回填是全仓首个「数据回填」类步骤，按天表达式走 database.DayExpr 双方言约定。
func upStatAggregates(db *gorm.DB) error {
	tables := []interface{}{
		&models.StatTotal{},
		&models.StatIP{},
		&models.StatDailyIP{},
		&models.StatDaily{},
	}
	for _, t := range tables {
		if db.Migrator().HasTable(t) {
			continue
		}
		if err := db.Migrator().CreateTable(t); err != nil {
			return err
		}
	}
	logger.Log.Info().Msg("迁移 005：已创建 stat_totals / stat_ips / stat_daily_ips / stat_daily 表")

	// 回填守卫：stat_totals 已有数据说明回填过；page_views 为空（新库）无历史可回填
	var pvCount int64
	if err := db.Table("page_views").Count(&pvCount).Error; err != nil {
		return err
	}
	var totalRows int64
	if err := db.Table("stat_totals").Count(&totalRows).Error; err != nil {
		return err
	}
	if pvCount == 0 || totalRows > 0 {
		logger.Log.Info().Int64("page_views", pvCount).Msg("迁移 005：无需回填历史统计数据")
		return nil
	}

	day := database.DayExpr(db, "created_at")
	now := time.Now()
	lvExpr := "CASE WHEN page_type = 'landlord_view' THEN 1 ELSE 0 END"
	detailExpr := "CASE WHEN page_type IN ('building_detail','room_detail') THEN 1 ELSE 0 END"

	// 平台累计：总 PV / 总获电数
	if err := db.Exec(
		"INSERT INTO stat_totals (id, total_pv, total_landlord_view, updated_at) "+
			"SELECT 1, COUNT(*), COALESCE(SUM("+lvExpr+"), 0), ? FROM page_views", now,
	).Error; err != nil {
		return err
	}
	// 全历史去重 IP（与写入端口径一致：空 IP 不计入）
	if err := db.Exec(
		"INSERT INTO stat_ips (ip, created_at) SELECT DISTINCT ip, ? FROM page_views WHERE ip <> ''", now,
	).Error; err != nil {
		return err
	}
	// 每日去重 IP
	if err := db.Exec(
		"INSERT INTO stat_daily_ips (day, ip, created_at) SELECT DISTINCT "+day+", ip, ? FROM page_views WHERE ip <> ''", now,
	).Error; err != nil {
		return err
	}
	// 楼栋日计数：JOIN 存活（未软删）楼栋——升级前已删公寓的历史明细不建楼栋行，
	// 否则孤儿行会挤占排行榜（排行查询虽已按存活楼栋过滤，这里从源头不留垃圾）。
	// building_id=0 的列表页事件归平台行，避免与平台行主键冲突。
	dayB := database.DayExpr(db, "pv.created_at")
	if err := db.Exec(
		"INSERT INTO stat_daily (day, building_id, pv, detail_pv, landlord_view, updated_at) "+
			"SELECT "+dayB+", pv.building_id, COUNT(*), COALESCE(SUM("+detailExpr+"), 0), COALESCE(SUM("+lvExpr+"), 0), ? "+
			"FROM page_views pv JOIN buildings b ON b.id = pv.building_id AND b.deleted_at IS NULL "+
			"WHERE pv.building_id > 0 GROUP BY "+dayB+", pv.building_id", now,
	).Error; err != nil {
		return err
	}
	// 平台日计数（building_id=0，趋势图/今日昨日 PV 数据源；含列表页等全部事件）
	if err := db.Exec(
		"INSERT INTO stat_daily (day, building_id, pv, detail_pv, landlord_view, updated_at) "+
			"SELECT "+day+", 0, COUNT(*), COALESCE(SUM("+detailExpr+"), 0), COALESCE(SUM("+lvExpr+"), 0), ? "+
			"FROM page_views GROUP BY "+day, now,
	).Error; err != nil {
		return err
	}

	logger.Log.Info().Int64("page_views", pvCount).Msg("迁移 005：已从 page_views 回填历史统计数据")
	return nil
}

// downStatAggregates 回退：删除四张统计聚合表（回填数据随之丢弃，
// page_views 明细不受影响，可再次 Up 重建）。
func downStatAggregates(db *gorm.DB) error {
	return db.Migrator().DropTable(
		&models.StatDaily{},
		&models.StatDailyIP{},
		&models.StatIP{},
		&models.StatTotal{},
	)
}
