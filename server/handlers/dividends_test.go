package handlers

import (
	"testing"

	"rental-server/models"
	"rental-server/services"
)

// setupDividendHandler 造分红处理器。
func setupDividendHandler(t *testing.T, e *testEnv) *DividendHandler {
	t.Helper()
	return &DividendHandler{DB: e.db, DividendService: services.NewDividendService(e.db)}
}

// 股东新增/修改/删除。
func TestShareholderCRUD(t *testing.T) {
	e := newTestEnv(t)
	h := setupDividendHandler(t, e)
	seedBuilding(t, e.db, 1)

	e.eng.POST("/shareholders", h.CreateShareholder)
	e.eng.PUT("/shareholders/:id", h.UpdateShareholder)
	e.eng.DELETE("/shareholders/:id", h.DeleteShareholder)

	// 新增
	w := e.do(t, "POST", "/shareholders", 1, map[string]any{"name": "王五", "share_ratio": 60})
	expectOK(t, w)
	var sh models.Shareholder
	if err := e.db.First(&sh).Error; err != nil {
		t.Fatalf("未创建股东: %v", err)
	}
	if sh.ShareRatio != 60 || sh.Name != "王五" {
		t.Fatalf("股东字段不符: %+v", sh)
	}

	// 修改持股比例
	w = e.do(t, "PUT", "/shareholders/"+itoa(sh.ID), 1, map[string]any{"name": "王五", "share_ratio": 70})
	expectOK(t, w)
	e.db.First(&sh, sh.ID)
	if sh.ShareRatio != 70 {
		t.Fatalf("持股比例应为 70，实际 %v", sh.ShareRatio)
	}

	// 删除
	w = e.do(t, "DELETE", "/shareholders/"+itoa(sh.ID), 1, nil)
	expectOK(t, w)
	var n int64
	e.db.Model(&models.Shareholder{}).Count(&n)
	if n != 0 {
		t.Fatalf("股东应被删除，实际剩余 %d 条", n)
	}
}

// 跨公寓不可修改/删除他人股东。
func TestShareholderCrossBuildingRejected(t *testing.T) {
	e := newTestEnv(t)
	h := setupDividendHandler(t, e)
	seedBuilding(t, e.db, 1)
	seedBuilding(t, e.db, 2)
	sh := models.Shareholder{BuildingID: 2, Name: "别人股东", ShareRatio: 100}
	if err := e.db.Create(&sh).Error; err != nil {
		t.Fatalf("造股东失败: %v", err)
	}

	e.eng.PUT("/shareholders/:id", h.UpdateShareholder)
	e.eng.DELETE("/shareholders/:id", h.DeleteShareholder)

	w := e.do(t, "PUT", "/shareholders/"+itoa(sh.ID), 1, map[string]any{"share_ratio": 10})
	expectFail(t, w)
	w = e.do(t, "DELETE", "/shareholders/"+itoa(sh.ID), 1, nil)
	expectFail(t, w)

	e.db.First(&sh, sh.ID)
	if sh.ShareRatio != 100 {
		t.Fatalf("越权操作不应修改股东，实际 %v", sh.ShareRatio)
	}
}

// 分红结算：按持股比例生成分红记录；重复结算覆盖旧记录（幂等）。
func TestDividendSettle(t *testing.T) {
	e := newTestEnv(t)
	h := setupDividendHandler(t, e)
	seedBuilding(t, e.db, 1)
	// 股东：王五 60%、赵六 40%
	sh1 := models.Shareholder{BuildingID: 1, Name: "王五", ShareRatio: 60}
	sh2 := models.Shareholder{BuildingID: 1, Name: "赵六", ShareRatio: 40}
	e.db.Create(&sh1)
	e.db.Create(&sh2)
	// 10 月收入 10000，支出 2000 → 净利润 8000
	seedBill(t, e.db, 1, 1, nil, "income", "租金", 10000, "2026-10-05")
	seedBill(t, e.db, 2, 1, nil, "expense", "维修", 2000, "2026-10-08")

	e.eng.POST("/dividends/settle", h.Settle)
	w := e.do(t, "POST", "/dividends/settle", 1, map[string]any{"month": "2026-10"})
	expectOK(t, w)

	var divs []models.Dividend
	e.db.Where("settle_month = ?", "2026-10").Find(&divs)
	if len(divs) != 2 {
		t.Fatalf("应生成 2 条分红记录，实际 %d 条", len(divs))
	}
	for _, d := range divs {
		if d.NetProfit != 8000 {
			t.Fatalf("净利润应为 8000，实际 %v", d.NetProfit)
		}
		want := 0.0
		if d.ShareholderID == sh1.ID {
			want = 4800
		} else {
			want = 3200
		}
		if d.DividendAmount != want {
			t.Fatalf("股东 %d 分红应为 %v，实际 %v", d.ShareholderID, want, d.DividendAmount)
		}
	}

	// 重复结算：旧记录被覆盖，不重复累加
	w = e.do(t, "POST", "/dividends/settle", 1, map[string]any{"month": "2026-10"})
	expectOK(t, w)
	var n int64
	e.db.Model(&models.Dividend{}).Where("settle_month = ?", "2026-10").Count(&n)
	if n != 2 {
		t.Fatalf("重复结算后仍应是 2 条记录，实际 %d 条", n)
	}
}

// 分红结算校验：月份格式错误、未配置股东。
func TestDividendSettleValidation(t *testing.T) {
	e := newTestEnv(t)
	h := setupDividendHandler(t, e)
	seedBuilding(t, e.db, 1)

	e.eng.POST("/dividends/settle", h.Settle)

	// 月份格式错误
	w := e.do(t, "POST", "/dividends/settle", 1, map[string]any{"month": "2026-13"})
	expectFail(t, w)

	// 未配置股东
	w = e.do(t, "POST", "/dividends/settle", 1, map[string]any{"month": "2026-10"})
	expectFail(t, w)

	var n int64
	e.db.Model(&models.Dividend{}).Count(&n)
	if n != 0 {
		t.Fatalf("校验失败不应生成分红记录，实际 %d 条", n)
	}
}

// 净利润为 0（无账单）时结算，分红金额为 0 但记录仍生成。
func TestDividendSettleZeroProfit(t *testing.T) {
	e := newTestEnv(t)
	h := setupDividendHandler(t, e)
	seedBuilding(t, e.db, 1)
	sh := models.Shareholder{BuildingID: 1, Name: "王五", ShareRatio: 100}
	e.db.Create(&sh)

	e.eng.POST("/dividends/settle", h.Settle)
	w := e.do(t, "POST", "/dividends/settle", 1, map[string]any{"month": "2026-10"})
	expectOK(t, w)

	var div models.Dividend
	if err := e.db.First(&div).Error; err != nil {
		t.Fatalf("未生成分红记录: %v", err)
	}
	if div.DividendAmount != 0 {
		t.Fatalf("无利润时分红应为 0，实际 %v", div.DividendAmount)
	}
}
