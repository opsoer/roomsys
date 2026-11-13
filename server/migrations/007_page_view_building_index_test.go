package migrations

import (
	"testing"

	"rental-server/models"
)

// TestPageViewBuildingIndex 验证 007 能为旧库补 page_views.building_id 独立索引，
// 并支持回退。
func TestPageViewBuildingIndex(t *testing.T) {
	db := newTestDB(t)
	if err := db.AutoMigrate(&models.PageView{}); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	// 模拟旧库：只有 idx_pv_query，没有 building_id 独立索引
	if err := db.Migrator().DropIndex(&models.PageView{}, "idx_pv_building"); err != nil {
		t.Fatalf("删除索引失败: %v", err)
	}
	if db.Migrator().HasIndex(&models.PageView{}, "idx_pv_building") {
		t.Fatal("前置条件失败：索引应已被删除")
	}

	if err := upPageViewBuildingIndex(db); err != nil {
		t.Fatalf("补索引失败: %v", err)
	}
	if !db.Migrator().HasIndex(&models.PageView{}, "idx_pv_building") {
		t.Fatal("索引未创建")
	}
	// 幂等：再执行一次不应报错
	if err := upPageViewBuildingIndex(db); err != nil {
		t.Fatalf("重复执行失败: %v", err)
	}

	if err := downPageViewBuildingIndex(db); err != nil {
		t.Fatalf("回退失败: %v", err)
	}
	if db.Migrator().HasIndex(&models.PageView{}, "idx_pv_building") {
		t.Fatal("回退后索引仍存在")
	}
}
