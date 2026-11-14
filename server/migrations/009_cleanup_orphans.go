package migrations

import (
	"rental-server/logger"

	"gorm.io/gorm"
)

// upCleanupOrphans 清理历史遗留的孤儿行（2026-11 修复）。
//
// 背景：存量库的外键都是 ON DELETE NO ACTION（模型里的 constraint 标签写在标量字段上，
// GORM 从不消费，实际建出来的是无 ON DELETE 子句的外键），而删除逻辑又按
// 「数据库会级联」的假设编写，于是删公寓后有大量数据永久残留：
//   - 账单被硬删后，bill_evidences 行留在库里（凭证文件也因此被永久钉住，GC 不再回收）；
//   - 房间被硬删后，bills.room_id 仍指向它，导致房间行再也删不掉；
//   - 房间/公寓被删后，room_media、building_landlords 等无软删字段的子行成为孤儿；
//   - 待办/合同指向已不存在的房间或公寓。
//
// 本步骤只做「数据修复」，不改表结构（外键仍是 NO ACTION，靠显式解绑与顺序编排兜底）。
// 判定口径统一为「父行完全不存在（硬删）」——软删的父行仍算存在，其子行保持不动。
//
// 注意：rental_contracts.room_id / building_id 是 NOT NULL 列，不能置空，
// 只能删除指向已不存在父行的合同（这类行本就无法正常展示）。
func upCleanupOrphans(db *gorm.DB) error {
	steps := []struct {
		name  string
		table string
		sql   string
	}{
		// 账单凭证：账单已不存在（被硬删）的孤儿行
		{"bill_evidences(账单不存在)", "bill_evidences", `DELETE FROM bill_evidences WHERE bill_id NOT IN (SELECT id FROM bills)`},
		// 房间媒体：房间已不存在的孤儿行
		{"room_media(房间不存在)", "room_media", `DELETE FROM room_media WHERE room_id NOT IN (SELECT id FROM rooms)`},
		// 房东：公寓已不存在的孤儿行
		{"building_landlords(公寓不存在)", "building_landlords", `DELETE FROM building_landlords WHERE building_id NOT IN (SELECT id FROM buildings)`},
		// 账单的房间引用：房间已不存在则置空（账单本身保留）
		{"bills.room_id(房间不存在)", "bills", `UPDATE bills SET room_id = NULL WHERE room_id IS NOT NULL AND room_id NOT IN (SELECT id FROM rooms)`},
		// 账单本身：公寓已不存在则删除（公寓没了，账单在界面上已不可达）
		{"bills(公寓不存在)", "bills", `DELETE FROM bills WHERE building_id NOT IN (SELECT id FROM buildings)`},
		// 待办的房间/公寓引用：父行已不存在则置空（平台级待办本就是 NULL）
		{"tasks.room_id(房间不存在)", "tasks", `UPDATE tasks SET room_id = NULL WHERE room_id IS NOT NULL AND room_id NOT IN (SELECT id FROM rooms)`},
		{"tasks.building_id(公寓不存在)", "tasks", `UPDATE tasks SET building_id = NULL WHERE building_id IS NOT NULL AND building_id NOT IN (SELECT id FROM buildings)`},
		// 合同的房间/公寓引用：父行已不存在则删除该合同（room_id/building_id 为 NOT NULL，无法置空）
		{"rental_contracts(房间或公寓不存在)", "rental_contracts", `DELETE FROM rental_contracts WHERE room_id NOT IN (SELECT id FROM rooms) OR building_id NOT IN (SELECT id FROM buildings)`},
	}
	for _, s := range steps {
		// 表不存在（老库缺表）时跳过，不让迁移整体失败
		if !db.Migrator().HasTable(s.table) {
			continue
		}
		if err := db.Exec(s.sql).Error; err != nil {
			return err
		}
		logger.Log.Info().Str("step", s.name).Msg("迁移 009：已清理孤儿数据")
	}
	logger.Log.Info().Msg("迁移 009：孤儿数据清理完成")
	return nil
}

// downCleanupOrphans 回退：数据修复不可逆（孤儿行已被物理删除），此处仅记录。
func downCleanupOrphans(db *gorm.DB) error {
	logger.Log.Warn().Msg("迁移 009 是数据修复，回退不会恢复已删除的孤儿行")
	return nil
}
