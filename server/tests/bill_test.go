package tests

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"rental-server/models"
	"rental-server/tests/testkit"
	"rental-server/utils"
)

func createBill(t *testing.T, app *testkit.App, token, typ, subtype string, amount float64, date, desc string, roomID *uint) models.Bill {
	t.Helper()
	body := map[string]any{"type": typ, "subtype": subtype, "amount": amount, "bill_date": date, "description": desc}
	if roomID != nil {
		body["room_id"] = *roomID
	}
	r := app.Client.Do(http.MethodPost, "/api/building/bills", token, body)
	r.RequireOK(t)
	var data struct {
		Bill models.Bill `json:"bill"`
	}
	r.Decode(t, &data)
	return data.Bill
}

func TestBill_CreateValidation(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)
	roomID := f.Rooms["A101"].ID

	created := createBill(t, app, f.TokenA, "income", "租金", 2000, f.Today, "测试收入", &roomID)
	require.NotZero(t, created.ID)
	require.NotEmpty(t, created.BillNo)
	require.Equal(t, f.A.ID, created.BuildingID)
	require.Equal(t, "income", getBill(t, app, created.ID).Type)

	createBill(t, app, f.TokenA, "expense", "维修", 300, f.Today, "测试支出", nil)

	failures := []struct {
		name string
		body map[string]any
	}{
		{"invalid type", map[string]any{"type": "unknown", "subtype": "租金", "amount": 100, "bill_date": f.Today}},
		{"invalid date", map[string]any{"type": "income", "subtype": "租金", "amount": 100, "bill_date": "2026/10/05"}},
		{"negative amount", map[string]any{"type": "income", "subtype": "租金", "amount": -1, "bill_date": f.Today}},
		{"zero amount", map[string]any{"type": "income", "subtype": "租金", "amount": 0, "bill_date": f.Today}},
		{"other without desc", map[string]any{"type": "income", "subtype": "其他", "amount": 100, "bill_date": f.Today}},
		{"cross building room", map[string]any{"type": "income", "subtype": "租金", "amount": 100, "bill_date": f.Today, "room_id": f.Rooms["B201"].ID}},
		{"nonexistent room", map[string]any{"type": "income", "subtype": "租金", "amount": 100, "bill_date": f.Today, "room_id": 999999}},
		{"settled month", map[string]any{"type": "income", "subtype": "租金", "amount": 100, "bill_date": testkit.MonthDay(f.LastMonth, 15)}},
	}
	before := countWhere(t, app.DB, &models.Bill{}, "1 = 1")
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			r := app.Client.Do(http.MethodPost, "/api/building/bills", f.TokenA, tc.body)
			r.RequireError(t, http.StatusBadRequest, 400)
		})
	}
	require.Equal(t, before, countWhere(t, app.DB, &models.Bill{}, "1 = 1"))

	other := createBill(t, app, f.TokenA, "income", "其他", 100, f.Today, "其他收入", nil)
	require.Equal(t, "其他", other.Subtype)

	noRoom := createBill(t, app, f.TokenA, "expense", "维修", 50, f.Today, "", nil)
	require.Nil(t, getBill(t, app, noRoom.ID).RoomID)
}

