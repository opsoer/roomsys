package tests

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"rental-server/models"
	"rental-server/tests/testkit"
)

func TestDividend_Shareholders(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	list := app.Client.Do(http.MethodGet, "/api/building/dividends/shareholders", f.TokenA, nil)
	list.RequireOK(t)
	var data struct {
		Shareholders []models.Shareholder `json:"shareholders"`
	}
	list.Decode(t, &data)
	require.Len(t, data.Shareholders, 2)

	created := app.Client.Do(http.MethodPost, "/api/building/dividends/shareholders", f.TokenA, map[string]any{
		"name": "新股东", "share_ratio": 10,
	})
	created.RequireOK(t)
	var createdData struct {
		Shareholder models.Shareholder `json:"shareholder"`
	}
	created.Decode(t, &createdData)
	require.Equal(t, f.A.ID, createdData.Shareholder.BuildingID)

	missing := app.Client.Do(http.MethodPost, "/api/building/dividends/shareholders", f.TokenA, map[string]any{"share_ratio": 10})
	missing.RequireError(t, http.StatusBadRequest, 400)

	zero := app.Client.Do(http.MethodPost, "/api/building/dividends/shareholders", f.TokenA, map[string]any{"name": "零", "share_ratio": 0})
	zero.RequireError(t, http.StatusBadRequest, 400)

	// 当前实现允许负数/超过 100 的比例
	negative := app.Client.Do(http.MethodPost, "/api/building/dividends/shareholders", f.TokenA, map[string]any{"name": "负", "share_ratio": -10})
	negative.RequireOK(t)
	over := app.Client.Do(http.MethodPost, "/api/building/dividends/shareholders", f.TokenA, map[string]any{"name": "超", "share_ratio": 150})
	over.RequireOK(t)

	upd := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/dividends/shareholders/%d", createdData.Shareholder.ID), f.TokenA, map[string]any{
		"name": "改名股东", "share_ratio": 20,
	})
	upd.RequireOK(t)
	var got models.Shareholder
	require.NoError(t, app.DB.First(&got, createdData.Shareholder.ID).Error)
	require.Equal(t, "改名股东", got.Name)
	require.Equal(t, 20.0, got.ShareRatio)

	// 当前实现只更新 >0 的比例，0 被忽略
	zeroUpd := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/dividends/shareholders/%d", createdData.Shareholder.ID), f.TokenA, map[string]any{"share_ratio": 0})
	zeroUpd.RequireOK(t)
	require.NoError(t, app.DB.First(&got, createdData.Shareholder.ID).Error)
	require.Equal(t, 20.0, got.ShareRatio)

	// 跨栋
	bSh := models.Shareholder{BuildingID: f.B.ID, Name: "B股东", ShareRatio: 100}
	require.NoError(t, app.DB.Create(&bSh).Error)
	app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/dividends/shareholders/%d", bSh.ID), f.TokenA, map[string]any{"share_ratio": 1}).
		RequireError(t, http.StatusNotFound, 404)
	app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/building/dividends/shareholders/%d", bSh.ID), f.TokenA, nil).
		RequireError(t, http.StatusNotFound, 404)

	del := app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/building/dividends/shareholders/%d", createdData.Shareholder.ID), f.TokenA, nil)
	del.RequireOK(t)
	require.Zero(t, countWhere(t, app.DB, &models.Shareholder{}, "id = ?", createdData.Shareholder.ID))
}

