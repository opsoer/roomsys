package testkit

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"rental-server/models"
	"rental-server/utils"
)

// Fixture 回归测试基线数据（与 docs/testcases/01 对齐）。
type Fixture struct {
	T         *testing.T
	App       *App
	Today     string
	ThisMonth string
	LastMonth string

	Root models.User

	A, B, C, D models.Building

	AUser, APlain, BUser, CUser, DUser, Disabled models.User

	Rooms map[string]models.Room

	TokenRoot   string
	TokenA      string
	TokenAPlain string
	TokenB      string
	TokenC      string
	TokenD      string

	ContractA102       models.RentalContract
	ContractA103       models.RentalContract
	ContractA104       models.RentalContract
	ContractA105       models.RentalContract
	ContractA105Future models.RentalContract
	ContractB202       models.RentalContract

	BillLastIncome  models.Bill
	BillLastExpense models.Bill
	BillThisIncome  models.Bill
	BillThisExpense models.Bill
	BillInject      models.Bill
	BillSoftDeleted models.Bill

	Shareholder1 models.Shareholder
	Shareholder2 models.Shareholder
	TaskA104     models.Task
}

func ptr[T any](v T) *T { return &v }

// SeedBase 创建与测试文档一致的 A/B/C/D、账号、房间、合同、账单、股东、待办。
func (a *App) SeedBase(t *testing.T) *Fixture {
	t.Helper()

	now := utils.Now()
	f := &Fixture{
		T:         t,
		App:       a,
		Today:     now.Format("2006-01-02"),
		ThisMonth: now.Format("2006-01"),
		LastMonth: now.AddDate(0, -1, 0).Format("2006-01"),
		Rooms:     map[string]models.Room{},
	}
	day := func(month string, d int) string { return fmt.Sprintf("%s-%02d", month, d) }
	plus := func(n int) string { return now.AddDate(0, 0, n).Format("2006-01-02") }
	minus := func(n int) string { return now.AddDate(0, 0, -n).Format("2006-01-02") }

	var root models.User
	require.NoError(t, a.DB.Where("username = ?", "root").First(&root).Error)
	f.Root = root

	f.A = a.mustBuilding(t, f.Root, "测试公寓A-全套餐", "full", 5000, "南山区", "科技园", "测试花园", "A座", plus(365))
	f.B = a.mustBuilding(t, f.Root, "测试公寓B-越权", "full", 4000, "南山区", "科技园", "测试花园", "B座", plus(365))
	f.C = a.mustBuilding(t, f.Root, "测试公寓C-基础套餐", "basic", 3000, "宝安区", "石岩街道", "测试村", "C座", plus(365))
	f.D = a.mustBuilding(t, f.Root, "测试公寓D-到期", "full", 2000, "龙岗区", "坂田街道", "测试小区", "D座", plus(10))

	f.AUser = a.mustUser(t, "aadmin", "Test12345", "building_admin", &f.A.ID, true)
	f.APlain = a.mustUser(t, "aadmin2", "Test12345", "admin", &f.A.ID, true)
	f.BUser = a.mustUser(t, "badmin", "Test12345", "building_admin", &f.B.ID, true)
	f.CUser = a.mustUser(t, "cadmin", "Test12345", "building_admin", &f.C.ID, true)
	f.DUser = a.mustUser(t, "dadmin", "Test12345", "building_admin", &f.D.ID, true)
	f.Disabled = a.mustUser(t, "disabled", "Test12345", "admin", &f.A.ID, false)

	f.TokenRoot = a.TokenFor(f.Root)
	f.TokenA = a.TokenFor(f.AUser)
	f.TokenAPlain = a.TokenFor(f.APlain)
	f.TokenB = a.TokenFor(f.BUser)
	f.TokenC = a.TokenFor(f.CUser)
	f.TokenD = a.TokenFor(f.DUser)

	// A 栋房间
	f.Rooms["A101"] = a.mustRoom(t, f.A, "A101", "1", "单间", 2000, 200)
	f.Rooms["A102"] = a.mustRoom(t, f.A, "A102", "1", "一室一厅", 2000, 200)
	f.Rooms["A103"] = a.mustRoom(t, f.A, "A103", "2", "单间", 2100, 200)
	f.Rooms["A104"] = a.mustRoom(t, f.A, "A104", "2", "单间", 1800, 180)
	f.Rooms["A105"] = a.mustRoom(t, f.A, "A105", "3", "两房", 2200, 220)
	f.Rooms["A106"] = a.mustRoom(t, f.A, "A106", "3", "单间", 2000, 200)
	f.Rooms["A107"] = a.mustRoom(t, f.A, "A107", "4", "两房", 2000, 200)
	// B/C/D 房间
	f.Rooms["B201"] = a.mustRoom(t, f.B, "B201", "1", "单间", 3000, 300)
	f.Rooms["B202"] = a.mustRoom(t, f.B, "B202", "1", "单间", 3000, 300)
	f.Rooms["C101"] = a.mustRoom(t, f.C, "C101", "1", "单间", 1500, 150)
	f.Rooms["D101"] = a.mustRoom(t, f.D, "D101", "1", "单间", 1600, 160)

	// 合同
	f.ContractA102 = a.mustContract(t, f.A, f.Rooms["A102"], "张三", "13900000001", 2000, 200, 2000, minus(30), plus(335), "active", 0)
	f.ContractA103 = a.mustContract(t, f.A, f.Rooms["A103"], "李四", "13900000002", 2100, 200, 2100, plus(31), plus(395), "reserved", 500)
	f.ContractA104 = a.mustContract(t, f.A, f.Rooms["A104"], "王五", "13900000003", 1800, 180, 1800, minus(300), plus(10), "active", 0)
	f.ContractA105 = a.mustContract(t, f.A, f.Rooms["A105"], "赵六", "13900000004", 2200, 220, 2200, minus(60), plus(30), "active", 0)
	f.ContractA105Future = a.mustContract(t, f.A, f.Rooms["A105"], "钱七", "13900000005", 2300, 230, 2300, plus(31), plus(395), "reserved", 600)
	f.ContractB202 = a.mustContract(t, f.B, f.Rooms["B202"], "B租客", "13900000006", 3000, 300, 3000, minus(20), plus(345), "active", 0)

	// 房间状态与合同保持一致
	f.setStatus(t, "A102", "rented")
	f.setStatus(t, "A104", "rented")
	f.setStatus(t, "A105", "rented")
	f.setStatus(t, "A103", "reserved")
	f.setStatus(t, "B202", "rented")

	// 本月已存在的整月租金账单（A104 故意不建，供 rent_bills 任务测试）
	a.mustBill(t, f.A.ID, roomID(f, "A102"), "income", "租金", 2200, day(f.ThisMonth, 1), "A102 本月租金", "FIX-A102")
	a.mustBill(t, f.A.ID, roomID(f, "A105"), "income", "租金", 2420, day(f.ThisMonth, 1), "A105 本月租金", "FIX-A105")
	a.mustBill(t, f.B.ID, roomID(f, "B202"), "income", "租金", 3300, day(f.ThisMonth, 1), "B202 本月租金", "FIX-B202")

	// 财务基线
	f.BillLastIncome = a.mustBill(t, f.A.ID, nil, "income", "租金", 10000, day(f.LastMonth, 5), "上月租金", "FIX-LAST-IN")
	f.BillLastExpense = a.mustBill(t, f.A.ID, nil, "expense", "维修", 2000, day(f.LastMonth, 8), "上月维修", "FIX-LAST-EXP")
	f.BillThisIncome = a.mustBill(t, f.A.ID, nil, "income", "租金", 2000, day(f.ThisMonth, 5), "本月租金", "FIX-THIS-IN")
	f.BillThisExpense = a.mustBill(t, f.A.ID, nil, "expense", "维修", 300, day(f.ThisMonth, 8), "本月维修", "FIX-THIS-EXP")
	f.BillInject = a.mustBill(t, f.A.ID, nil, "income", "其他", 100, day(f.ThisMonth, 10), "=2+3", "FIX-INJECT")

	// 股东与上月结算（净利 8000 -> 4800/3200）
	f.Shareholder1 = models.Shareholder{BuildingID: f.A.ID, Name: "股东甲", ShareRatio: 60}
	require.NoError(t, a.DB.Create(&f.Shareholder1).Error)
	f.Shareholder2 = models.Shareholder{BuildingID: f.A.ID, Name: "股东乙", ShareRatio: 40}
	require.NoError(t, a.DB.Create(&f.Shareholder2).Error)
	require.NoError(t, a.DB.Create(&models.Dividend{
		BuildingID: f.A.ID, SettleMonth: f.LastMonth,
		TotalIncome: 10000, TotalExpense: 2000, NetProfit: 8000,
		ShareholderID: f.Shareholder1.ID, DividendAmount: 4800, PaidStatus: "unpaid",
	}).Error)
	require.NoError(t, a.DB.Create(&models.Dividend{
		BuildingID: f.A.ID, SettleMonth: f.LastMonth,
		TotalIncome: 10000, TotalExpense: 2000, NetProfit: 8000,
		ShareholderID: f.Shareholder2.ID, DividendAmount: 3200, PaidStatus: "unpaid",
	}).Error)

	// A104 到期退租待办
	f.TaskA104 = models.Task{
		BuildingID: &f.A.ID, Title: "房间到期退租", Type: "expired_room",
		Status: "pending", RoomID: roomID(f, "A104"), Scope: "building", Priority: "medium",
	}
	require.NoError(t, a.DB.Create(&f.TaskA104).Error)

	// 软删除账单（编号回归夹具）
	soft := a.mustBill(t, f.A.ID, nil, "income", "其他", 1, f.Today, "软删除编号回归", "FIX-SOFT")
	require.NoError(t, a.DB.Delete(&soft).Error)
	f.BillSoftDeleted = soft

	return f
}

