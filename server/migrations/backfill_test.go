package migrations

import (
	"testing"
	"time"

	"rental-server/models"
)

// TestStatAggregatesBackfill 模拟老库升级：先只有 page_views 表且带历史数据，
// 再跑全量迁移，验证迁移 005 正确建表并把历史数据回填进聚合表。
func TestStatAggregatesBackfill(t *testing.T) {
	db := newTestDB(t)

	// 老库只有 page_views 明细与 buildings 表（001 AutoMigrate 会补齐其余表，不影响已有数据）
	if err := db.AutoMigrate(&models.Building{}, &models.PageView{}); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)
	rows := []models.PageView{
		{PageType: "building_detail", BuildingID: 1, IP: "A", CreatedAt: today},
		{PageType: "room_detail", BuildingID: 1, IP: "B", CreatedAt: today},
		{PageType: "landlord_view", BuildingID: 1, IP: "B", CreatedAt: today},
		{PageType: "phone_reveal", BuildingID: 1, IP: "C", CreatedAt: today},
		{PageType: "room_detail", BuildingID: 2, IP: "A", CreatedAt: today},
		{PageType: "building_list", BuildingID: 0, IP: "A", CreatedAt: yesterday},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("插入历史明细失败: %v", err)
	}
	// 楼栋 1/2 存活（回填按存活楼栋 JOIN）；楼栋 3 已软删——它的历史明细不得产生楼栋行
	for _, b := range []models.Building{
		{ID: 1, Name: "楼1", Status: "active"},
		{ID: 2, Name: "楼2", Status: "active"},
		{ID: 3, Name: "已删楼", Status: "active"},
	} {
		if err := db.Create(&b).Error; err != nil {
			t.Fatalf("插入楼栋失败: %v", err)
		}
	}
	if err := db.Delete(&models.Building{}, 3).Error; err != nil {
		t.Fatalf("软删楼栋 3 失败: %v", err)
	}
	deletedBuildingViews := []models.PageView{
		{PageType: "building_detail", BuildingID: 3, IP: "D", CreatedAt: today},
	}
	if err := db.Create(&deletedBuildingViews).Error; err != nil {
		t.Fatalf("插入已删楼栋明细失败: %v", err)
	}

	if _, err := Apply(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}

	// stat_totals：总 PV=7（含已删楼栋的历史事件），获电=1
	var totals models.StatTotal
	if err := db.First(&totals, 1).Error; err != nil {
		t.Fatalf("stat_totals 计数行缺失: %v", err)
	}
	if totals.TotalPV != 7 || totals.TotalLandlordView != 1 {
		t.Fatalf("stat_totals 应为 (7,1)，实际 (%d,%d)", totals.TotalPV, totals.TotalLandlordView)
	}
	// stat_ips：A/B/C/D 去重 4 个（已删楼栋的历史 IP 也计入平台口径，只增不减）
	var ipCount int64
	db.Model(&models.StatIP{}).Count(&ipCount)
	if ipCount != 4 {
		t.Fatalf("stat_ips 应有 4 行，实际 %d", ipCount)
	}
	// 已软删楼栋（id=3）不得产生楼栋日行
	var orphanRows int64
	db.Model(&models.StatDaily{}).Where("building_id = 3").Count(&orphanRows)
	if orphanRows != 0 {
		t.Fatalf("已软删楼栋不应回填楼栋日行，实际 %d 行", orphanRows)
	}
	// stat_daily_ips：(今日,A/B/C/D) + (昨日,A) = 5
	var dayIPCount int64
	db.Model(&models.StatDailyIP{}).Count(&dayIPCount)
	if dayIPCount != 5 {
		t.Fatalf("stat_daily_ips 应有 5 行，实际 %d", dayIPCount)
	}

	todayKey := today.Format("2006-01-02")
	yesterdayKey := yesterday.Format("2006-01-02")

	// 楼栋日行：楼栋 1 今日 (pv=4, detail=2, lv=1)；楼栋 2 今日 (1,1,0)
	var b1, b2 models.StatDaily
	if err := db.Where("day = ? AND building_id = 1", todayKey).First(&b1).Error; err != nil {
		t.Fatalf("楼栋 1 日行缺失: %v", err)
	}
	if b1.PV != 4 || b1.DetailPV != 2 || b1.LandlordView != 1 {
		t.Fatalf("楼栋 1 日行应为 (4,2,1)，实际 (%d,%d,%d)", b1.PV, b1.DetailPV, b1.LandlordView)
	}
	if err := db.Where("day = ? AND building_id = 2", todayKey).First(&b2).Error; err != nil {
		t.Fatalf("楼栋 2 日行缺失: %v", err)
	}
	if b2.PV != 1 || b2.DetailPV != 1 || b2.LandlordView != 0 {
		t.Fatalf("楼栋 2 日行应为 (1,1,0)，实际 (%d,%d,%d)", b2.PV, b2.DetailPV, b2.LandlordView)
	}
	// 平台日行：今日 (6,4,1)（含已删楼栋的历史事件——平台总量只增不减）、昨日 (1,0,0)
	var p1, p2 models.StatDaily
	if err := db.Where("day = ? AND building_id = 0", todayKey).First(&p1).Error; err != nil {
		t.Fatalf("平台今日日行缺失: %v", err)
	}
	if p1.PV != 6 || p1.DetailPV != 4 || p1.LandlordView != 1 {
		t.Fatalf("平台今日日行应为 (6,4,1)，实际 (%d,%d,%d)", p1.PV, p1.DetailPV, p1.LandlordView)
	}
	if err := db.Where("day = ? AND building_id = 0", yesterdayKey).First(&p2).Error; err != nil {
		t.Fatalf("平台昨日日行缺失: %v", err)
	}
	if p2.PV != 1 || p2.DetailPV != 0 || p2.LandlordView != 0 {
		t.Fatalf("平台昨日日行应为 (1,0,0)，实际 (%d,%d,%d)", p2.PV, p2.DetailPV, p2.LandlordView)
	}
}

// TestStatAggregatesBackfillSkipsEmpty 新库（001 建表后 page_views 为空）不产生回填数据。
func TestStatAggregatesBackfillSkipsEmpty(t *testing.T) {
	db := newTestDB(t)
	if _, err := Apply(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	var totalRows int64
	db.Table("stat_totals").Count(&totalRows)
	if totalRows != 0 {
		t.Fatalf("空库不应回填 stat_totals，实际 %d 行", totalRows)
	}
	var dailyRows int64
	db.Table("stat_daily").Count(&dailyRows)
	if dailyRows != 0 {
		t.Fatalf("空库不应回填 stat_daily，实际 %d 行", dailyRows)
	}
}
