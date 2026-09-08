package services

import (
	"testing"

	"rental-server/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newContractTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := db.AutoMigrate(&models.Room{}, &models.Tenant{}, &models.RentalContract{}); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	return db
}

// seedContractData 造数据：公寓1有3个房间、4份合同（含预订/已结束），公寓2有1份合同用于验证隔离
func seedContractData(t *testing.T, db *gorm.DB) {
	t.Helper()
	rooms := []models.Room{
		{BuildingID: 1, RoomNumber: "101", Floor: "1", Layout: "一室一厅", Status: "rented"},
		{BuildingID: 1, RoomNumber: "102", Floor: "1", Layout: "一室一厅", Status: "vacant"},
		{BuildingID: 1, RoomNumber: "201", Floor: "2", Layout: "两室一厅", Status: "rented"},
		{BuildingID: 2, RoomNumber: "999", Floor: "9", Layout: "一室一厅", Status: "rented"},
	}
	if err := db.Create(&rooms).Error; err != nil {
		t.Fatalf("造房间数据失败: %v", err)
	}
	tenants := []models.Tenant{
		{Name: "张三", Phone: "13800000001"},
		{Name: "李四", Phone: "13800000002"},
		{Name: "王五", Phone: "13900000003"},
	}
	if err := db.Create(&tenants).Error; err != nil {
		t.Fatalf("造租客数据失败: %v", err)
	}
	contracts := []models.RentalContract{
		{RoomID: rooms[0].ID, BuildingID: 1, TenantID: tenants[0].ID, RentPrice: 1000, StartDate: "2024-01-01", EndDate: "2024-12-31", Status: "ended"},
		{RoomID: rooms[0].ID, BuildingID: 1, TenantID: tenants[1].ID, RentPrice: 1100, StartDate: "2025-01-01", Status: "active"},
		{RoomID: rooms[2].ID, BuildingID: 1, TenantID: tenants[2].ID, RentPrice: 2000, StartDate: "2025-06-01", Status: "reserved"},
		{RoomID: rooms[1].ID, BuildingID: 1, TenantID: tenants[1].ID, RentPrice: 900, StartDate: "2023-05-01", EndDate: "2023-08-31", Status: "cancelled"},
		{RoomID: rooms[3].ID, BuildingID: 2, TenantID: tenants[0].ID, RentPrice: 888, StartDate: "2025-01-01", Status: "active"},
	}
	if err := db.Create(&contracts).Error; err != nil {
		t.Fatalf("造合同数据失败: %v", err)
	}
}

func TestListBuildingContractsDefault(t *testing.T) {
	db := newContractTestDB(t)
	seedContractData(t, db)
	svc := &RoomService{DB: db}

	contracts, total, err := svc.ListBuildingContracts(1, 1, 0, 20, 0, "", "", "")
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if total != 4 {
		t.Errorf("total = %d, 期望 4（不包含其他公寓的合同）", total)
	}
	// 从新到旧：2025-06-01, 2025-01-01, 2024-01-01, 2023-05-01
	wantOrder := []string{"2025-06-01", "2025-01-01", "2024-01-01", "2023-05-01"}
	for i, c := range contracts {
		if c.StartDate != wantOrder[i] {
			t.Errorf("第%d条 start_date = %s, 期望 %s", i, c.StartDate, wantOrder[i])
		}
	}
	if contracts[0].Tenant.Name == "" || contracts[0].Room.RoomNumber == "" {
		t.Error("Preload Tenant/Room 未生效")
	}
}

func TestListBuildingContractsFilters(t *testing.T) {
	db := newContractTestDB(t)
	seedContractData(t, db)
	svc := &RoomService{DB: db}

	// 房间筛选
	got, total, _ := svc.ListBuildingContracts(1, 1, 0, 20, 1, "", "", "")
	if total != 2 {
		t.Errorf("room_id 筛选 total = %d, 期望 2", total)
	}
	_ = got

	// 状态筛选
	_, total, _ = svc.ListBuildingContracts(1, 1, 0, 20, 0, "ended", "", "")
	if total != 1 {
		t.Errorf("status=ended total = %d, 期望 1", total)
	}

	// 关键词：租客姓名
	_, total, _ = svc.ListBuildingContracts(1, 1, 0, 20, 0, "", "李四", "")
	if total != 2 {
		t.Errorf("keyword=李四 total = %d, 期望 2", total)
	}

	// 关键词：电话
	_, total, _ = svc.ListBuildingContracts(1, 1, 0, 20, 0, "", "13900000003", "")
	if total != 1 {
		t.Errorf("keyword=电话 total = %d, 期望 1", total)
	}

	// 关键词：房间号
	_, total, _ = svc.ListBuildingContracts(1, 1, 0, 20, 0, "", "201", "")
	if total != 1 {
		t.Errorf("keyword=房间号 total = %d, 期望 1", total)
	}

	// 组合：房间 + 状态
	_, total, _ = svc.ListBuildingContracts(1, 1, 0, 20, 1, "active", "", "")
	if total != 1 {
		t.Errorf("room_id+status total = %d, 期望 1", total)
	}
}

func TestListBuildingContractsCursorPaging(t *testing.T) {
	db := newContractTestDB(t)
	seedContractData(t, db)
	svc := &RoomService{DB: db}

	first, total, err := svc.ListBuildingContracts(1, 1, 0, 2, 0, "", "", "")
	if err != nil {
		t.Fatalf("第一页查询失败: %v", err)
	}
	if total != 4 {
		t.Errorf("total = %d, 期望 4", total)
	}
	if len(first) != 2 {
		t.Fatalf("第一页返回 %d 条, 期望 2", len(first))
	}

	last := first[len(first)-1]
	second, total2, err := svc.ListBuildingContracts(1, 1, int(last.ID), 2, 0, "", "", last.StartDate)
	if err != nil {
		t.Fatalf("游标页查询失败: %v", err)
	}
	if total2 != -1 {
		t.Errorf("游标页 total = %d, 期望 -1（不重复计数）", total2)
	}
	if len(second) != 2 {
		t.Fatalf("第二页返回 %d 条, 期望 2", len(second))
	}
	seen := map[uint]bool{}
	for _, c := range append(append([]models.RentalContract{}, first...), second...) {
		if seen[c.ID] {
			t.Errorf("游标分页出现重复合同 id=%d", c.ID)
		}
		seen[c.ID] = true
	}
}