func TestBill_ListFilters(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	type billListResp struct {
		Bills []models.Bill `json:"bills"`
		Total int           `json:"total"`
		Page  int           `json:"page"`
		Size  int           `json:"size"`
	}
	get := func(q string) billListResp {
		r := app.Client.Do(http.MethodGet, "/api/building/bills"+q, f.TokenA, nil)
		r.RequireOK(t)
		var d billListResp
		r.Decode(t, &d)
		return d
	}

	income := get("?type=income")
	require.NotEmpty(t, income.Bills)
	for _, b := range income.Bills {
		require.Equal(t, "income", b.Type)
	}

	repair := get("?subtype=维修")
	require.NotEmpty(t, repair.Bills)
	for _, b := range repair.Bills {
		require.Equal(t, "维修", b.Subtype)
	}

	// 当前列表实现下 room_id 参数不参与过滤，返回全量账单
	allBills := get("")
	byRoom := get(fmt.Sprintf("?room_id=%d", f.Rooms["A102"].ID))
	require.Equal(t, allBills.Total, byRoom.Total)

	byNumber := get("?room_number=A102")
	require.NotEmpty(t, byNumber.Bills)
	for _, b := range byNumber.Bills {
		require.NotNil(t, b.RoomID)
		require.Equal(t, f.Rooms["A102"].ID, *b.RoomID)
	}

	rangeList := get("?start_date=" + testkit.MonthDay(f.LastMonth, 1) + "&end_date=" + testkit.MonthLastDay(f.LastMonth))
	require.Equal(t, 2, rangeList.Total)

	page := get("?page=1&page_size=2")
	require.Equal(t, 1, page.Page)
	require.Equal(t, 2, page.Size)
	require.Len(t, page.Bills, 2)

	first := page.Bills
	last := first[len(first)-1]
	cursor := get(fmt.Sprintf("?last_id=%d&last_key=%s&page_size=2", last.ID, last.BillDate))
	require.Equal(t, -1, cursor.Total)
	for _, b := range cursor.Bills {
		require.True(t, b.BillDate < last.BillDate || (b.BillDate == last.BillDate && b.ID < last.ID))
	}

	// 软删除账单不出现在列表
	for _, b := range get("").Bills {
		require.NotEqual(t, "FIX-SOFT", b.BillNo)
	}
}

func TestBill_Update(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	r := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/bills/%d", f.BillThisIncome.ID), f.TokenA, map[string]any{
		"amount": 1800.0, "modify_reason": "金额录错",
	})
	r.RequireOK(t)
	updated := getBill(t, app, f.BillThisIncome.ID)
	require.Equal(t, 1800.0, updated.Amount)
	require.Contains(t, updated.Description, "金额录错")
	require.Contains(t, updated.Description, "2000.00 -> 1800.00")

	missingReason := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/bills/%d", f.BillThisIncome.ID), f.TokenA, map[string]any{"amount": 1900.0})
	missingReason.RequireError(t, http.StatusBadRequest, 400)

	negative := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/bills/%d", f.BillThisIncome.ID), f.TokenA, map[string]any{"amount": -1.0, "modify_reason": "x"})
	negative.RequireError(t, http.StatusBadRequest, 400)

	var bBill models.Bill
	require.NoError(t, app.DB.Where("building_id = ? AND subtype = ?", f.B.ID, "租金").First(&bBill).Error)
	cross := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/bills/%d", bBill.ID), f.TokenA, map[string]any{"amount": 1.0, "modify_reason": "x"})
	cross.RequireError(t, http.StatusNotFound, 404)

	settled := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/bills/%d", f.BillLastIncome.ID), f.TokenA, map[string]any{"amount": 1.0, "modify_reason": "x"})
	settled.RequireError(t, http.StatusBadRequest, 400).
		RequireMessageContains(t, "已结算")

	// 当前实现只更新 amount/description，忽略 type/subtype/date/room
	ignored := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/bills/%d", f.BillThisIncome.ID), f.TokenA, map[string]any{
		"type": "expense", "subtype": "维修", "bill_date": f.Today, "room_id": f.Rooms["B201"].ID, "modify_reason": "尝试改类型",
	})
	ignored.RequireOK(t)
	after := getBill(t, app, f.BillThisIncome.ID)
	require.Equal(t, "income", after.Type)
	require.Equal(t, "租金", after.Subtype)
	require.Nil(t, after.RoomID)
}

func TestBill_Delete(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	r := app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/building/bills/%d", f.BillThisExpense.ID), f.TokenA, nil)
	r.RequireOK(t)
	require.Zero(t, countWhere(t, app.DB, &models.Bill{}, "id = ?", f.BillThisExpense.ID))

	again := app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/building/bills/%d", f.BillThisExpense.ID), f.TokenA, nil)
	again.RequireError(t, http.StatusNotFound, 404)

	var bBill models.Bill
	require.NoError(t, app.DB.Where("building_id = ?", f.B.ID).First(&bBill).Error)
	cross := app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/building/bills/%d", bBill.ID), f.TokenA, nil)
	cross.RequireError(t, http.StatusNotFound, 404)

	settled := app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/building/bills/%d", f.BillLastIncome.ID), f.TokenA, nil)
	settled.RequireError(t, http.StatusBadRequest, 400).
		RequireMessageContains(t, "已结算")
}

