package services

import (
	"testing"

	"rental-server/models"
	"rental-server/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newRentControlTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := db.AutoMigrate(&models.Building{}, &models.Room{}, &models.Tenant{}, &models.RentalContract{}); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	return db
}

func TestRentControlData(t *testing.T) {
	db := newRentControlTestDB(t)

	buildings := []models.Building{
		{Name: "测试公寓", District: "测试区", Street: "测试街道", Village: "测试路", BuildingNo: "1号", Directions: "地铁测试站A口，步行300米"},
		{Name: "其他公寓", District: "测试区", Street: "其他街道"},
	}
	if err := db.Create(&buildings).Error; err != nil {
		t.Fatalf("造公寓数据失败: %v", err)
	}

	tenant := models.Tenant{Name: "测试租客", Phone: "13800000000"}
	if err := db.Create(&tenant).Error; err != nil {
		t.Fatalf("造租客数据失败: %v", err)
	}

	rooms := []models.Room{
		{BuildingID: buildings[0].ID, RoomNumber: "101", Floor: "1", Layout: "一室一厅", Status: "vacant", RentPrice: ptrFloat(1200), ManagementFee: ptrFloat(12), ElectricityUnitPrice: ptrFloat(1.2), WaterUnitPrice: ptrFloat(7)},
		{BuildingID: buildings[0].ID, RoomNumber: "102", Floor: "1", Layout: "单间", Status: "vacant", RentPrice: ptrFloat(900), ManagementFee: ptrFloat(10)},
		{BuildingID: buildings[0].ID, RoomNumber: "201", Floor: "2", Layout: "两室一厅", Status: "rented", RentPrice: ptrFloat(2000), ManagementFee: ptrFloat(20)},
		{BuildingID: buildings[0].ID, RoomNumber: "202", Floor: "2", Layout: "两室一厅", Status: "rented", RentPrice: ptrFloat(2100), ManagementFee: ptrFloat(20)},
		{BuildingID: buildings[1].ID, RoomNumber: "999", Floor: "9", Layout: "一室一厅", Status: "vacant", RentPrice: ptrFloat(888), ManagementFee: ptrFloat(8)},
	}
	if err := db.Create(&rooms).Error; err != nil {
		t.Fatalf("造房间数据失败: %v", err)
	}

	plus := func(n int) string { return utils.Now().AddDate(0, 0, n).Format("2006-01-02") }
	contracts := []models.RentalContract{
		{RoomID: rooms[2].ID, BuildingID: buildings[0].ID, TenantID: tenant.ID, RentPrice: 2000, StartDate: plus(-30), EndDate: plus(3), Status: "active"},
		{RoomID: rooms[3].ID, BuildingID: buildings[0].ID, TenantID: tenant.ID, RentPrice: 2100, StartDate: plus(-30), EndDate: plus(30), Status: "active"},
	}
	if err := db.Create(&contracts).Error; err != nil {
		t.Fatalf("造合同数据失败: %v", err)
	}

	svc := &RoomService{DB: db}
	data, err := svc.RentControlData(buildings[0].ID)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}

	if data.Building.Address != "测试区测试街道测试路1号" {
		t.Errorf("address = %q, 期望拼接后的地址", data.Building.Address)
	}
	if data.Building.Directions == "" {
		t.Error("directions 应原样返回")
	}

	if data.Total != 3 {
		t.Fatalf("total = %d, 期望 3（2 个空置 + 1 个即将到期，排除其他公寓）", data.Total)
	}

	// 空房表：只有 101、102 两行，且不带备注
	if len(data.Vacant) != 2 {
		t.Fatalf("vacant 行数 = %d, 期望 2", len(data.Vacant))
	}
	wantVacant := []string{"101", "102"}
	for i, item := range data.Vacant {
		if item.RoomNumber != wantVacant[i] {
			t.Errorf("空房表第%d行 room_number = %q, 期望 %q", i, item.RoomNumber, wantVacant[i])
		}
		if item.Remark != "" {
			t.Errorf("空房表第%d行不应有备注, 实际 %q", i, item.Remark)
		}
	}

	// 即将到期表：只有 201，备注写到期时间
	if len(data.Expiring) != 1 {
		t.Fatalf("expiring 行数 = %d, 期望 1", len(data.Expiring))
	}
	if data.Expiring[0].RoomNumber != "201" {
		t.Errorf("到期表 room_number = %q, 期望 201", data.Expiring[0].RoomNumber)
	}
	if want := "到期时间：" + plus(3); data.Expiring[0].Remark != want {
		t.Errorf("到期表 remark = %q, 期望 %q", data.Expiring[0].Remark, want)
	}

	// 水电单价随行返回，供前端生成「收费标准」默认文案
	if data.Vacant[0].ElectricityUnitPrice == nil || *data.Vacant[0].ElectricityUnitPrice != 1.2 {
		t.Errorf("空房表第0行 electricity_unit_price 未正确返回")
	}
	if data.Vacant[0].WaterUnitPrice == nil || *data.Vacant[0].WaterUnitPrice != 7 {
		t.Errorf("空房表第0行 water_unit_price 未正确返回")
	}
}

func ptrFloat(v float64) *float64 { return &v }
