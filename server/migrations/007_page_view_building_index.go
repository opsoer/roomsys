package migrations

import (
	"rental-server/logger"
	"rental-server/models"

	"gorm.io/gorm"
)

// upPageViewBuildingIndex 补 page_views.building_id 独立索引。
//
// 现有 idx_pv_query 以 page_type 为最左列，删除公寓、孤儿清理按 building_id 过滤时
// 用不上；数据量大时会退化为全表扫描。
func upPageViewBuildingIndex(db *gorm.DB) error {
	if db.Migrator().HasIndex(&models.PageView{}, "idx_pv_building") {
		return nil
	}
	if err := db.Migrator().CreateIndex(&models.PageView{}, "idx_pv_building"); err != nil {
		return err
	}
	logger.Log.Info().Msg("迁移 007：已补 page_views.building_id 索引")
	return nil
}

// downPageViewBuildingIndex 回退：删除 007 补的索引。
func downPageViewBuildingIndex(db *gorm.DB) error {
	if !db.Migrator().HasIndex(&models.PageView{}, "idx_pv_building") {
		return nil
	}
	return db.Migrator().DropIndex(&models.PageView{}, "idx_pv_building")
}
