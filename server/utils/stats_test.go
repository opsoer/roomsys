package utils

import (
	"testing"
	"time"

	"rental-server/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newStatsTestDB 建内存库并迁移 PV 明细、四张统计聚合表与 buildings 表
// （写入路径需要按 buildings 判断公寓存活）。
func newStatsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	ClearBuildingAliveCache()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Building{}, &models.PageView{}, &models.StatTotal{}, &models.StatIP{},
		&models.StatDailyIP{}, &models.StatDaily{},
	); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	return db
}

func statTotalPV(t *testing.T, db *gorm.DB) (int64, int64) {
	t.Helper()
	var row models.StatTotal
	if err := db.First(&row, 1).Error; err != nil {
		return -1, -1
	}
	return row.TotalPV, row.TotalLandlordView
}

func statTotalUV(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var row models.StatTotal
	if err := db.First(&row, 1).Error; err != nil {
		return -1
	}
	return row.TotalUV
}

// TestApplyPageViewBatch_Aggregates 验证一批访问记录写入后，明细、平台累计、
// 去重 IP、平台/楼栋日行五处数据一致，且重复 IP 不重复计数。
func TestApplyPageViewBatch_Aggregates(t *testing.T) {
	db := newStatsTestDB(t)
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)

	require.NoError(t, db.Create(&models.Building{Name: "楼1", Status: "active"}).Error)

	batch := []models.PageView{
		{PageType: "building_detail", BuildingID: 1, IP: "1.1.1.1", CreatedAt: today},
		{PageType: "room_detail", ResourceID: 11, BuildingID: 1, IP: "1.1.1.1", CreatedAt: today},
		{PageType: "room_detail", ResourceID: 12, BuildingID: 1, IP: "2.2.2.2", CreatedAt: today},
		{PageType: "landlord_view", BuildingID: 1, IP: "2.2.2.2", CreatedAt: today},
		{PageType: "phone_reveal", BuildingID: 1, IP: "2.2.2.2", CreatedAt: today},
		{PageType: "building_list", BuildingID: 0, IP: "3.3.3.3", CreatedAt: yesterday},
	}
	if err := ApplyPageViewBatch(db, batch); err != nil {
		t.Fatalf("ApplyPageViewBatch 失败: %v", err)
	}

	// 明细行数
	var pvCount int64
	db.Model(&models.PageView{}).Count(&pvCount)
	if pvCount != 6 {
		t.Fatalf("明细应有 6 行，实际 %d", pvCount)
	}
	// 平台累计：总 PV=6，获电=1
	pv, lv := statTotalPV(t, db)
	if pv != 6 || lv != 1 {
		t.Fatalf("stat_totals 应为 (6,1)，实际 (%d,%d)", pv, lv)
	}
	// 总 UV：本批 3 个去重 IP
	if uv := statTotalUV(t, db); uv != 3 {
		t.Fatalf("stat_totals.total_uv 应为 3，实际 %d", uv)
	}
	// 全历史去重 IP：3 个
	var ipCount int64
	db.Model(&models.StatIP{}).Count(&ipCount)
	if ipCount != 3 {
		t.Fatalf("stat_ips 应有 3 行，实际 %d", ipCount)
	}
	// 每日去重 IP：今日 2 个 + 昨日 1 个
	var dayIPCount int64
	db.Model(&models.StatDailyIP{}).Count(&dayIPCount)
	if dayIPCount != 3 {
		t.Fatalf("stat_daily_ips 应有 3 行，实际 %d", dayIPCount)
	}

	todayKey := today.Format("2006-01-02")
	yesterdayKey := yesterday.Format("2006-01-02")

	// 楼栋 1 日行：pv=5（含 phone_reveal/landlord_view），detail_pv=3，获电=1
	var b1 models.StatDaily
	if err := db.Where("day = ? AND building_id = 1", todayKey).First(&b1).Error; err != nil {
		t.Fatalf("楼栋日行缺失: %v", err)
	}
	if b1.PV != 5 || b1.DetailPV != 3 || b1.LandlordView != 1 {
		t.Fatalf("楼栋 1 日行应为 (5,3,1)，实际 (%d,%d,%d)", b1.PV, b1.DetailPV, b1.LandlordView)
	}
	// 平台日行（building_id=0）：今日 pv=5 / detail=3 / 获电=1，昨日 pv=1
	var p1, p2 models.StatDaily
	if err := db.Where("day = ? AND building_id = 0", todayKey).First(&p1).Error; err != nil {
		t.Fatalf("平台今日日行缺失: %v", err)
	}
	if p1.PV != 5 || p1.DetailPV != 3 || p1.LandlordView != 1 {
		t.Fatalf("平台今日日行应为 (5,3,1)，实际 (%d,%d,%d)", p1.PV, p1.DetailPV, p1.LandlordView)
	}
	if err := db.Where("day = ? AND building_id = 0", yesterdayKey).First(&p2).Error; err != nil {
		t.Fatalf("平台昨日日行缺失: %v", err)
	}
	if p2.PV != 1 || p2.DetailPV != 0 || p2.LandlordView != 0 {
		t.Fatalf("平台昨日日行应为 (1,0,0)，实际 (%d,%d,%d)", p2.PV, p2.DetailPV, p2.LandlordView)
	}

	// 第二批：重复 IP / 重复日行，验证去重与累加
	if err := ApplyPageViewBatch(db, []models.PageView{
		{PageType: "building_detail", BuildingID: 1, IP: "1.1.1.1", CreatedAt: today},
	}); err != nil {
		t.Fatalf("第二批写入失败: %v", err)
	}
	pv, lv = statTotalPV(t, db)
	if pv != 7 || lv != 1 {
		t.Fatalf("累计后 stat_totals 应为 (7,1)，实际 (%d,%d)", pv, lv)
	}
	if uv := statTotalUV(t, db); uv != 3 {
		t.Fatalf("重复 IP 不应累加 total_uv，实际 %d", uv)
	}
	db.Model(&models.StatIP{}).Count(&ipCount)
	if ipCount != 3 {
		t.Fatalf("重复 IP 不应新增 stat_ips 行，实际 %d", ipCount)
	}
	db.Model(&models.StatDailyIP{}).Count(&dayIPCount)
	if dayIPCount != 3 {
		t.Fatalf("重复 (day,ip) 不应新增 stat_daily_ips 行，实际 %d", dayIPCount)
	}
	if err := db.Where("day = ? AND building_id = 1", todayKey).First(&b1).Error; err != nil {
		t.Fatal(err)
	}
	if b1.PV != 6 || b1.DetailPV != 4 {
		t.Fatalf("楼栋 1 日行累加后应为 (6,4)，实际 (%d,%d)", b1.PV, b1.DetailPV)
	}
	if err := db.Where("day = ? AND building_id = 0", todayKey).First(&p1).Error; err != nil {
		t.Fatal(err)
	}
	if p1.PV != 6 || p1.DetailPV != 4 {
		t.Fatalf("平台今日日行累加后应为 (6,4)，实际 (%d,%d)", p1.PV, p1.DetailPV)
	}
}