func TestBill_NumberRegression(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)
	roomID := f.Rooms["A101"].ID

	first := createBill(t, app, f.TokenA, "income", "租金", 100, f.Today, "编号测试1", &roomID)
	require.Regexp(t, `^B\d{8}\d{5}$`, first.BillNo)

	del := app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/building/bills/%d", first.ID), f.TokenA, nil)
	del.RequireOK(t)

	second := createBill(t, app, f.TokenA, "income", "租金", 100, f.Today, "编号测试2", &roomID)
	require.NotEqual(t, first.BillNo, second.BillNo)

	third := createBill(t, app, f.TokenA, "income", "租金", 100, f.Today, "编号测试3", &roomID)
	require.Greater(t, third.BillNo, second.BillNo)

	seen := map[string]bool{}
	for i := 0; i < 5; i++ {
		b := createBill(t, app, f.TokenA, "income", "租金", 1, f.Today, fmt.Sprintf("编号并发%d", i), &roomID)
		require.False(t, seen[b.BillNo], "duplicate bill_no %s", b.BillNo)
		seen[b.BillNo] = true
	}
}

func TestBill_Stats(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	r := app.Client.Do(http.MethodGet, "/api/building/bills/stats?month="+f.LastMonth, f.TokenA, nil)
	r.RequireOK(t)
	var data struct {
		TotalIncome   float64          `json:"total_income"`
		TotalExpense  float64          `json:"total_expense"`
		NetProfit     float64          `json:"net_profit"`
		BillCount     int64            `json:"bill_count"`
		IncomeDetail  []map[string]any `json:"income_detail"`
		ExpenseDetail []map[string]any `json:"expense_detail"`
	}
	r.Decode(t, &data)
	require.InDelta(t, 10000, data.TotalIncome, 0.01)
	require.InDelta(t, 2000, data.TotalExpense, 0.01)
	require.InDelta(t, 8000, data.NetProfit, 0.01)
	require.EqualValues(t, 2, data.BillCount)
	require.NotEmpty(t, data.IncomeDetail)
	require.NotEmpty(t, data.ExpenseDetail)

	year := utils.Now().Format("2006")
	yr := app.Client.Do(http.MethodGet, "/api/building/bills/stats?year="+year, f.TokenA, nil)
	yr.RequireOK(t)
	var yd struct {
		TotalIncome  float64 `json:"total_income"`
		TotalExpense float64 `json:"total_expense"`
		NetProfit    float64 `json:"net_profit"`
	}
	yr.Decode(t, &yd)
	require.InDelta(t, yd.TotalIncome-yd.TotalExpense, yd.NetProfit, 0.01)

	empty := app.Client.Do(http.MethodGet, "/api/building/bills/stats?month=2000-01", f.TokenA, nil)
	empty.RequireOK(t)
	var ed struct {
		TotalIncome  float64 `json:"total_income"`
		TotalExpense float64 `json:"total_expense"`
		NetProfit    float64 `json:"net_profit"`
		BillCount    int64   `json:"bill_count"`
	}
	empty.Decode(t, &ed)
	require.Zero(t, ed.TotalIncome)
	require.Zero(t, ed.TotalExpense)
	require.Zero(t, ed.NetProfit)
	require.Zero(t, ed.BillCount)

	// 当前实现：非法月份被 service 解析失败，handler 返回 500
	invalid := app.Client.Do(http.MethodGet, "/api/building/bills/stats?month=2026-13", f.TokenA, nil)
	invalid.RequireStatus(t, http.StatusInternalServerError, 500)
}

