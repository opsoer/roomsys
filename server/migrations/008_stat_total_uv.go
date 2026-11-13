package migrations

import (
	"rental-server/logger"
	"rental-server/models"

	"gorm.io/gorm"
)

// upStatTotalUV 新增 stat_totals.total_uv，并从 stat_ips 精确回填。
// 总 UV 改读计数器，避免对只增不减的 stat_ips 做大表 COUNT(*)。
func upStatTotalUV(db *gorm.DB) error {
	if !db.Migrator().HasTable(&models.StatTotal{}) {
		return nil
	}
	if !db.Migrator().HasColumn(&models.StatTotal{}, "total_uv") {
		if err := db.Migrator().AddColumn(&models.StatTotal{}, "TotalUV"); err != nil {
			return err
		}
	}
	if err := db.Exec(
		"UPDATE stat_totals SET total_uv = (SELECT COUNT(*) FROM stat_ips) WHERE id = 1",
	).Error; err != nil {
		return err
	}
	logger.Log.Info().Msg("迁移 008：已补 stat_totals.total_uv 并回填")
	return nil
}

// downStatTotalUV 回退：删除 total_uv 列（总 UV 回退到 stat_ips COUNT(*) 口径）。
func downStatTotalUV(db *gorm.DB) error {
	if !db.Migrator().HasTable(&models.StatTotal{}) {
		return nil
	}
	if !db.Migrator().HasColumn(&models.StatTotal{}, "total_uv") {
		return nil
	}
	return db.Migrator().DropColumn(&models.StatTotal{}, "total_uv")
}
