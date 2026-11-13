package migrations

import (
	"testing"

	"rental-server/models"
)

// TestStatTotalUVBackfill 模拟旧库只有 stat_totals.total_pv/total_landlord_view 的形态，
// 验证 008 能补 total_uv 列并从 stat_ips 精确回填。
func TestStatTotalUVBackfill(t *testing.T) {
	db := newTestDB(t)
	// 旧表：没有 total_uv 列
	if err := db.Exec(`
		CREATE TABLE stat_totals (
			id integer PRIMARY KEY,
			total_pv integer NOT NULL DEFAULT 0,
			total_landlord_view integer NOT NULL DEFAULT 0,
			updated_at datetime
		)`).Error; err != nil {
		t.Fatalf("建旧表失败: %v", err)
	}
	if err := db.Exec("INSERT INTO stat_totals (id, total_pv, total_landlord_view) VALUES (1, 10, 2)").Error; err != nil {
		t.Fatalf("插入旧计数失败: %v", err)
	}
	if err := db.AutoMigrate(&models.StatIP{}); err != nil {
		t.Fatalf("建 stat_ips 失败: %v", err)
	}
	if err := db.Create(&[]models.StatIP{
		{IP: "1.1.1.1"}, {IP: "2.2.2.2"}, {IP: "3.3.3.3"},
	}).Error; err != nil {
		t.Fatalf("插入 stat_ips 失败: %v", err)
	}

	if db.Migrator().HasColumn(&models.StatTotal{}, "total_uv") {
		t.Fatal("前置条件失败：旧表不应有 total_uv 列")
	}
	if err := upStatTotalUV(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}

	var total models.StatTotal
	if err := db.First(&total, 1).Error; err != nil {
		t.Fatalf("读取计数失败: %v", err)
	}
	if total.TotalUV != 3 {
		t.Fatalf("total_uv 应回填为 3，实际 %d", total.TotalUV)
	}
	if total.TotalPV != 10 || total.TotalLandlordView != 2 {
		t.Fatalf("其他计数不应变化，实际 (%d,%d)", total.TotalPV, total.TotalLandlordView)
	}
	// 幂等：再执行一次不应报错
	if err := upStatTotalUV(db); err != nil {
		t.Fatalf("重复执行失败: %v", err)
	}

	if err := downStatTotalUV(db); err != nil {
		t.Fatalf("回退失败: %v", err)
	}
	if db.Migrator().HasColumn(&models.StatTotal{}, "total_uv") {
		t.Fatal("回退后 total_uv 仍存在")
	}
}