func (a *App) mustUser(t *testing.T, username, password, role string, buildingID *uint, active bool) models.User {
	t.Helper()
	u := models.User{
		Username:     username,
		PasswordHash: HashFor(password),
		Role:         role,
		BuildingID:   buildingID,
		IsActive:     active,
	}
	require.NoError(t, a.DB.Create(&u).Error, "创建用户 %s 失败", username)
	if !active {
		// GORM 的 default:true 会把 false 当零值补成 true，必须显式更新
		require.NoError(t, a.DB.Model(&models.User{}).Where("id = ?", u.ID).Update("is_active", false).Error)
		u.IsActive = false
	}
	return u
}

func (a *App) mustBuilding(t *testing.T, root models.User, name, pkg string, deposit float64, district, street, village, no, expiredAt string) models.Building {
	t.Helper()
	b := models.Building{
		Name: name, Package: pkg, Deposit: deposit, ContractDate: a.today(),
		ExpiredAt: expiredAt, District: district, Street: street, Village: village,
		BuildingNo: no, Description: "自动化测试楼栋", Status: "active", CreatedBy: root.ID,
	}
	require.NoError(t, a.DB.Create(&b).Error, "创建楼栋 %s 失败", name)
	require.NoError(t, a.DB.Create(&models.BuildingLandlord{
		BuildingID: b.ID, Name: name + "房东", Phone: "13800000001",
	}).Error)
	return b
}

