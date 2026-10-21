package handlers

import (
	"testing"

	"rental-server/models"
	"rental-server/utils"

	"gorm.io/gorm"
)

// 造一份活跃合同（起止日可控）。
func seedRentContract(t *testing.T, db *gorm.DB, id, roomID, buildingID, tenantID uint, start, end string, rent, mgmt float64) {
	t.Helper()
	ct := models.RentalContract{
		ID: id, RoomID: roomID, BuildingID: buildingID, TenantID: tenantID,
		RentPrice: rent, ManagementFee: mgmt,
		StartDate: start, EndDate: end, Status: "active",
	}
	if err := db.Create(&ct).Error; err != nil {
		t.Fatalf("造合同失败: %v", err)
	}
}

// seedRoomWithMgmt 造房间并设置管理费。
//
// 口径说明：月度账单的管理费取自「房间」而非合同（见 AutoCreateMonthlyRentBills），
// 因此计费相关测试必须给房间设置 management_fee，否则账单只含租金。
func seedRoomWithMgmt(t *testing.T, db *gorm.DB, id, buildingID uint, status string, mgmt float64) models.Room {
	t.Helper()
	room := seedRoom(t, db, id, buildingID, status)
	m := mgmt
	if err := db.Model(&models.Room{}).Where("id = ?", id).Update("management_fee", m).Error; err != nil {
		t.Fatalf("设置房间管理费失败: %v", err)
	}
	return room
}

// 自动生成月度租金账单：活跃合同缺当月账单时应补生成，金额为当月租金+管理费。
func TestAutoCreateMonthlyRentBillsGenerates(t *testing.T) {
	e := newTestEnv(t)
	seedBuilding(t, e.db, 1)
	seedRoomWithMgmt(t, e.db, 1, 1, "rented", 200)
	tenant := seedTenant(t, e.db, "张三")
	now := utils.Now()
	prev := now.AddDate(0, -1, 0)
	start := utils.FirstDayOfMonth(prev).Format("2006-01-02")
	end := utils.LastDayOfMonth(now).Format("2006-01-02")
	seedRentContract(t, e.db, 1, 1, 1, tenant.ID, start, end, 2000, 200)

	result := AutoCreateMonthlyRentBills(e.db)
	if result == "" {
		t.Fatal("应返回执行结果")
	}

	if n := countBills(t, e.db, "subtype = ?", "租金"); n != 1 {
		t.Fatalf("应生成 1 条租金账单，实际 %d 条", n)
	}
	var bill models.Bill
	e.db.Where("subtype = ?", "租金").First(&bill)
	if bill.Amount != 2200 {
		t.Fatalf("整月账单金额应为 2200（租金2000+管理费200），实际 %v", bill.Amount)
	}
	if bill.Type != "income" {
		t.Fatalf("账单类型应为 income，实际 %q", bill.Type)
	}
	if bill.BuildingID != 1 || bill.RoomID == nil || *bill.RoomID != 1 {
		t.Fatalf("账单归属不符: %+v", bill)
	}
}

// 自动生成月度租金账单：已有当月账单时不重复生成（幂等，防重复收租）。
func TestAutoCreateMonthlyRentBillsIsIdempotent(t *testing.T) {
	e := newTestEnv(t)
	seedBuilding(t, e.db, 1)
	seedRoomWithMgmt(t, e.db, 1, 1, "rented", 200)
	tenant := seedTenant(t, e.db, "张三")
	now := utils.Now()
	prev := now.AddDate(0, -1, 0)
	start := utils.FirstDayOfMonth(prev).Format("2006-01-02")
	end := utils.LastDayOfMonth(now).Format("2006-01-02")
	seedRentContract(t, e.db, 1, 1, 1, tenant.ID, start, end, 2000, 200)

	AutoCreateMonthlyRentBills(e.db)
	first := countBills(t, e.db, "subtype = ?", "租金")
	if first != 1 {
		t.Fatalf("首次应生成 1 条账单，实际 %d 条", first)
	}

	AutoCreateMonthlyRentBills(e.db)
	second := countBills(t, e.db, "subtype = ?", "租金")
	if second != first {
		t.Fatalf("重复执行不应新增账单，前 %d 条后 %d 条", first, second)
	}
}

// 自动生成月度租金账单：合同已在上月之前结束的，本月不生成账单。
func TestAutoCreateMonthlyRentBillsSkipsEndedContract(t *testing.T) {
	e := newTestEnv(t)
	seedBuilding(t, e.db, 1)
	seedRoomWithMgmt(t, e.db, 1, 1, "rented", 200)
	tenant := seedTenant(t, e.db, "张三")
	now := utils.Now()
	longAgo := now.AddDate(0, -3, 0)
	seedRentContract(t, e.db, 1, 1, 1, tenant.ID,
		utils.FirstDayOfMonth(longAgo).Format("2006-01-02"),
		utils.LastDayOfMonth(longAgo).Format("2006-01-02"),
		2000, 200)

	AutoCreateMonthlyRentBills(e.db)
	if n := countBills(t, e.db, "subtype = ?", "租金"); n != 0 {
		t.Fatalf("已结束的合同不应生成账单，实际 %d 条", n)
	}
}

// 自动生成月度租金账单：非整月起租按天折算。
func TestAutoCreateMonthlyRentBillsProrates(t *testing.T) {
	e := newTestEnv(t)
	seedBuilding(t, e.db, 1)
	seedRoomWithMgmt(t, e.db, 1, 1, "rented", 200)
	tenant := seedTenant(t, e.db, "张三")
	now := utils.Now()
	monthStart := utils.FirstDayOfMonth(now)
	monthEnd := utils.LastDayOfMonth(now)
	// 月中起租：本月只算后半段
	start := monthStart.AddDate(0, 0, 14)
	seedRentContract(t, e.db, 1, 1, 1, tenant.ID,
		start.Format("2006-01-02"), monthEnd.Format("2006-01-02"), 2000, 200)

	AutoCreateMonthlyRentBills(e.db)
	if n := countBills(t, e.db, "subtype = ?", "租金"); n != 1 {
		t.Fatalf("应生成 1 条账单，实际 %d 条", n)
	}
	var bill models.Bill
	e.db.Where("subtype = ?", "租金").First(&bill)
	daysInMonth := monthEnd.Day()
	wantRent := utils.CalcProratedAmount(2000, start, monthEnd, daysInMonth)
	wantMgmt := utils.CalcProratedAmount(200, start, monthEnd, daysInMonth)
	want := float64(int((wantRent+wantMgmt)*100)) / 100
	if bill.Amount != want {
		t.Fatalf("按天折算金额应为 %v，实际 %v", want, bill.Amount)
	}
}
