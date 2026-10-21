package handlers

import (
	"testing"

	"rental-server/config"
	"rental-server/models"
	"rental-server/services"
	"rental-server/utils"
)

// 当前月的首末日，作为出租/退租测试的确定性账期。
func currentMonthRange() (string, string) {
	now := utils.Now()
	start := utils.FirstDayOfMonth(now).Format("2006-01-02")
	end := utils.LastDayOfMonth(now).Format("2006-01-02")
	return start, end
}

// setupRoomHandler 造房间处理器。
func setupRoomHandler(t *testing.T, e *testEnv) *RoomHandler {
	t.Helper()
	return &RoomHandler{DB: e.db, Cfg: &config.Config{}, RoomService: services.NewRoomService(e.db)}
}

// 出租（rented）：应创建租客、合同、租金账单与押金账单，并把房间置为 rented。
func TestUpdateStatusRentCreatesContractAndBills(t *testing.T) {
	e := newTestEnv(t)
	h := setupRoomHandler(t, e)
	seedBuilding(t, e.db, 1)
	seedRoom(t, e.db, 1, 1, "vacant")
	start, end := currentMonthRange()

	e.eng.POST("/rooms/:id/status", h.UpdateStatus)
	w := e.do(t, "POST", "/rooms/1/status", 1, map[string]any{
		"status": "rented", "tenant_name": "张三", "tenant_phone": "13900000001",
		"rent_price": 2000, "management_fee": 200, "deposit": 2000,
		"start_date": start, "end_date": end,
	})
	expectOK(t, w)

	// 房间状态
	if got := getRoom(t, e.db, 1).Status; got != "rented" {
		t.Fatalf("房间状态应为 rented，实际 %q", got)
	}
	// 租客
	var tenant models.Tenant
	if err := e.db.First(&tenant).Error; err != nil {
		t.Fatalf("未创建租客: %v", err)
	}
	if tenant.Name != "张三" {
		t.Fatalf("租客姓名不符: %q", tenant.Name)
	}
	// 合同
	var ct models.RentalContract
	if err := e.db.Where("room_id = ? AND status = ?", 1, "active").First(&ct).Error; err != nil {
		t.Fatalf("未创建生效合同: %v", err)
	}
	if ct.RentPrice != 2000 || ct.ManagementFee != 200 || ct.Deposit != 2000 {
		t.Fatalf("合同金额不符: %+v", ct)
	}
	if ct.TenantID != tenant.ID || ct.StartDate != start || ct.EndDate != end {
		t.Fatalf("合同字段不符: %+v", ct)
	}
	// 账单：整月租金+管理费，押金单独一笔
	if n := countBills(t, e.db, "room_id = ? AND subtype = ?", 1, "租金"); n != 1 {
		t.Fatalf("应生成 1 条租金账单，实际 %d 条", n)
	}
	var rentBill models.Bill
	e.db.Where("room_id = ? AND subtype = ?", 1, "租金").First(&rentBill)
	if rentBill.Amount != 2200 {
		t.Fatalf("整月租金+管理费应为 2200，实际 %v", rentBill.Amount)
	}
	if rentBill.Type != "income" {
		t.Fatalf("租金账单类型应为 income，实际 %q", rentBill.Type)
	}
	if n := countBills(t, e.db, "room_id = ? AND subtype = ?", 1, "押金"); n != 1 {
		t.Fatalf("应生成 1 条押金账单，实际 %d 条", n)
	}
}

