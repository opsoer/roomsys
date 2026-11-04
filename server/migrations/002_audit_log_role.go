package migrations

import (
	"rental-server/logger"
	"rental-server/models"

	"gorm.io/gorm"
)

// upAuditLogRole 给 audit_logs 表补上 role 列：审计页面需要按角色筛选与展示，
// 此前角色只写进了文件日志、未落库，历史记录的角色值为空字符串。
func upAuditLogRole(db *gorm.DB) error {
	// 新库由 001 按最新模型建表时已带 role 列；老库才需要补列，因此先判断再执行
	if db.Migrator().HasColumn(&models.AuditLog{}, "Role") {
		logger.Log.Info().Msg("迁移 002：audit_logs 已存在 role 列，跳过")
		return nil
	}
	if err := db.Migrator().AddColumn(&models.AuditLog{}, "Role"); err != nil {
		return err
	}
	logger.Log.Info().Msg("迁移 002：audit_logs 已补充 role 列")
	return nil
}

// downAuditLogRole 回退：删除 role 列。
func downAuditLogRole(db *gorm.DB) error {
	return db.Migrator().DropColumn(&models.AuditLog{}, "Role")
}