func TestDividend_Calculate(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	type calcData struct {
		Month        string  `json:"month"`
		TotalIncome  float64 `json:"total_income"`
		TotalExpense float64 `json:"total_expense"`
		NetProfit    float64 `json:"net_profit"`
		Results      []struct {
			Name           string  `json:"name"`
			ShareRatio     float64 `json:"share_ratio"`
			DividendAmount float64 `json:"dividend_amount"`
		} `json:"results"`
	}

	r := app.Client.Do(http.MethodGet, "/api/building/dividends/calculate?month="+f.LastMonth, f.TokenA, nil)
	r.RequireOK(t)
	var data calcData
	r.Decode(t, &data)
	require.InDelta(t, 10000, data.TotalIncome, 0.01)
	require.InDelta(t, 2000, data.TotalExpense, 0.01)
	require.InDelta(t, 8000, data.NetProfit, 0.01)
	require.Len(t, data.Results, 2)
	require.InDelta(t, 4800, data.Results[0].DividendAmount, 0.01)
	require.InDelta(t, 3200, data.Results[1].DividendAmount, 0.01)

	// 默认月份 = 上月
	def := app.Client.Do(http.MethodGet, "/api/building/dividends/calculate", f.TokenA, nil)
	def.RequireOK(t)
	def.Decode(t, &data)
	require.Equal(t, f.LastMonth, data.Month)

	// 无净利润
	empty := app.Client.Do(http.MethodGet, "/api/building/dividends/calculate?month=2000-01", f.TokenA, nil)
	empty.RequireOK(t)
	empty.Decode(t, &data)
	require.Zero(t, data.NetProfit)
	require.Empty(t, data.Results)
	require.Contains(t, empty.Message, "无净利润")

	// 非法月份当前返回 200 + 0
	invalid := app.Client.Do(http.MethodGet, "/api/building/dividends/calculate?month=2026-13", f.TokenA, nil)
	invalid.RequireOK(t)
}

func TestDividend_CalculateNoShareholders(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)
	require.NoError(t, app.DB.Where("building_id = ?", f.A.ID).Delete(&models.Shareholder{}).Error)

	r := app.Client.Do(http.MethodGet, "/api/building/dividends/calculate?month="+f.LastMonth, f.TokenA, nil)
	r.RequireError(t, http.StatusBadRequest, 400).
		RequireMessageContains(t, "股东")
}

func TestDividend_Settle(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	r := app.Client.Do(http.MethodPost, "/api/building/dividends/settle", f.TokenA, map[string]any{"month": f.LastMonth})
	r.RequireOK(t)
	require.EqualValues(t, 2, countWhere(t, app.DB, &models.Dividend{}, "building_id = ? AND settle_month = ?", f.A.ID, f.LastMonth))

	var divs []models.Dividend
	require.NoError(t, app.DB.Where("building_id = ? AND settle_month = ?", f.A.ID, f.LastMonth).Find(&divs).Error)
	sum := 0.0
	for _, d := range divs {
		sum += d.DividendAmount
		require.NotZero(t, d.SettledBy)
	}
	require.InDelta(t, 8000, sum, 0.01)

	// 重复结算覆盖，不累加
	app.Client.Do(http.MethodPost, "/api/building/dividends/settle", f.TokenA, map[string]any{"month": f.LastMonth}).RequireOK(t)
	require.EqualValues(t, 2, countWhere(t, app.DB, &models.Dividend{}, "building_id = ? AND settle_month = ?", f.A.ID, f.LastMonth))

	// 非法/缺失月份
	app.Client.Do(http.MethodPost, "/api/building/dividends/settle", f.TokenA, map[string]any{"month": "2026-13"}).
		RequireError(t, http.StatusBadRequest, 400)
	app.Client.Do(http.MethodPost, "/api/building/dividends/settle", f.TokenA, map[string]any{}).
		RequireError(t, http.StatusBadRequest, 400)

	// 零利润月仍生成 0 分红快照
	app.Client.Do(http.MethodPost, "/api/building/dividends/settle", f.TokenA, map[string]any{"month": "2000-01"}).RequireOK(t)
	require.EqualValues(t, 2, countWhere(t, app.DB, &models.Dividend{}, "building_id = ? AND settle_month = ?", f.A.ID, "2000-01"))
}

func TestDividend_SettleNoShareholders(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)
	require.NoError(t, app.DB.Where("building_id = ?", f.A.ID).Delete(&models.Shareholder{}).Error)

	r := app.Client.Do(http.MethodPost, "/api/building/dividends/settle", f.TokenA, map[string]any{"month": "2000-01"})
	r.RequireError(t, http.StatusBadRequest, 400).
		RequireMessageContains(t, "股东")
	require.Zero(t, countWhere(t, app.DB, &models.Dividend{}, "building_id = ? AND settle_month = ?", f.A.ID, "2000-01"))
}