// 出租参数校验：缺管理费、日期非法、起止颠倒都应被拒绝，且不产生任何数据。
func TestUpdateStatusRentValidation(t *testing.T) {
	e := newTestEnv(t)
	h := setupRoomHandler(t, e)
	seedBuilding(t, e.db, 1)
	seedRoom(t, e.db, 1, 1, "vacant")
	start, end := currentMonthRange()

	e.eng.POST("/rooms/:id/status", h.UpdateStatus)

	cases := []struct {
		name string
		body map[string]any
	}{
		{"缺管理费", map[string]any{
			"status": "rented", "tenant_name": "张三", "rent_price": 2000,
			"start_date": start, "end_date": end}},
		{"日期格式非法", map[string]any{
			"status": "rented", "tenant_name": "张三", "management_fee": 200,
			"start_date": "2026/10/01", "end_date": end}},
		{"起止颠倒", map[string]any{
			"status": "rented", "tenant_name": "张三", "management_fee": 200,
			"start_date": end, "end_date": start}},
		{"状态值非法", map[string]any{
			"status": "unknown", "tenant_name": "张三", "management_fee": 200,
			"start_date": start, "end_date": end}},
	}
	for _, tc := range cases {
		w := e.do(t, "POST", "/rooms/1/status", 1, tc.body)
		expectFail(t, w)
	}
	// 未产生任何合同/账单
	var ctCount, billCount int64
	e.db.Model(&models.RentalContract{}).Count(&ctCount)
	e.db.Model(&models.Bill{}).Count(&billCount)
	if ctCount != 0 || billCount != 0 {
		t.Fatalf("校验失败不应写入数据，合同 %d 条 账单 %d 条", ctCount, billCount)
	}
	if got := getRoom(t, e.db, 1).Status; got != "vacant" {
		t.Fatalf("房间状态不应变化，实际 %q", got)
	}
}

// 越权：其他公寓的房间不可操作。
func TestUpdateStatusRejectsOtherBuilding(t *testing.T) {
	e := newTestEnv(t)
	h := setupRoomHandler(t, e)
	seedBuilding(t, e.db, 1)
	seedBuilding(t, e.db, 2)
	seedRoom(t, e.db, 5, 2, "vacant")
	start, end := currentMonthRange()

	e.eng.POST("/rooms/:id/status", h.UpdateStatus)
	w := e.do(t, "POST", "/rooms/5/status", 1, map[string]any{
		"status": "rented", "tenant_name": "张三", "management_fee": 200,
		"rent_price": 2000, "start_date": start, "end_date": end,
	})
	expectFail(t, w)
	if got := getRoom(t, e.db, 5).Status; got != "vacant" {
		t.Fatalf("越权操作不应改变房间状态，实际 %q", got)
	}
}

// 退租（vacant）：结束合同、生成押金退还账单、房间置空、完成到期退租待办。
func TestUpdateStatusVacantRefundsDeposit(t *testing.T) {
	e := newTestEnv(t)
	h := setupRoomHandler(t, e)
	seedBuilding(t, e.db, 1)
	seedRoom(t, e.db, 1, 1, "rented")
	tenant := seedTenant(t, e.db, "张三")
	seedActiveContract(t, e.db, 1, 1, 1, tenant.ID, 2000)
	// 到期退租待办
	task := models.Task{BuildingID: ptrU(1), Title: "房间到期退租", Type: "expired_room", Status: "pending", RoomID: ptrU(1)}
	if err := e.db.Create(&task).Error; err != nil {
		t.Fatalf("造待办失败: %v", err)
	}

	e.eng.POST("/rooms/:id/status", h.UpdateStatus)
	w := e.do(t, "POST", "/rooms/1/status", 1, map[string]any{
		"status": "vacant", "refunded_deposit": 1800,
	})
	expectOK(t, w)

	// 合同结束
	var ct models.RentalContract
	e.db.First(&ct, 1)
	if ct.Status != "ended" {
		t.Fatalf("合同状态应为 ended，实际 %q", ct.Status)
	}
	// 押金退还账单
	if n := countBills(t, e.db, "room_id = ? AND subtype = ?", 1, "押金退还"); n != 1 {
		t.Fatalf("应生成 1 条押金退还账单，实际 %d 条", n)
	}
	var refund models.Bill
	e.db.Where("room_id = ? AND subtype = ?", 1, "押金退还").First(&refund)
	if refund.Amount != 1800 {
		t.Fatalf("退还金额应为 1800，实际 %v", refund.Amount)
	}
	if refund.Type != "expense" {
		t.Fatalf("退还账单类型应为 expense，实际 %q", refund.Type)
	}
	// 房间空置
	if got := getRoom(t, e.db, 1).Status; got != "vacant" {
		t.Fatalf("房间状态应为 vacant，实际 %q", got)
	}
	// 到期退租待办完成
	e.db.First(&task, task.ID)
	if task.Status != "completed" {
		t.Fatalf("到期退租待办应为 completed，实际 %q", task.Status)
	}
}