func TestBill_Trend(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	type trendResp struct {
		Months []struct {
			Month   string  `json:"month"`
			Income  float64 `json:"income"`
			Expense float64 `json:"expense"`
			Profit  float64 `json:"profit"`
		} `json:"months"`
		Growth []map[string]any `json:"growth"`
	}
	get := func(q string) trendResp {
		r := app.Client.Do(http.MethodGet, "/api/building/bills/trend"+q, f.TokenA, nil)
		r.RequireOK(t)
		var d trendResp
		r.Decode(t, &d)
		return d
	}

	d12 := get("?years=12")
	require.Len(t, d12.Months, 12)
	require.Len(t, d12.Growth, 12)
	found := false
	for _, m := range d12.Months {
		if m.Month == f.LastMonth {
			found = true
			require.InDelta(t, 10000, m.Income, 0.01)
			require.InDelta(t, 2000, m.Expense, 0.01)
			require.InDelta(t, 8000, m.Profit, 0.01)
		}
	}
	require.True(t, found)

	require.Len(t, get("?years=3").Months, 3)
	require.Len(t, get("?years=0").Months, 12)
	require.Len(t, get("?years=abc").Months, 12)

	// 无账单月份补零
	emptyMonth := utils.Now().AddDate(0, -2, 0).Format("2006-01")
	for _, m := range d12.Months {
		if m.Month == emptyMonth {
			require.Zero(t, m.Income)
			require.Zero(t, m.Expense)
			require.Zero(t, m.Profit)
		}
	}
}

func TestBill_ExportCSV(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	r := app.Client.Do(http.MethodGet, "/api/building/bills/export", f.TokenA, nil)
	r.RequireOK(t)
	require.Contains(t, r.Header.Get("Content-Type"), "text/csv")
	require.Contains(t, r.Header.Get("Content-Disposition"), "bills.csv")
	require.GreaterOrEqual(t, len(r.Raw), 3)
	require.Equal(t, []byte{0xEF, 0xBB, 0xBF}, r.Raw[:3])

	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(r.Raw), "\ufeff")))
	rows, err := reader.ReadAll()
	require.NoError(t, err)
	require.Equal(t, []string{"账单编号", "日期", "类型", "子类型", "金额", "关联房间", "备注"}, rows[0])
	require.Greater(t, len(rows), 1)

	// 筛选上月：2 条数据 + 表头
	filtered := app.Client.Do(http.MethodGet, "/api/building/bills/export?start_date="+testkit.MonthDay(f.LastMonth, 1)+"&end_date="+testkit.MonthLastDay(f.LastMonth), f.TokenA, nil)
	filtered.RequireOK(t)
	fr, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(filtered.Raw), "\ufeff"))).ReadAll()
	require.NoError(t, err)
	require.Len(t, fr, 3)

	// CSV 注入防护
	require.Contains(t, string(r.Raw), "'=2+3")

	// 空月份只有表头
	empty := app.Client.Do(http.MethodGet, "/api/building/bills/export?start_date=2000-01-01&end_date=2000-01-31", f.TokenA, nil)
	empty.RequireOK(t)
	er, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(empty.Raw), "\ufeff"))).ReadAll()
	require.NoError(t, err)
	require.Len(t, er, 1)

	// 软删除不导出
	require.NotContains(t, string(r.Raw), "FIX-SOFT")
}

func TestBill_Audit(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)
	roomID := f.Rooms["A101"].ID

	b := createBill(t, app, f.TokenA, "income", "租金", 100, f.Today, "审计测试", &roomID)
	app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/bills/%d", b.ID), f.TokenA, map[string]any{
		"amount": 120.0, "modify_reason": "审计修改",
	}).RequireOK(t)
	app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/building/bills/%d", b.ID), f.TokenA, nil).RequireOK(t)

	require.GreaterOrEqual(t, countWhere(t, app.DB, &models.AuditLog{}, "resource = ? AND action = ?", "bill", "create"), int64(1))
	require.GreaterOrEqual(t, countWhere(t, app.DB, &models.AuditLog{}, "resource = ? AND action = ?", "bill", "update"), int64(1))
	require.GreaterOrEqual(t, countWhere(t, app.DB, &models.AuditLog{}, "resource = ? AND action = ?", "bill", "delete"), int64(1))
}
