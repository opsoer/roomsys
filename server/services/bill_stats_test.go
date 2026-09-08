package services

import (
	"testing"
	"time"

	"rental-server/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// statsMonthOffset 返回相对当前月份偏移 n 个月的 "YYYY-MM"（固定用 1 号，规避月末天数归一化）
func statsMonthOffset(months int) string {
	return time.Now().AddDate(0, months, 0).Format("2006-01")
}

// statsDateOffset 返回相对当前月份偏移 n 个月的月初 "YYYY-MM-DD"
func statsDateOffset(months int) string {
	return statsMonthOffset(months) + "-01"
}

func newBillStatsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := db.AutoMigrate(&models.Bill{}); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	return db
}

func TestGetStats(t *testing.T) {
	db := newBillStatsTestDB(t)
	svc := NewBillService(db)
	bills := []models.Bill{
		{BillNo: "B1", Type: "income", Subtype: "租金", Amount: 1000, BuildingID: 1, BillDate: "2026-01-05"},
		{BillNo: "B2", Type: "income", Subtype: "租金", Amount: 1200, BuildingID: 1, BillDate: "2026-01-20"},
		{BillNo: "B3", Type: "income", Subtype: "押金", Amount: 500, BuildingID: 1, BillDate: "2026-01-21"},
		{BillNo: "B4", Type: "expense", Subtype: "维修费", Amount: 300, BuildingID: 1, BillDate: "2026-01-10"},
		{BillNo: "B5", Type: "income", Subtype: "租金", Amount: 900, BuildingID: 2, BillDate: "2026-01-05"},
		{BillNo: "B6", Type: "income", Subtype: "租金", Amount: 800, BuildingID: 1, BillDate: "2026-02-05"},
		{BillNo: "B7", Type: "income", Subtype: "租金", Amount: 777, BuildingID: 1, BillDate: "2025-01-05"},
	}
	if err := db.Create(&bills).Error; err != nil {
		t.Fatalf("造账单数据失败: %v", err)
	}

	stats, err := svc.GetStats(1, "2026-01", "")
	if err != nil {
		t.Fatalf("GetStats 失败: %v", err)
	}
	if got := stats["total_income"].(float64); got != 2700 {
		t.Errorf("total_income = %v, want 2700", got)
	}
	if got := stats["total_expense"].(float64); got != 300 {
		t.Errorf("total_expense = %v, want 300", got)
	}
	if got := stats["net_profit"].(float64); got != 2400 {
		t.Errorf("net_profit = %v, want 2400", got)
	}
	if got := stats["bill_count"].(int64); got != 4 {
		t.Errorf("bill_count = %v, want 4（仅统计本公寓本月，不含软删除外的他月/他楼栋）", got)
	}
	// 明细须按金额降序：租金 2200 在押金 500 之前
	income := stats["income_detail"].([]map[string]interface{})
	if len(income) != 2 || income[0]["subtype"].(string) != "租金" || income[0]["total"].(float64) != 2200 {
		t.Errorf("income_detail 排序或内容错误: %v", income)
	}
}

func TestGetTrendZeroFillAndYoY(t *testing.T) {
	db := newBillStatsTestDB(t)
	svc := NewBillService(db)

	// 以当前时间为基准造 14 个月前和 2 个月前的数据：
	// 14 个月前在展示窗口（近12个月）之外但用于其 12 个月后月份的同比
	rows := []models.Bill{
		{BillNo: "T1", Type: "income", Subtype: "租金", Amount: 1000, BuildingID: 1, BillDate: statsDateOffset(-14)},
		{BillNo: "T2", Type: "income", Subtype: "租金", Amount: 1500, BuildingID: 1, BillDate: statsDateOffset(-2)},
		{BillNo: "T3", Type: "expense", Subtype: "维修费", Amount: 200, BuildingID: 1, BillDate: statsDateOffset(-2)},
		{BillNo: "T4", Type: "income", Subtype: "租金", Amount: 999, BuildingID: 2, BillDate: statsDateOffset(-2)},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("造账单数据失败: %v", err)
	}

	result, err := svc.GetTrend(1, 12)
	if err != nil {
		t.Fatalf("GetTrend 失败: %v", err)
	}
	months := result["months"].([]map[string]interface{})
	growth := result["growth"].([]map[string]interface{})

	// 展示窗口固定输出近 12 个自然月（含当月），无账单的月份也要补零占位
	if len(months) != 12 {
		t.Fatalf("months 长度 = %d, want 12", len(months))
	}
	if len(growth) != 12 {
		t.Fatalf("growth 长度 = %d, want 12", len(growth))
	}

	// 找到 2 个月前那个月的增长率：环比基准（3 个月前）无数据 → null；
	// 同比基准是 14 个月前的 1000：1500 vs 1000 => +50%
	monthWanted := statsMonthOffset(-2)
	var target map[string]interface{}
	for _, g := range growth {
		if g["month"].(string) == monthWanted {
			target = g
			break
		}
	}
	if target == nil {
		t.Fatalf("growth 缺少 %s 的记录", monthWanted)
	}
	if target["income_mom"] != nil {
		t.Errorf("上期收入为 0 时环比应为 null, got %v", target["income_mom"])
	}
	if got, ok := target["income_yoy"].(float64); !ok || got != 50.0 {
		t.Errorf("income_yoy = %v, want 50", target["income_yoy"])
	}

	// 汇总窗口外的 14 个月前数据不应出现在 months 里
	for _, m := range months {
		if m["month"].(string) == statsMonthOffset(-14) {
			t.Errorf("展示窗口不应包含 14 个月前: %v", m["month"])
		}
	}
}

func TestGetTrendSoftDeletedExcluded(t *testing.T) {
	db := newBillStatsTestDB(t)
	svc := NewBillService(db)
	bill := models.Bill{BillNo: "D1", Type: "income", Subtype: "租金", Amount: 1000, BuildingID: 1, BillDate: statsDateOffset(-1)}
	if err := db.Create(&bill).Error; err != nil {
		t.Fatalf("造账单数据失败: %v", err)
	}
	if err := db.Delete(&bill).Error; err != nil {
		t.Fatalf("软删除失败: %v", err)
	}
	stats, err := svc.GetStats(1, statsDateOffset(-1)[:7], "")
	if err != nil {
		t.Fatalf("GetStats 失败: %v", err)
	}
	if got := stats["total_income"].(float64); got != 0 {
		t.Errorf("软删除账单不应计入统计, total_income = %v", got)
	}
}