// 退租校验：没有生效合同、未填退还金额、负数金额都应被拒绝。
func TestUpdateStatusVacantValidation(t *testing.T) {
	e := newTestEnv(t)
	h := setupRoomHandler(t, e)
	seedBuilding(t, e.db, 1)
	seedRoom(t, e.db, 1, 1, "rented")
	tenant := seedTenant(t, e.db, "张三")
	seedActiveContract(t, e.db, 1, 1, 1, tenant.ID, 2000)

	e.eng.POST("/rooms/:id/status", h.UpdateStatus)

	// 未填退还金额
	w := e.do(t, "POST", "/rooms/1/status", 1, map[string]any{"status": "vacant"})
	expectFail(t, w)

	// 负数退还金额
	w = e.do(t, "POST", "/rooms/1/status", 1, map[string]any{"status": "vacant", "refunded_deposit": -1})
	expectFail(t, w)

	// 合同仍生效，房间未被清空
	var ct models.RentalContract
	e.db.First(&ct, 1)
	if ct.Status != "active" {
		t.Fatalf("失败的退租不应结束合同，实际 %q", ct.Status)
	}
	if got := getRoom(t, e.db, 1).Status; got != "rented" {
		t.Fatalf("失败的退租不应改变房间状态，实际 %q", got)
	}
	if n := countBills(t, e.db, "subtype = ?", "押金退还"); n != 0 {
		t.Fatalf("失败的退租不应生成账单，实际 %d 条", n)
	}
}

// 退租时无生效合同（对空置房误操作）应被拒绝，不产生押金退还账单。
func TestUpdateStatusVacantWithoutActiveContract(t *testing.T) {
	e := newTestEnv(t)
	h := setupRoomHandler(t, e)
	seedBuilding(t, e.db, 1)
	seedRoom(t, e.db, 1, 1, "vacant")

	e.eng.POST("/rooms/:id/status", h.UpdateStatus)
	w := e.do(t, "POST", "/rooms/1/status", 1, map[string]any{
		"status": "vacant", "refunded_deposit": 100,
	})
	expectFail(t, w)
	if n := countBills(t, e.db, "subtype = ?", "押金退还"); n != 0 {
		t.Fatalf("不应生成押金退还账单，实际 %d 条", n)
	}
}

// 预订（reserved）：房间置为 reserved，生成定金预订合同。
func TestUpdateStatusReserveCreatesContract(t *testing.T) {
	e := newTestEnv(t)
	h := setupRoomHandler(t, e)
	seedBuilding(t, e.db, 1)
	seedRoom(t, e.db, 1, 1, "vacant")
	start, end := currentMonthRange()

	e.eng.POST("/rooms/:id/status", h.UpdateStatus)
	w := e.do(t, "POST", "/rooms/1/status", 1, map[string]any{
		"status": "reserved", "tenant_name": "李四", "tenant_phone": "13900000002",
		"rent_price": 2000, "management_fee": 200, "deposit": 2000,
		"earnest_money": 500, "start_date": start, "end_date": end,
	})
	expectOK(t, w)

	if got := getRoom(t, e.db, 1).Status; got != "reserved" {
		t.Fatalf("房间状态应为 reserved，实际 %q", got)
	}
	var ct models.RentalContract
	if err := e.db.Where("room_id = ? AND status = ?", 1, "reserved").First(&ct).Error; err != nil {
		t.Fatalf("未创建预订合同: %v", err)
	}
	if ct.EarnestMoney != 500 {
		t.Fatalf("定金应为 500，实际 %v", ct.EarnestMoney)
	}
	// 预订不产生租金/押金账单（钱在签约时才记）
	if n := countBills(t, e.db, "room_id = ?", 1); n != 0 {
		t.Fatalf("预订不应生成账单，实际 %d 条", n)
	}
}

