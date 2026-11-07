package migrations

import (
	"rental-server/logger"
	"rental-server/models"

	"gorm.io/gorm"
)

// upBillEvidences 建 bill_evidences 表：账单凭证截图（可选功能）。
// 纯新增表，无历史数据迁移；老库补表、新库由 001 按最新模型建表时已存在。
func upBillEvidences(db *gorm.DB) error {
	if db.Migrator().HasTable(&models.BillEvidence{}) {
		logger.Log.Info().Msg("迁移 004：bill_evidences 已存在，跳过")
		return nil
	}
	if err := db.Migrator().CreateTable(&models.BillEvidence{}); err != nil {
		return err
	}
	logger.Log.Info().Msg("迁移 004：已创建 bill_evidences 表")
	return nil
}

// downBillEvidences 回退：删除 bill_evidences 表。
func downBillEvidences(db *gorm.DB) error {
	return db.Migrator().DropTable(&models.BillEvidence{})
}