func (a *App) today() string { return utils.Now().Format("2006-01-02") }

func (a *App) mustRoom(t *testing.T, b models.Building, number, floor, layout string, rent, mgmt float64) models.Room {
	t.Helper()
	r := models.Room{
		BuildingID: b.ID, RoomNumber: number, Floor: floor, Layout: layout,
		Status: "vacant", RentPrice: ptr(rent), DepositMonths: ptr(uint(1)),
		ManagementFee: ptr(mgmt), ElectricityUnitPrice: ptr(1.5), WaterUnitPrice: ptr(4.5),
		Description: "自动化测试房间",
	}
	require.NoError(t, a.DB.Create(&r).Error, "创建房间 %s 失败", number)
	return r
}

func (a *App) mustContract(t *testing.T, b models.Building, r models.Room, tenantName, phone string, rent, mgmt, deposit float64, start, end, status string, earnest float64) models.RentalContract {
	t.Helper()
	tenant := models.Tenant{Name: tenantName, Phone: phone}
	require.NoError(t, a.DB.Create(&tenant).Error)
	ct := models.RentalContract{
		RoomID: r.ID, BuildingID: b.ID, TenantID: tenant.ID,
		RentPrice: rent, ManagementFee: mgmt, Deposit: deposit, EarnestMoney: earnest,
		StartDate: start, EndDate: end, Status: status,
	}
	require.NoError(t, a.DB.Create(&ct).Error)
	return ct
}

func (a *App) mustBill(t *testing.T, buildingID uint, roomID *uint, typ, subtype string, amount float64, date, desc, billNo string) models.Bill {
	t.Helper()
	b := models.Bill{
		BillNo: billNo, Type: typ, Subtype: subtype, Amount: amount,
		BuildingID: buildingID, RoomID: roomID, BillDate: date, Description: desc,
		PaidStatus: "paid", CreatedBy: 1,
	}
	require.NoError(t, a.DB.Create(&b).Error, "创建账单 %s 失败", billNo)
	return b
}

// MonthDay 返回某月第 d 天；d 超过当月天数时截断。
func MonthDay(month string, d int) string {
	t, err := time.Parse("2006-01", month)
	if err != nil {
		return fmt.Sprintf("%s-%02d", month, d)
	}
	last := t.AddDate(0, 1, -1).Day()
	if d > last {
		d = last
	}
	return fmt.Sprintf("%s-%02d", month, d)
}
func (f *Fixture) setStatus(t *testing.T, key, status string) {
	t.Helper()
	r := f.Rooms[key]
	require.NoError(t, f.App.DB.Model(&models.Room{}).Where("id = ?", r.ID).Update("status", status).Error)
	r.Status = status
	f.Rooms[key] = r
}

func roomID(f *Fixture, key string) *uint {
	id := f.Rooms[key].ID
	return &id
}

// NewAppWithFixture 一步创建 app 与基线 fixture。
func NewAppWithFixture(t *testing.T) (*App, *Fixture) {
	t.Helper()
	app := NewApp(t)
	return app, app.SeedBase(t)
}

// MonthLastDay 返回某月最后一天。
func MonthLastDay(month string) string {
	t, err := time.Parse("2006-01", month)
	if err != nil {
		return month + "-28"
	}
	return t.AddDate(0, 1, -1).Format("2006-01-02")
}