// 预订校验：定金为 0、缺租客姓名、缺管理费都应被拒绝。
func TestUpdateStatusReserveValidation(t *testing.T) {
	e := newTestEnv(t)
	h := setupRoomHandler(t, e)
	seedBuilding(t, e.db, 1)
	seedRoom(t, e.db, 1, 1, "vacant")
	start, end := currentMonthRange()

	e.eng.POST("/rooms/:id/status", h.UpdateStatus)
	cases := []struct {
		name string
		body map[string]any
	}{
		{"定金为0", map[string]any{
			"status": "reserved", "tenant_name": "李四", "management_fee": 200,
			"earnest_money": 0, "start_date": start, "end_date": end}},
		{"缺租客姓名", map[string]any{
			"status": "reserved", "management_fee": 200, "earnest_money": 500,
			"start_date": start, "end_date": end}},
		{"缺管理费", map[string]any{
			"status": "reserved", "tenant_name": "李四", "earnest_money": 500,
			"start_date": start, "end_date": end}},
	}
	for _, tc := range cases {
		w := e.do(t, "POST", "/rooms/1/status", 1, tc.body)
		expectFail(t, w)
	}
	var ctCount int64
	e.db.Model(&models.RentalContract{}).Count(&ctCount)
	if ctCount != 0 {
		t.Fatalf("预订校验失败不应创建合同，实际 %d 条", ctCount)
	}
	if got := getRoom(t, e.db, 1).Status; got != "vacant" {
		t.Fatalf("预订校验失败不应改变房间状态，实际 %q", got)
	}
}

// 取消预订：合同置 cancelled、房间回到 vacant；定金不退时生成「定金违约」收入账单。
func TestUpdateStatusCancelReservation(t *testing.T) {
	e := newTestEnv(t)
	h := setupRoomHandler(t, e)
	seedBuilding(t, e.db, 1)
	seedRoom(t, e.db, 1, 1, "reserved")
	tenant := seedTenant(t, e.db, "李四")
	ct := models.RentalContract{
		RoomID: 1, BuildingID: 1, TenantID: tenant.ID, RentPrice: 2000, ManagementFee: 200,
		Deposit: 2000, EarnestMoney: 500, StartDate: "2026-10-01", EndDate: "2026-10-31", Status: "reserved",
	}
	if err := e.db.Create(&ct).Error; err != nil {
		t.Fatalf("造预订合同失败: %v", err)
	}

	e.eng.POST("/rooms/:id/status", h.UpdateStatus)
	w := e.do(t, "POST", "/rooms/1/status", 1, map[string]any{
		"status": "vacant", "refunded_deposit": 0,
	})
	expectOK(t, w)

	var got models.RentalContract
	e.db.First(&got, ct.ID)
	if got.Status != "cancelled" {
		t.Fatalf("预订合同状态应为 cancelled，实际 %q", got.Status)
	}
	if room := getRoom(t, e.db, 1); room.Status != "vacant" {
		t.Fatalf("房间状态应为 vacant，实际 %q", room.Status)
	}
	// 定金 500 全额扣留 → 收入 500
	if n := countBills(t, e.db, "room_id = ? AND subtype = ?", 1, "定金违约"); n != 1 {
		t.Fatalf("应生成 1 条定金违约账单，实际 %d 条", n)
	}
	var bill models.Bill
	e.db.Where("room_id = ? AND subtype = ?", 1, "定金违约").First(&bill)
	if bill.Amount != 500 || bill.Type != "income" {
		t.Fatalf("定金违约账单应为收入 500，实际 type=%s amount=%v", bill.Type, bill.Amount)
	}
}

// 取消预订：未填退还金额应被拒绝；多退定金（房东违约）记支出。
func TestUpdateStatusCancelReservationRules(t *testing.T) {
	e := newTestEnv(t)
	h := setupRoomHandler(t, e)
	seedBuilding(t, e.db, 1)
	seedRoom(t, e.db, 1, 1, "reserved")
	tenant := seedTenant(t, e.db, "李四")
	ct := models.RentalContract{
		RoomID: 1, BuildingID: 1, TenantID: tenant.ID, RentPrice: 2000, ManagementFee: 200,
		Deposit: 2000, EarnestMoney: 500, StartDate: "2026-10-01", EndDate: "2026-10-31", Status: "reserved",
	}
	if err := e.db.Create(&ct).Error; err != nil {
		t.Fatalf("造预订合同失败: %v", err)
	}

	e.eng.POST("/rooms/:id/status", h.UpdateStatus)

	// 未填退还金额
	w := e.do(t, "POST", "/rooms/1/status", 1, map[string]any{"status": "vacant"})
	expectFail(t, w)

	// 多退 200 元 → 房东违约，记支出 200
	w = e.do(t, "POST", "/rooms/1/status", 1, map[string]any{
		"status": "vacant", "refunded_deposit": 700,
	})
	expectOK(t, w)
	var bill models.Bill
	e.db.Where("room_id = ? AND subtype = ?", 1, "定金违约").First(&bill)
	if bill.Amount != 200 || bill.Type != "expense" {
		t.Fatalf("房东违约应为支出 200，实际 type=%s amount=%v", bill.Type, bill.Amount)
	}
}

