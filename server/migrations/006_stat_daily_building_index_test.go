package migrations

import (
	"testing"

	"rental-server/models"
)

// TestStatDailyBuildingIndex 验证 006 能为旧版 005 建出的「无 building_id 索引」表
// 幂等补索引，并支持回退。
func TestStatDailyBuildingIndex(t *testing.T) {
	db := newTestDB(t)
	if err := db.AutoMigrate(&models.StatDaily{}); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	// 模拟旧版 005：表存在，但没有 building_id 索引
	if err := db.Migrator().DropIndex(&models.StatDaily{}, "idx_stat_daily_building_id"); err != nil {
		t.Fatalf("删除索引失败: %v", err)
	}
	if db.Migrator().HasIndex(&models.StatDaily{}, "idx_stat_daily_building_id") {
		t.Fatal("前置条件失败：索引应已被删除")
	}

	if err := upStatDailyBuildingIndex(db); err != nil {
		t.Fatalf("补索引失败: %v", err)
	}
	if !db.Migrator().HasIndex(&models.StatDaily{}, "idx_stat_daily_building_id") {
		t.Fatal("索引未创建")
	}
	// 幂等：再执行一次不应报错
	if err := upStatDailyBuildingIndex(db); err != nil {
		t.Fatalf("重复执行失败: %v", err)
	}

	if err := downStatDailyBuildingIndex(db); err != nil {
		t.Fatalf("回退失败: %v", err)
	}
	if db.Migrator().HasIndex(&models.StatDaily{}, "idx_stat_daily_building_id") {
		t.Fatal("回退后索引仍存在")
	}
}
