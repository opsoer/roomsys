package migrations

import (
	"rental-server/logger"
	"rental-server/models"

	"gorm.io/gorm"
)

// upStatDailyBuildingIndex 幂等补齐 stat_daily.building_id 索引。
//
// 005 首次建表时会按模型定义带索引；但 005 的早期版本建表后没有该索引的库
// （例如开发环境已跑过旧版 005），需要单独一步补上。删除公寓、孤儿行清理、
// 按楼栋聚合都依赖 building_id 过滤，缺索引会退化为全表扫描。
func upStatDailyBuildingIndex(db *gorm.DB) error {
	if db.Migrator().HasIndex(&models.StatDaily{}, "idx_stat_daily_building_id") {
		return nil
	}
	if err := db.Migrator().CreateIndex(&models.StatDaily{}, "idx_stat_daily_building_id"); err != nil {
		return err
	}
	logger.Log.Info().Msg("迁移 006：已补 stat_daily.building_id 索引")
	return nil
}

// downStatDailyBuildingIndex 回退：删除 006 补的索引。注意 005 重新建表时
// 仍会按模型定义自动创建该索引，回退后索引可能因表重建而再次出现。
func downStatDailyBuildingIndex(db *gorm.DB) error {
	if !db.Migrator().HasIndex(&models.StatDaily{}, "idx_stat_daily_building_id") {
		return nil
	}
	return db.Migrator().DropIndex(&models.StatDaily{}, "idx_stat_daily_building_id")
}