// 未来预订：对已出租房间预订，房间保持 rented，只新增 reserved 合同。
func TestUpdateStatusFutureReservationKeepsRoomRented(t *testing.T) {
	e := newTestEnv(t)
	h := setupRoomHandler(t, e)
	seedBuilding(t, e.db, 1)
	seedRoom(t, e.db, 1, 1, "rented")
	tenant := seedTenant(t, e.db, "张三")
	seedActiveContract(t, e.db, 1, 1, 1, tenant.ID, 2000)
	// 当前合同到本月末结束，未来预定的起租日必须在其之后
	now := utils.Now()
	ctEnd := utils.LastDayOfMonth(now).Format("2006-01-02")
	next := now.AddDate(0, 1, 0)
	start := utils.FirstDayOfMonth(next).Format("2006-01-02")
	end := utils.LastDayOfMonth(next).Format("2006-01-02")
	if err := e.db.Model(&models.RentalContract{}).Where("id = ?", 1).Update("end_date", ctEnd).Error; err != nil {
		t.Fatalf("调整合同结束日失败: %v", err)
	}

	e.eng.POST("/rooms/:id/status", h.UpdateStatus)
	w := e.do(t, "POST", "/rooms/1/status", 1, map[string]any{
		"status": "reserved", "tenant_name": "李四", "management_fee": 200,
		"rent_price": 2100, "earnest_money": 500, "start_date": start, "end_date": end,
	})
	expectOK(t, w)

	if got := getRoom(t, e.db, 1).Status; got != "rented" {
		t.Fatalf("未来预订不应改变房间状态，实际 %q", got)
	}
	var n int64
	e.db.Model(&models.RentalContract{}).Where("room_id = ? AND status = ?", 1, "reserved").Count(&n)
	if n != 1 {
		t.Fatalf("应新增 1 条预订合同，实际 %d 条", n)
	}
	// 原合同仍生效
	var ct models.RentalContract
	e.db.First(&ct, 1)
	if ct.Status != "active" {
		t.Fatalf("原合同应保持 active，实际 %q", ct.Status)
	}
}

// 签约：已有预订合同时确认签约，预订合同转 active，且不再重复生成账单。
func TestUpdateStatusConfirmReservedContract(t *testing.T) {
	e := newTestEnv(t)
	h := setupRoomHandler(t, e)
	seedBuilding(t, e.db, 1)
	seedRoom(t, e.db, 1, 1, "reserved")
	tenant := seedTenant(t, e.db, "李四")
	ct := models.RentalContract{
		RoomID: 1, BuildingID: 1, TenantID: tenant.ID, RentPrice: 2000, ManagementFee: 200,
		Deposit: 2000, EarnestMoney: 500, StartDate: "2026-10-01", EndDate: "2026-10-31", Status: "reserved",
	}
	if err := e.db.Create(&ct).Error; err != nil {
		t.Fatalf("造预订合同失败: %v", err)
	}
	start, end := currentMonthRange()

	e.eng.POST("/rooms/:id/status", h.UpdateStatus)
	w := e.do(t, "POST", "/rooms/1/status", 1, map[string]any{
		"status": "rented", "tenant_name": "李四", "management_fee": 200,
		"rent_price": 2000, "deposit": 2000, "start_date": start, "end_date": end,
	})
	expectOK(t, w)

	var got models.RentalContract
	e.db.First(&got, ct.ID)
	if got.Status != "active" {
		t.Fatalf("预订合同应转 active，实际 %q", got.Status)
	}
	if got.RentPrice != 2000 {
		t.Fatalf("合同租金不符: %v", got.RentPrice)
	}
	if room := getRoom(t, e.db, 1); room.Status != "rented" {
		t.Fatalf("房间状态应为 rented，实际 %q", room.Status)
	}
	// 确认签约会生成账单（未预收）
	if n := countBills(t, e.db, "room_id = ? AND subtype = ?", 1, "租金"); n != 1 {
		t.Fatalf("确认签约应生成 1 条租金账单，实际 %d 条", n)
	}
}

