package migrations

import (
	"rental-server/logger"
	"rental-server/models"

	"gorm.io/gorm"
)

// upBuildingDirections 给 buildings 表补上 directions 列：详细地址/路引（每行一条），
// 公开主页与房间公开页在简介后合并展示。
func upBuildingDirections(db *gorm.DB) error {
	// 新库由 001 按最新模型建表时已带 directions 列；老库才需要补列，因此先判断再执行
	if db.Migrator().HasColumn(&models.Building{}, "Directions") {
		logger.Log.Info().Msg("迁移 003：buildings 已存在 directions 列，跳过")
		return nil
	}
	if err := db.Migrator().AddColumn(&models.Building{}, "Directions"); err != nil {
		return err
	}
	logger.Log.Info().Msg("迁移 003：buildings 已补充 directions 列")
	return nil
}

// downBuildingDirections 回退：删除 directions 列。
func downBuildingDirections(db *gorm.DB) error {
	return db.Migrator().DropColumn(&models.Building{}, "Directions")
}