func TestDividend_List(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	type listResp struct {
		Dividends []models.Dividend `json:"dividends"`
		Total     int               `json:"total"`
		Page      int               `json:"page"`
		Size      int               `json:"size"`
	}
	r := app.Client.Do(http.MethodGet, "/api/building/dividends", f.TokenA, nil)
	r.RequireOK(t)
	var data listResp
	r.Decode(t, &data)
	require.Equal(t, 2, data.Total)

	page := app.Client.Do(http.MethodGet, "/api/building/dividends?page=1&page_size=1", f.TokenA, nil)
	page.RequireOK(t)
	page.Decode(t, &data)
	require.Equal(t, 2, data.Total)
	require.Len(t, data.Dividends, 1)

	last := data.Dividends[0]
	cursor := app.Client.Do(http.MethodGet, fmt.Sprintf("/api/building/dividends?last_id=%d&last_key=%s&page_size=1", last.ID, last.SettleMonth), f.TokenA, nil)
	cursor.RequireOK(t)
	cursor.Decode(t, &data)
	require.Equal(t, -1, data.Total)
}

func TestDividend_Predict(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	type predictResp struct {
		Predictions []struct {
			Month     string  `json:"month"`
			Rent      float64 `json:"rent"`
			Deposit   float64 `json:"deposit"`
			Available float64 `json:"available"`
		} `json:"predictions"`
	}
	get := func(q string) predictResp {
		r := app.Client.Do(http.MethodGet, "/api/building/dividends/predict"+q, f.TokenA, nil)
		r.RequireOK(t)
		var d predictResp
		r.Decode(t, &d)
		return d
	}

	d := get("")
	require.Len(t, d.Predictions, 3)
	// A102 2200 + A104 1980 + A105 2420 = 6600（reserved 不计）
	require.InDelta(t, 6600, d.Predictions[0].Rent, 0.01)
	require.Zero(t, d.Predictions[0].Deposit)
	require.Less(t, d.Predictions[0].Available, d.Predictions[0].Rent)

	require.Len(t, get("?months=6").Predictions, 6)
	require.Empty(t, get("?months=0").Predictions)
	require.Empty(t, get("?months=-1").Predictions)
	require.Len(t, get("?months=abc").Predictions, 3)

	// 只统计 active 合同：结束所有 active 后 rent=0
	require.NoError(t, app.DB.Model(&models.RentalContract{}).Where("building_id = ? AND status = ?", f.A.ID, "active").Update("status", "ended").Error)
	zero := get("")
	require.Zero(t, zero.Predictions[0].Rent)
	require.LessOrEqual(t, zero.Predictions[0].Available, 0.0)

	// B 栋只统计 B 合同
	b := app.Client.Do(http.MethodGet, "/api/building/dividends/predict", f.TokenB, nil)
	b.RequireOK(t)
	var bd predictResp
	b.Decode(t, &bd)
	require.InDelta(t, 3300, bd.Predictions[0].Rent, 0.01)
}

func TestDividend_SettledMonthLocksBills(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)
	monthDate := testkit.MonthDay(f.LastMonth, 15)

	app.Client.Do(http.MethodPost, "/api/building/bills", f.TokenA, map[string]any{
		"type": "income", "subtype": "租金", "amount": 100, "bill_date": monthDate,
	}).RequireError(t, http.StatusBadRequest, 400)

	app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/bills/%d", f.BillLastIncome.ID), f.TokenA, map[string]any{
		"amount": 1.0, "modify_reason": "x",
	}).RequireError(t, http.StatusBadRequest, 400)

	app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/building/bills/%d", f.BillLastIncome.ID), f.TokenA, nil).
		RequireError(t, http.StatusBadRequest, 400)
}