// TestApplyPageViewBatch_DeletedBuilding 异步竞争兜底：批次落库时公寓已被软删，
// 其明细行与楼栋日行不再写回，但平台级计数（stat_totals/stat_ips/stat_daily_ips/
// stat_daily 平台行）照常累计——事件真实发生过。
func TestApplyPageViewBatch_DeletedBuilding(t *testing.T) {
	db := newStatsTestDB(t)
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, now.Location())

	dead := models.Building{Name: "已删楼", Status: "active"}
	if err := db.Create(&dead).Error; err != nil {
		t.Fatal(err)
	}
	if err := ApplyPageViewBatch(db, []models.PageView{
		{PageType: "building_detail", BuildingID: dead.ID, IP: "5.5.5.5", CreatedAt: today},
	}); err != nil {
		t.Fatalf("删除前写入失败: %v", err)
	}
	// 落库后公寓被删除（软删），随后异步批次里又来了该公寓的事件
	if err := db.Delete(&dead).Error; err != nil {
		t.Fatal(err)
	}
	// 模拟 BuildingService.Delete 的缓存失效（测试直接走 DB 软删，不会触发 service）
	InvalidateBuildingAliveCache(dead.ID)
	if err := ApplyPageViewBatch(db, []models.PageView{
		{PageType: "building_detail", BuildingID: dead.ID, IP: "5.5.5.6", CreatedAt: today},
	}); err != nil {
		t.Fatalf("删除后写入失败: %v", err)
	}

	var pvCount int64
	db.Model(&models.PageView{}).Where("building_id = ?", dead.ID).Count(&pvCount)
	if pvCount != 1 {
		t.Fatalf("删除后的明细不应写回，building_id=%d 明细应只有删除前 1 行，实际 %d", dead.ID, pvCount)
	}
	var bRows int64
	db.Model(&models.StatDaily{}).Where("building_id = ?", dead.ID).Count(&bRows)
	if bRows != 1 {
		t.Fatalf("删除后的楼栋日行不应写回，应只有删除前 1 行，实际 %d", bRows)
	}
	// 平台级计数不受影响：总 PV=2、总 IP=2、平台今日行 pv=2
	pv, _ := statTotalPV(t, db)
	if pv != 2 {
		t.Fatalf("stat_totals.total_pv 应为 2，实际 %d", pv)
	}
	var ipCount int64
	db.Model(&models.StatIP{}).Count(&ipCount)
	if ipCount != 2 {
		t.Fatalf("stat_ips 应有 2 行，实际 %d", ipCount)
	}
	if uv := statTotalUV(t, db); uv != 2 {
		t.Fatalf("stat_totals.total_uv 应保留已删楼栋事件的 2 个 IP，实际 %d", uv)
	}
	var dayIPCount int64
	db.Model(&models.StatDailyIP{}).Count(&dayIPCount)
	if dayIPCount != 2 {
		t.Fatalf("stat_daily_ips 应保留已删楼栋事件的 2 个 IP，实际 %d", dayIPCount)
	}
	var p1 models.StatDaily
	if err := db.Where("day = ? AND building_id = 0", today.Format("2006-01-02")).First(&p1).Error; err != nil {
		t.Fatalf("平台今日日行缺失: %v", err)
	}
	if p1.PV != 2 {
		t.Fatalf("平台今日日行 pv 应为 2，实际 %d", p1.PV)
	}
}
