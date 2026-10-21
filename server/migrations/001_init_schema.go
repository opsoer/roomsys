package migrations

import (
	"rental-server/models"

	"gorm.io/gorm"
)

// upInitSchema 建全部表（首次初始化）。
//
// 说明：初始建表用 AutoMigrate 是安全的——此时表还不存在，不存在「历史数据留在旧列」的问题。
// 之后的所有结构变更都必须以新的编号步骤追加，不再依赖 AutoMigrate 猜测。
func upInitSchema(db *gorm.DB) error {
	return models.AutoMigrate(db)
}