// ptrU 返回 uint 指针（models 中 BuildingID/RoomID 为指针字段）。
func ptrU(v uint) *uint { return &v }

// 再次出租：旧合同应被置为 ended，且只保留一条生效合同。
func TestUpdateStatusRentEndsPreviousContract(t *testing.T) {
	e := newTestEnv(t)
	h := setupRoomHandler(t, e)
	seedBuilding(t, e.db, 1)
	seedRoom(t, e.db, 1, 1, "rented")
	tenant := seedTenant(t, e.db, "张三")
	seedActiveContract(t, e.db, 1, 1, 1, tenant.ID, 2000)
	start, end := currentMonthRange()

	e.eng.POST("/rooms/:id/status", h.UpdateStatus)
	w := e.do(t, "POST", "/rooms/1/status", 1, map[string]any{
		"status": "rented", "tenant_name": "李四", "management_fee": 200,
		"rent_price": 2200, "deposit": 2200, "start_date": start, "end_date": end,
	})
	expectOK(t, w)

	var old models.RentalContract
	e.db.First(&old, 1)
	if old.Status != "ended" {
		t.Fatalf("旧合同应被结束，实际 %q", old.Status)
	}
	var activeCount int64
	e.db.Model(&models.RentalContract{}).Where("room_id = ? AND status = ?", 1, "active").Count(&activeCount)
	if activeCount != 1 {
		t.Fatalf("应只有 1 条生效合同，实际 %d 条", activeCount)
	}
}

// 起租日在上月：不重复记录押金账单（押金已在之前月份记过）。
func TestUpdateStatusRentSkipsDuplicateDepositBill(t *testing.T) {
	e := newTestEnv(t)
	h := setupRoomHandler(t, e)
	seedBuilding(t, e.db, 1)
	seedRoom(t, e.db, 1, 1, "vacant")
	now := utils.Now()
	prev := now.AddDate(0, -1, 0)
	start := utils.FirstDayOfMonth(prev).Format("2006-01-02")
	end := utils.LastDayOfMonth(now).Format("2006-01-02")

	e.eng.POST("/rooms/:id/status", h.UpdateStatus)
	w := e.do(t, "POST", "/rooms/1/status", 1, map[string]any{
		"status": "rented", "tenant_name": "张三", "management_fee": 200,
		"rent_price": 2000, "deposit": 2000, "start_date": start, "end_date": end,
	})
	expectOK(t, w)

	if n := countBills(t, e.db, "room_id = ? AND subtype = ?", 1, "押金"); n != 0 {
		t.Fatalf("跨月起租不应重复记录押金账单，实际 %d 条", n)
	}
	if n := countBills(t, e.db, "room_id = ? AND subtype = ?", 1, "租金"); n != 1 {
		t.Fatalf("应生成 1 条租金账单，实际 %d 条", n)
	}
}

// 退租时存在未来预订：房间应回到 reserved（等待下一租客签约），而不是 vacant。
func TestUpdateStatusVacantWithFutureReservation(t *testing.T) {
	e := newTestEnv(t)
	h := setupRoomHandler(t, e)
	seedBuilding(t, e.db, 1)
	seedRoom(t, e.db, 1, 1, "rented")
	tenant := seedTenant(t, e.db, "张三")
	seedActiveContract(t, e.db, 1, 1, 1, tenant.ID, 2000)
	now := utils.Now()
	next := now.AddDate(0, 1, 0)
	future := models.RentalContract{
		RoomID: 1, BuildingID: 1, TenantID: tenant.ID, RentPrice: 2100, ManagementFee: 200,
		Deposit: 2100, EarnestMoney: 500,
		StartDate: utils.FirstDayOfMonth(next).Format("2006-01-02"),
		EndDate:   utils.LastDayOfMonth(next).Format("2006-01-02"),
		Status:    "reserved",
	}
	if err := e.db.Create(&future).Error; err != nil {
		t.Fatalf("造未来预订失败: %v", err)
	}

	e.eng.POST("/rooms/:id/status", h.UpdateStatus)
	w := e.do(t, "POST", "/rooms/1/status", 1, map[string]any{
		"status": "vacant", "refunded_deposit": 2000,
	})
	expectOK(t, w)

	if got := getRoom(t, e.db, 1).Status; got != "reserved" {
		t.Fatalf("存在未来预订时房间应为 reserved，实际 %q", got)
	}
}
