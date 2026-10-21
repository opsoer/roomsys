package handlers

import (
	"testing"

	"rental-server/models"
	"rental-server/services"
)

// setupBillHandler 造账单处理器。
func setupBillHandler(t *testing.T, e *testEnv) *BillHandler {
	t.Helper()
	return &BillHandler{
		DB:          e.db,
		BillService: services.NewBillService(e.db),
		RoomService: services.NewRoomService(e.db),
	}
}

// 账单创建：正常创建，并生成不重复的单号。
func TestBillCreate(t *testing.T) {
	e := newTestEnv(t)
	h := setupBillHandler(t, e)
	seedBuilding(t, e.db, 1)
	seedRoom(t, e.db, 1, 1, "vacant")

	e.eng.POST("/bills", h.Create)
	w := e.do(t, "POST", "/bills", 1, map[string]any{
		"type": "income", "subtype": "租金", "amount": 2000,
		"room_id": 1, "bill_date": "2026-10-05", "description": "10月租金",
	})
	expectOK(t, w)

	var bills []models.Bill
	e.db.Find(&bills)
	if len(bills) != 1 {
		t.Fatalf("应创建 1 条账单，实际 %d 条", len(bills))
	}
	if bills[0].Amount != 2000 || bills[0].Type != "income" || bills[0].BuildingID != 1 {
		t.Fatalf("账单字段不符: %+v", bills[0])
	}
	if bills[0].BillNo == "" {
		t.Fatal("账单单号不应为空")
	}
}

// 账单创建校验：类型非法、日期非法、跨公寓房间、子类型「其他」缺备注。
func TestBillCreateValidation(t *testing.T) {
	e := newTestEnv(t)
	h := setupBillHandler(t, e)
	seedBuilding(t, e.db, 1)
	seedBuilding(t, e.db, 2)
	seedRoom(t, e.db, 1, 1, "vacant")
	seedRoom(t, e.db, 2, 2, "vacant")

	e.eng.POST("/bills", h.Create)
	cases := []struct {
		name string
		body map[string]any
	}{
		{"类型非法", map[string]any{"type": "unknown", "subtype": "租金", "amount": 100, "bill_date": "2026-10-05"}},
		{"日期非法", map[string]any{"type": "income", "subtype": "租金", "amount": 100, "bill_date": "2026/10/05"}},
		{"跨公寓房间", map[string]any{"type": "income", "subtype": "租金", "amount": 100, "room_id": 2, "bill_date": "2026-10-05"}},
		{"其他缺备注", map[string]any{"type": "income", "subtype": "其他", "amount": 100, "bill_date": "2026-10-05"}},
		{"金额为负", map[string]any{"type": "income", "subtype": "租金", "amount": -1, "bill_date": "2026-10-05"}},
	}
	for _, tc := range cases {
		w := e.do(t, "POST", "/bills", 1, tc.body)
		expectFail(t, w)
	}
	var n int64
	e.db.Model(&models.Bill{}).Count(&n)
	if n != 0 {
		t.Fatalf("校验失败不应创建账单，实际 %d 条", n)
	}
}

// 账单修改：改金额并写入修改原因到描述。
func TestBillUpdate(t *testing.T) {
	e := newTestEnv(t)
	h := setupBillHandler(t, e)
	seedBuilding(t, e.db, 1)
	seedBill(t, e.db, 1, 1, nil, "income", "租金", 2000, "2026-10-05")

	e.eng.PUT("/bills/:id", h.Update)
	w := e.do(t, "PUT", "/bills/1", 1, map[string]any{"amount": 1800, "modify_reason": "金额录错"})
	expectOK(t, w)

	var bill models.Bill
	e.db.First(&bill, 1)
	if bill.Amount != 1800 {
		t.Fatalf("账单金额应为 1800，实际 %v", bill.Amount)
	}
	if bill.Description == "" {
		t.Fatal("修改原因应写入账单描述")
	}
}

// 账单修改校验：缺少修改原因、负数金额、修改他人公寓账单、修改已结算月份的账单。
func TestBillUpdateValidation(t *testing.T) {
	e := newTestEnv(t)
	h := setupBillHandler(t, e)
	seedBuilding(t, e.db, 1)
	seedBuilding(t, e.db, 2)
	seedBill(t, e.db, 1, 1, nil, "income", "租金", 2000, "2026-10-05")
	seedBill(t, e.db, 2, 2, nil, "income", "租金", 100, "2026-10-05")

	e.eng.PUT("/bills/:id", h.Update)

	// 缺少修改原因
	w := e.do(t, "PUT", "/bills/1", 1, map[string]any{"amount": 1800})
	expectFail(t, w)

	// 负数金额
	w = e.do(t, "PUT", "/bills/1", 1, map[string]any{"amount": -1, "modify_reason": "x"})
	expectFail(t, w)

	// 跨公寓账单
	w = e.do(t, "PUT", "/bills/2", 1, map[string]any{"amount": 50, "modify_reason": "x"})
	expectFail(t, w)

	// 原账单未被改动
	var bill models.Bill
	e.db.First(&bill, 1)
	if bill.Amount != 2000 {
		t.Fatalf("失败的修改不应改变金额，实际 %v", bill.Amount)
	}
}

// 已结算月份的账单不可修改、不可删除、不可新建。
func TestBillLockedAfterDividendSettled(t *testing.T) {
	e := newTestEnv(t)
	h := setupBillHandler(t, e)
	seedBuilding(t, e.db, 1)
	seedBill(t, e.db, 1, 1, nil, "income", "租金", 2000, "2026-10-05")
	// 该月已结算
	div := models.Dividend{BuildingID: 1, SettleMonth: "2026-10", TotalIncome: 2000, NetProfit: 2000, DividendAmount: 2000}
	if err := e.db.Create(&div).Error; err != nil {
		t.Fatalf("造分红记录失败: %v", err)
	}

	e.eng.PUT("/bills/:id", h.Update)
	e.eng.DELETE("/bills/:id", h.Delete)
	e.eng.POST("/bills", h.Create)

	w := e.do(t, "PUT", "/bills/1", 1, map[string]any{"amount": 1800, "modify_reason": "x"})
	expectFail(t, w)

	w = e.do(t, "DELETE", "/bills/1", 1, nil)
	expectFail(t, w)

	w = e.do(t, "POST", "/bills", 1, map[string]any{
		"type": "income", "subtype": "租金", "amount": 100, "bill_date": "2026-10-20",
	})
	expectFail(t, w)

	var bill models.Bill
	e.db.First(&bill, 1)
	if bill.Amount != 2000 {
		t.Fatalf("已结算月份的账单不应被修改，实际 %v", bill.Amount)
	}
}

// 账单删除：正常删除（软删）与跨公寓删除拒绝。
func TestBillDelete(t *testing.T) {
	e := newTestEnv(t)
	h := setupBillHandler(t, e)
	seedBuilding(t, e.db, 1)
	seedBuilding(t, e.db, 2)
	seedBill(t, e.db, 1, 1, nil, "income", "租金", 2000, "2026-10-05")
	seedBill(t, e.db, 2, 2, nil, "income", "租金", 100, "2026-10-05")

	e.eng.DELETE("/bills/:id", h.Delete)

	// 跨公寓删除
	w := e.do(t, "DELETE", "/bills/2", 1, nil)
	expectFail(t, w)

	// 本公寓删除
	w = e.do(t, "DELETE", "/bills/1", 1, nil)
	expectOK(t, w)

	var n int64
	e.db.Model(&models.Bill{}).Where("id = ?", 1).Count(&n)
	if n != 0 {
		t.Fatal("账单应被删除")
	}
}
